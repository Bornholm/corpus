package minio

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/minio/minio-go/v7"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type File struct {
	ctx    context.Context
	client *minio.Client
	bucket string
	name   string
	mode   os.FileMode

	closed bool
	offset int64
	writer io.WriteCloser
	reader readerAtCloser
}

type readerAtCloser interface {
	io.ReaderAt
	io.Closer
}

// Close implements afero.File.
func (f *File) Close() error {
	if f.closed {
		return errors.WithStack(afero.ErrFileClosed)
	}

	f.closed = true

	if err := f.close(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

func (f *File) close() error {
	if f.writer != nil {
		if err := f.writer.Close(); err != nil {
			return errors.WithStack(err)
		}
		f.writer = nil
	}

	if f.reader != nil {
		if err := f.reader.Close(); err != nil {
			return errors.WithStack(err)
		}
		f.reader = nil
	}

	return nil
}

// Name implements afero.File.
func (f *File) Name() string {
	return f.name
}

// Read implements afero.File.
func (f *File) Read(p []byte) (n int, err error) {
	return f.ReadAt(p, f.offset)
}

// ReadAt implements afero.File.
func (f *File) ReadAt(p []byte, off int64) (n int, err error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	if cap(p) == 0 {
		return 0, nil
	}

	if off == f.offset && f.reader != nil {
		n, err = f.reader.ReadAt(p, off)
		f.offset += int64(n)
		return n, err
	}

	if err := f.close(); err != nil {
		return 0, errors.WithStack(err)
	}

	stat, err := f.Stat()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	opts := minio.GetObjectOptions{}
	r, err := f.client.GetObject(ctx, f.bucket, f.name, opts)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	f.reader = r
	f.offset = off

	buff := make([]byte, min(int(stat.Size()), len(p)))

	read, err := r.ReadAt(buff, off)
	f.offset += int64(read)

	if err == nil && len(p) > len(buff) {
		err = io.EOF
	}

	copy(p, buff)

	return read, err
}

// Readdir implements afero.File.
func (f *File) Readdir(count int) ([]os.FileInfo, error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	var res []os.FileInfo

	stat, err := f.Stat()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if !stat.IsDir() {
		return nil, syscall.ENOTDIR
	}

	prefix := f.name + separator

	opts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: false,
	}

	objs := f.client.ListObjects(ctx, f.bucket, opts)
	for obj := range objs {
		if obj.Key == prefix {
			continue
		}

		isDir := strings.HasSuffix(obj.Key, separator)
		res = append(res, &FileInfo{
			name:    filepath.Base(obj.Key),
			isDir:   isDir,
			modTime: obj.LastModified,
			mode:    defaultFileMode,
			size:    obj.Size,
		})
	}

	return res, nil
}

// Readdirnames implements afero.File.
func (f *File) Readdirnames(n int) ([]string, error) {
	fileInfos, err := f.Readdir(n)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	names := make([]string, len(fileInfos))
	for idx, stat := range fileInfos {
		names[idx] = stat.Name()
	}

	return names, nil
}

// Seek implements afero.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, errors.WithStack(afero.ErrFileClosed)
	}

	if (whence == 0 && offset == f.offset) || (whence == 1 && offset == 0) {
		return f.offset, nil
	}

	log.Printf("WARNING: Seek behavior triggered, highly inefficent. Offset before seek is at %d\n", f.offset)

	if err := f.Sync(); err != nil {
		return 0, errors.WithStack(err)
	}

	switch whence {
	case io.SeekStart:
		f.offset = offset
	case io.SeekCurrent:
		f.offset += offset
	case io.SeekEnd:
		stat, err := f.Stat()
		if err != nil {
			return 0, errors.WithStack(err)
		}

		f.offset = stat.Size() + offset
	}

	return f.offset, nil
}

// Stat implements afero.File.
func (f *File) Stat() (os.FileInfo, error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	fileInfo, err := stat(ctx, f.client, f.bucket, f.name)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fileInfo, nil
}

// Sync implements afero.File.
func (f *File) Sync() error {
	if err := f.close(); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// Truncate implements afero.File.
func (f *File) Truncate(size int64) error {
	return errors.WithStack(ErrNotSupported)
}

// Write implements afero.File.
func (f *File) Write(p []byte) (n int, err error) {
	ret, err := f.WriteAt(p, f.offset)
	if err != nil {
		return ret, errors.WithStack(err)
	}

	return ret, nil
}

// WriteAt implements afero.File.
func (f *File) WriteAt(b []byte, off int64) (n int, err error) {
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	if off == f.offset && f.writer != nil {
		n, err = f.writer.Write(b)
		f.offset += int64(n)
		return n, errors.WithStack(err)
	}

	if err := f.close(); err != nil {
		return 0, errors.WithStack(err)
	}

	f.offset = off

	buffer := bytes.NewReader(b)

	opts := minio.PutObjectOptions{
		ContentType: http.DetectContentType(b),
	}
	if off > 0 {
		opts.PartSize = uint64(off)
		opts.NumThreads = 8
		opts.ConcurrentStreamParts = false
		opts.DisableMultipart = true
	}
	_, err = f.client.PutObject(ctx, f.bucket, f.name, buffer, buffer.Size(), opts)
	if err != nil {
		return 0, errors.WithStack(err)
	}

	f.offset += int64(buffer.Len())

	return buffer.Len(), nil
}

// WriteString implements afero.File.
func (f *File) WriteString(s string) (int, error) {
	ret, err := f.Write([]byte(s))
	if err != nil {
		return ret, errors.WithStack(err)
	}

	return ret, nil
}

var _ afero.File = &File{}
