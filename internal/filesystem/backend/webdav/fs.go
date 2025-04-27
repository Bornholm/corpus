package webdav

import (
	"io/fs"
	"os"
	"time"

	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

type withClientFunc func(fn func(client *gowebdav.Client) error) error

type Fs struct {
	withClient withClientFunc
}

// Chmod implements afero.Fs.
func (fs *Fs) Chmod(name string, mode fs.FileMode) error {
	return nil
}

// Chown implements afero.Fs.
func (fs *Fs) Chown(name string, uid int, gid int) error {
	return errors.WithStack(backend.ErrNotImplemented)
}

// Chtimes implements afero.Fs.
func (fs *Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return nil
}

// Create implements afero.Fs.
func (fs *Fs) Create(name string) (afero.File, error) {
	var file *File
	err := fs.withClient(func(client *gowebdav.Client) error {
		err := client.Write(name, nil, 0644)
		if err != nil {
			return errors.WithStack(err)
		}

		file = &File{withClient: fs.withClient, name: name}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Mkdir implements afero.Fs.
func (fs *Fs) Mkdir(name string, perm fs.FileMode) error {
	err := fs.withClient(func(client *gowebdav.Client) error {
		if err := client.Mkdir(name, perm); err != nil {
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
	err := fs.withClient(func(client *gowebdav.Client) error {
		if err := client.MkdirAll(path, perm); err != nil {
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
	return "webdavfs"
}

// Open implements afero.Fs.
func (fs *Fs) Open(name string) (afero.File, error) {
	var file *File
	err := fs.withClient(func(client *gowebdav.Client) error {
		_, err := getFileInfo(client, name)
		if err != nil {
			return errors.WithStack(err)
		}

		file = &File{withClient: fs.withClient, name: name}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// OpenFile implements afero.Fs.
func (fs *Fs) OpenFile(name string, flag int, perm fs.FileMode) (afero.File, error) {
	var file *File
	err := fs.withClient(func(client *gowebdav.Client) error {
		fileInfo, err := getFileInfo(client, name)
		if err != nil {
			if flag&os.O_CREATE != 0 {
				file = &File{
					withClient: fs.withClient,
					name:       name,
				}

				return nil
			}

			return errors.WithStack(err)
		}

		if fileInfo.IsDir() {
			return errors.Errorf("entry '%s' is a directory", name)
		}

		file = &File{
			withClient: fs.withClient,
			name:       name,
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Remove implements afero.Fs.
func (fs *Fs) Remove(name string) error {
	err := fs.withClient(func(client *gowebdav.Client) error {
		if err := client.Remove(name); err != nil {
			return errors.WithStack(err)
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
	err := fs.withClient(func(client *gowebdav.Client) error {
		if err := client.RemoveAll(path); err != nil {
			return errors.WithStack(err)
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
	err := fs.withClient(func(client *gowebdav.Client) error {
		if err := client.Rename(oldname, newname, true); err != nil {
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
		fileInfo os.FileInfo
		err      error
	)

	err = fs.withClient(func(client *gowebdav.Client) error {
		fileInfo, err = getFileInfo(client, name)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if fileInfo == nil {
		return nil, os.ErrNotExist
	}

	return fileInfo, nil
}

func NewFs(withClient withClientFunc) *Fs {
	return &Fs{withClient}
}

var _ afero.Fs = &Fs{}

func getFileInfo(client *gowebdav.Client, path string) (*FileInfo, error) {
	stat, err := client.Stat(path)
	if err != nil {
		// Targeted file is a directory
		if isWebDavErr(err, "PROPFIND", 200) {
			stat, err := client.Stat(gowebdav.FixSlashes(path))
			if err != nil {
				return nil, errors.WithStack(err)
			}

			return fromFileInfo(path, stat), nil
		}

		if isWebDavErr(err, "PROPFIND", 404) {
			return nil, &os.PathError{
				Op:   "PROPFIND",
				Path: path,
				Err:  os.ErrNotExist,
			}
		}

		return nil, errors.WithStack(err)
	}

	return fromFileInfo(path, stat), nil
}
