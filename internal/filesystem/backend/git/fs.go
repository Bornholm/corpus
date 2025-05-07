package git

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-billy/v5"
	"github.com/go-git/go-git/v5"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Fs struct {
	ctx  context.Context
	repo *git.Repository
}

func NewFs(ctx context.Context, repo *git.Repository) *Fs {
	return &Fs{ctx: ctx, repo: repo}
}

func (f *Fs) billy() (billy.Filesystem, error) {
	wt, err := f.repo.Worktree()
	if err != nil {
		return nil, err
	}
	return wt.Filesystem, nil
}

// Name implements afero.Fs
func (f *Fs) Name() string {
	return "git"
}

func (f *Fs) Chmod(name string, mode os.FileMode) error {
	return errors.WithStack(ErrNotSupported)
}

// Chown not supported by billy
func (f *Fs) Chown(name string, uid int, gid int) error {
	return errors.WithStack(ErrNotSupported)
}

// Chtimes not supported by billy
func (f *Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return errors.WithStack(ErrNotSupported)
}

func (f *Fs) Mkdir(name string, perm os.FileMode) error {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return errors.WithStack(err)
	}

	return fsys.MkdirAll(name, perm)
}

func (f *Fs) MkdirAll(path string, perm os.FileMode) error {
	path = strings.TrimPrefix(path, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return errors.WithStack(err)
	}
	return fsys.MkdirAll(path, perm)
}

func (f *Fs) Open(name string) (afero.File, error) {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if name == "." {
		return &file{fs: fsys, f: nil, path: "."}, nil
	}

	stat, err := f.Stat(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if stat.IsDir() {
		return &file{fs: fsys, f: nil, path: name}, nil
	}

	bf, err := fsys.Open(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return &file{fs: fsys, f: bf, path: name}, nil
}

func (f *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bf, err := fsys.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &file{fs: fsys, f: bf, path: name}, nil
}

func (f *Fs) Create(name string) (afero.File, error) {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return nil, errors.WithStack(err)
	}
	bf, err := fsys.Create(name)
	if err != nil {
		return nil, err
	}
	return &file{fs: fsys, f: bf, path: name}, nil
}

func (f *Fs) Remove(name string) error {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return errors.WithStack(err)
	}
	return fsys.Remove(name)
}

func (f *Fs) Rename(oldname, newname string) error {
	oldname = strings.TrimPrefix(oldname, string(os.PathSeparator))
	newname = strings.TrimPrefix(newname, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return errors.WithStack(err)
	}
	return fsys.Rename(oldname, newname)
}

func (f *Fs) RemoveAll(path string) error {
	fsys, err := f.billy()
	if err != nil {
		return errors.WithStack(err)
	}

	path = strings.TrimPrefix(path, string(os.PathSeparator))

	infos, _ := fsys.ReadDir(path)
	for _, info := range infos {
		p := filepath.Join(path, info.Name())
		if info.IsDir() {
			if err := f.RemoveAll(p); err != nil {
				return err
			}
		} else {
			if err := fsys.Remove(p); err != nil {
				return err
			}
		}
	}

	if err := fsys.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	return nil
}

func (f *Fs) Stat(name string) (os.FileInfo, error) {
	name = strings.TrimPrefix(name, string(os.PathSeparator))

	fsys, err := f.billy()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	stat, err := fsys.Stat(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return stat, nil
}

type file struct {
	fs         billy.Filesystem
	path       string
	f          billy.File
	dirEntries []os.FileInfo
	dirOffset  int
}

// WriteString implements afero.File.
func (f *file) WriteString(s string) (ret int, err error) {
	return f.Write([]byte(s))
}

func (f *file) Close() error {
	if f.f == nil {
		return nil
	}

	return f.f.Close()
}

func (f *file) Read(p []byte) (int, error) {
	return f.f.Read(p)
}

func (f *file) ReadAt(p []byte, off int64) (int, error) {
	return f.f.ReadAt(p, off)
}

func (f *file) Write(p []byte) (int, error) {
	return f.f.Write(p)
}

func (f *file) WriteAt(p []byte, off int64) (int, error) {
	cur, err := f.f.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, err
	}

	if _, err := f.f.Seek(off, io.SeekStart); err != nil {
		return 0, err
	}

	n, err := f.f.Write(p)

	if _, seekErr := f.f.Seek(cur, io.SeekStart); seekErr != nil {
		return n, seekErr
	}

	return n, err
}

func (f *file) Seek(offset int64, whence int) (int64, error) {
	return f.f.Seek(offset, whence)
}

func (f *file) Name() string {
	return filepath.Base(f.path)
}

func (f *file) Readdir(count int) ([]os.FileInfo, error) {
	// load once
	if f.dirEntries == nil {
		var err error
		f.dirEntries, err = f.fs.ReadDir(f.path)
		if err != nil {
			return nil, err
		}
	}
	if count <= 0 {
		// return all
		return f.dirEntries, nil
	}
	if f.dirOffset >= len(f.dirEntries) {
		return nil, io.EOF
	}
	end := f.dirOffset + count
	if end > len(f.dirEntries) {
		end = len(f.dirEntries)
	}
	slice := f.dirEntries[f.dirOffset:end]
	f.dirOffset = end
	return slice, nil
}

func (f *file) Readdirnames(n int) ([]string, error) {
	fis, err := f.Readdir(n)
	if err != nil && err != io.EOF {
		return nil, err
	}
	names := make([]string, len(fis))
	for i, info := range fis {
		names[i] = info.Name()
	}
	return names, nil
}

func (f *file) Stat() (os.FileInfo, error) {
	stat, err := f.fs.Stat(f.path)
	if err != nil {
		return nil, err
	}

	return stat, nil
}

func (f *file) Sync() error {
	return errors.WithStack(ErrNotSupported)
}

func (f *file) Truncate(size int64) error {
	return errors.WithStack(ErrNotSupported)
}

var _ afero.Fs = &Fs{}
var _ afero.File = &file{}
