package webdav

import (
	"bytes"
	"io/fs"
	"os"
	"sync"

	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/filesystem/backend/util"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
	"github.com/studio-b12/gowebdav"
)

type File struct {
	name       string
	withClient withClientFunc

	openWriterOnce sync.Once
	openWriterErr  error
	writer         *util.WriteBuffer

	openReaderOnce sync.Once
	openReaderErr  error
	reader         *bytes.Reader
}

// Close implements afero.File.
func (f *File) Close() error {
	f.reader = nil

	if f.writer != nil {
		err := f.withClient(func(client *gowebdav.Client) error {
			err := client.Write(f.name, f.writer.Bytes(), 0644)
			if err != nil {
				return errors.WithStack(err)
			}

			return nil
		})
		if err != nil {
			return errors.WithStack(err)
		}
	}

	return nil
}

// Name implements afero.File.
func (f *File) Name() string {
	return f.name
}

// Read implements afero.File.
func (f *File) Read(p []byte) (int, error) {
	reader, err := f.openReader()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	n, err := reader.Read(p)
	if err != nil {
		return n, wrapWebDavError(err)
	}

	return n, nil
}

// ReadAt implements afero.File.
func (f *File) ReadAt(p []byte, off int64) (int, error) {
	reader, err := f.openReader()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return reader.ReadAt(p, off)
}

// Readdir implements afero.File.
func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	var (
		fileInfos []os.FileInfo
		err       error
	)

	err = f.withClient(func(client *gowebdav.Client) error {
		fileInfos, err = client.ReadDir(f.name)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return fileInfos, nil
}

// Readdirnames implements afero.File.
func (f *File) Readdirnames(n int) ([]string, error) {
	var (
		names []string
		err   error
	)

	err = f.withClient(func(client *gowebdav.Client) error {
		fileInfos, err := client.ReadDir(f.name)
		if err != nil {
			return errors.WithStack(err)
		}

		names = make([]string, 0)
		for idx, info := range fileInfos {
			if n > 0 && idx >= n {
				break
			}

			names = append(names, info.Name())
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return names, nil
}

// Seek implements afero.File.
func (f *File) Seek(offset int64, whence int) (int64, error) {
	return 0, errors.WithStack(backend.ErrNotImplemented)
}

// Stat implements afero.File.
func (f *File) Stat() (fs.FileInfo, error) {
	var (
		stat os.FileInfo
		err  error
	)

	err = f.withClient(func(client *gowebdav.Client) error {
		stat, err = getFileInfo(client, f.name)
		if err != nil {
			return errors.WithStack(err)
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return stat, nil
}

// Sync implements afero.File.
func (f *File) Sync() error {
	return nil
}

// Truncate implements afero.File.
func (f *File) Truncate(size int64) error {
	return errors.WithStack(backend.ErrNotImplemented)
}

// Write implements afero.File.
func (f *File) Write(p []byte) (n int, err error) {
	writer, err := f.openWriter()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return writer.Write(p)
}

// WriteAt implements afero.File.
func (f *File) WriteAt(p []byte, off int64) (n int, err error) {
	writer, err := f.openWriter()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return writer.WriteAt(p, off)
}

// WriteString implements afero.File.
func (f *File) WriteString(s string) (int, error) {
	n, err := f.Write([]byte(s))
	if err != nil {
		return n, errors.WithStack(err)
	}

	return n, nil
}

func (f *File) openReader() (*bytes.Reader, error) {
	f.openReaderOnce.Do(func() {
		err := f.withClient(func(client *gowebdav.Client) error {
			data, err := client.Read(f.name)
			if err != nil {
				return errors.WithStack(err)
			}

			f.reader = bytes.NewReader(data)

			return nil
		})
		if err != nil {
			f.openReaderErr = errors.WithStack(err)
		}
	})
	if f.openReaderErr != nil {
		return nil, errors.WithStack(f.openReaderErr)
	}

	return f.reader, nil
}

func (f *File) openWriter() (*util.WriteBuffer, error) {
	f.openWriterOnce.Do(func() {
		f.writer = util.NewWriteBuffer()
	})
	if f.openWriterErr != nil {
		return nil, errors.WithStack(f.openWriterErr)
	}

	return f.writer, nil
}

var _ afero.File = &File{}
