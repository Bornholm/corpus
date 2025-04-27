package ftp

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type withConnFunc func(fn func(*ftp.ServerConn) error) error

type Fs struct {
	withConn withConnFunc
}

// Chmod implements afero.Fs.
func (fs *Fs) Chmod(name string, mode fs.FileMode) error {
	return nil
}

// Chown implements afero.Fs.
func (fs *Fs) Chown(name string, uid int, gid int) error {
	return nil
}

// Chtimes implements afero.Fs.
func (fs *Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		if err := conn.SetTime(name, mtime); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Create implements afero.Fs.
func (fs *Fs) Create(name string) (afero.File, error) {
	var file *File

	err := fs.withConn(func(conn *ftp.ServerConn) error {
		null := bytes.NewBuffer(nil)
		if err := conn.Stor(name, null); err != nil {
			return errors.WithStack(err)
		}

		file = &File{
			name:     name,
			withConn: fs.withConn,
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Mkdir implements afero.Fs.
func (fs *Fs) Mkdir(name string, perm fs.FileMode) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		parent := filepath.Dir(name)

		entries, err := conn.List(parent)
		if err != nil {
			return errors.WithStack(err)
		}

		for _, e := range entries {
			if e.Name != name {
				continue
			}

			if e.Type != ftp.EntryTypeFolder {
				return errors.Errorf("File '%s' already exists and is not a directory", name)
			}

			return nil
		}

		if err := conn.MakeDir(name); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// MkdirAll implements afero.Fs.
func (fs *Fs) MkdirAll(path string, perm fs.FileMode) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		dir, err := fs.Stat(path)
		if err == nil {
			if dir.IsDir() {
				return nil
			}
			return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
		}

		i := len(path)
		for i > 0 && os.IsPathSeparator(path[i-1]) { // Skip trailing path separator.
			i--
		}

		j := i
		for j > 0 && !os.IsPathSeparator(path[j-1]) { // Scan backward over element.
			j--
		}

		if j > 1 {
			err = fs.MkdirAll(path[:j-1], perm)
			if err != nil {
				return errors.WithStack(err)
			}
		}

		err = fs.Mkdir(path, perm)
		if err != nil {
			dir, err1 := fs.Stat(path)
			if err1 == nil && dir.IsDir() {
				return nil
			}
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Name implements afero.Fs.
func (fs *Fs) Name() string {
	return "ftpfs"
}

// Open implements afero.Fs.
func (fs *Fs) Open(name string) (afero.File, error) {
	var file *File

	err := fs.withConn(func(conn *ftp.ServerConn) error {
		_, err := getFileInfo(conn, name)
		if err != nil {
			return errors.WithStack(err)
		}

		file = &File{
			withConn: fs.withConn,
			name:     name,
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// OpenFile implements afero.Fs.
func (fs *Fs) OpenFile(name string, flag int, perm fs.FileMode) (afero.File, error) {
	file, err := fs.Open(name)
	if file != nil {
		return file, nil
	}

	if !errors.Is(err, afero.ErrFileNotFound) || flag&os.O_CREATE == 0 {
		return nil, errors.WithStack(err)
	}

	file, err = fs.Create(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if err := fs.Chmod(name, perm); err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Remove implements afero.Fs.
func (fs *Fs) Remove(name string) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		fileInfo, err := getFileInfo(conn, name)
		if err != nil {
			return errors.WithStack(err)
		}

		if fileInfo.IsDir() {
			if err := conn.RemoveDir(name); err != nil {
				return errors.WithStack(err)
			}
		} else {
			if err := conn.Delete(name); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// RemoveAll implements afero.Fs.
func (fs *Fs) RemoveAll(path string) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		fileInfo, err := getFileInfo(conn, path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}

			return errors.WithStack(err)
		}

		if fileInfo.IsDir() {
			if err := conn.RemoveDirRecur(path); err != nil {
				return errors.WithStack(err)
			}
		} else {
			if err := conn.Delete(path); err != nil {
				return errors.WithStack(err)
			}
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Rename implements afero.Fs.
func (fs *Fs) Rename(oldname string, newname string) error {
	err := fs.withConn(func(conn *ftp.ServerConn) error {
		if err := conn.Rename(oldname, newname); err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Stat implements afero.Fs.
func (fs *Fs) Stat(name string) (fs.FileInfo, error) {
	var (
		fileInfo *FileInfo
		err      error
	)

	err = fs.withConn(func(conn *ftp.ServerConn) error {
		fileInfo, err = getFileInfo(conn, name)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

func NewFs(withConn withConnFunc) *Fs {
	return &Fs{
		withConn: withConn,
	}
}

var _ afero.Fs = &Fs{}
