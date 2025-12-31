package smb

import (
	"io/fs"
	"time"

	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/hirochachacha/go-smb2"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type Fs struct {
	share *smb2.Share
}

// Chmod implements afero.Fs.
func (fs *Fs) Chmod(name string, mode fs.FileMode) error {
	if err := fs.share.Chmod(name, mode); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Chown implements afero.Fs.
func (fs *Fs) Chown(name string, uid int, gid int) error {
	return errors.WithStack(backend.ErrNotImplemented)
}

// Chtimes implements afero.Fs.
func (fs *Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	if err := fs.share.Chtimes(name, atime, mtime); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Create implements afero.Fs.
func (fs *Fs) Create(name string) (afero.File, error) {
	file, err := fs.share.Create(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Mkdir implements afero.Fs.
func (fs *Fs) Mkdir(name string, perm fs.FileMode) error {
	if err := fs.share.Mkdir(name, perm); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// MkdirAll implements afero.Fs.
func (fs *Fs) MkdirAll(path string, perm fs.FileMode) error {
	if err := fs.share.MkdirAll(path, perm); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Name implements afero.Fs.
func (fs *Fs) Name() string {
	return "smbfs"
}

// Open implements afero.Fs.
func (fs *Fs) Open(name string) (afero.File, error) {
	file, err := fs.share.Open(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// OpenFile implements afero.Fs.
func (fs *Fs) OpenFile(name string, flag int, perm fs.FileMode) (afero.File, error) {
	file, err := fs.share.OpenFile(name, flag, perm)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Remove implements afero.Fs.
func (fs *Fs) Remove(name string) error {
	if err := fs.share.Remove(name); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// RemoveAll implements afero.Fs.
func (fs *Fs) RemoveAll(path string) error {
	if err := fs.share.RemoveAll(path); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Rename implements afero.Fs.
func (fs *Fs) Rename(oldname string, newname string) error {
	if err := fs.share.Rename(oldname, newname); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Stat implements afero.Fs.
func (fs *Fs) Stat(name string) (fs.FileInfo, error) {
	fileInfo, err := fs.share.Stat(name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

func NewFs(share *smb2.Share) *Fs {
	return &Fs{share}
}

var _ afero.Fs = &Fs{}
