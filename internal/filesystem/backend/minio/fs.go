package minio

import (
	"context"
	"os"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

const (
	defaultFileMode = 0o755
	separator       = "/"
)

type Fs struct {
	ctx    context.Context
	client *minio.Client
	bucket string
}

// Chmod implements afero.Fs.
func (f *Fs) Chmod(name string, mode os.FileMode) error {
	return errors.WithStack(ErrNotSupported)
}

// Chown implements afero.Fs.
func (f *Fs) Chown(name string, uid int, gid int) error {
	return errors.WithStack(ErrNotSupported)
}

// Chtimes implements afero.Fs.
func (f *Fs) Chtimes(name string, atime time.Time, mtime time.Time) error {
	return errors.WithStack(ErrNotSupported)
}

// Create implements afero.Fs.
func (f *Fs) Create(name string) (afero.File, error) {
	file, err := f.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// Mkdir implements afero.Fs.
func (f *Fs) Mkdir(name string, perm os.FileMode) error {
	panic("unimplemented")
}

// MkdirAll implements afero.Fs.
func (f *Fs) MkdirAll(path string, perm os.FileMode) error {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	prefix := path
	if !strings.HasSuffix(prefix, separator) {
		prefix += separator
	}

	if _, err := f.client.PutObject(ctx, f.bucket, prefix, nil, 0, minio.PutObjectOptions{}); err != nil {
		return nil
	}

	return nil
}

// Name implements afero.Fs.
func (f *Fs) Name() string {
	return "minio"
}

// Open implements afero.Fs.
func (f *Fs) Open(name string) (afero.File, error) {
	file, err := f.OpenFile(name, os.O_RDONLY, 0)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return file, nil
}

// OpenFile implements afero.Fs.
func (f *Fs) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	if flag&os.O_APPEND != 0 {
		return nil, errors.WithStack(ErrNotSupported)
	}

	file := &File{
		ctx:    f.ctx,
		client: f.client,
		bucket: f.bucket,
		name:   name,
		mode:   perm,
		closed: false,
	}

	if flag&os.O_CREATE != 0 {
		if _, err := file.WriteString(""); err != nil {
			return nil, errors.WithStack(err)
		}
	}

	return file, nil
}

// Remove implements afero.Fs.
func (f *Fs) Remove(name string) error {
	if name == "." {
		return errors.WithStack(ErrNotSupported)
	}

	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	err := f.client.RemoveObject(ctx, f.bucket, name, minio.RemoveObjectOptions{
		GovernanceBypass: true,
		ForceDelete:      true,
	})
	if err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// RemoveAll implements afero.Fs.
func (f *Fs) RemoveAll(path string) error {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	objects := f.client.ListObjects(ctx, f.bucket, minio.ListObjectsOptions{
		Prefix: path,
	})
	for obj := range objects {
		if obj.Err != nil {
			return errors.WithStack(obj.Err)
		}

		if !strings.HasPrefix(obj.Key, path) {
			return nil
		}

		if err := f.client.RemoveObject(ctx, f.bucket, obj.Key, minio.RemoveObjectOptions{
			GovernanceBypass: true,
			ForceDelete:      true,
		}); err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// Rename implements afero.Fs.
func (f *Fs) Rename(oldname string, newname string) error {
	panic("unimplemented")
}

// Stat implements afero.Fs.
func (f *Fs) Stat(name string) (os.FileInfo, error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	fileInfo, err := stat(ctx, f.client, f.bucket, name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

func NewFs(ctx context.Context, client *minio.Client, bucket string) *Fs {
	return &Fs{ctx, client, bucket}
}

var _ afero.Fs = &Fs{}

func stat(ctx context.Context, client *minio.Client, bucket string, name string) (os.FileInfo, error) {
	if name == "." {
		return &FileInfo{
			isDir:   true,
			modTime: time.Time{},
			mode:    0,
			name:    name,
			size:    0,
		}, nil
	}

	stat, err := client.StatObject(ctx, bucket, name, minio.GetObjectOptions{})
	if err != nil {
		errRes := minio.ToErrorResponse(err)
		if errRes.Code == "NoSuchKey" {
			fileInfo, err := statDir(ctx, client, bucket, name)
			if err != nil {
				return nil, errors.WithStack(err)
			}

			return fileInfo, nil
		}

		return nil, errors.WithStack(err)
	}

	return &FileInfo{
		isDir:   false,
		modTime: stat.LastModified,
		mode:    defaultFileMode,
		name:    name,
		size:    stat.Size,
	}, nil
}

func statDir(ctx context.Context, client *minio.Client, bucket string, name string) (os.FileInfo, error) {
	prefix := name + separator

	objects := client.ListObjects(ctx, bucket, minio.ListObjectsOptions{
		StartAfter: name,
	})
	for obj := range objects {
		if obj.Err != nil {
			return nil, errors.WithStack(obj.Err)
		}

		if obj.Key != name && obj.Key != prefix {
			continue
		}

		if obj.Key == name {
			return &FileInfo{
				isDir:   false,
				modTime: obj.LastModified,
				mode:    defaultFileMode,
				name:    name,
				size:    obj.Size,
			}, nil
		}

		if obj.Key == prefix {
			return &FileInfo{
				isDir:   true,
				modTime: obj.LastModified,
				mode:    0,
				name:    name,
				size:    obj.Size,
			}, nil
		}

	}

	return nil, errors.Wrapf(afero.ErrFileNotFound, "could not find file '%s'", name)
}
