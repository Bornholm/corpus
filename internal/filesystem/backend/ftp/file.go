package ftp

import (
	"bytes"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"sync"

	"github.com/bornholm/corpus/internal/filesystem/backend"
	"github.com/bornholm/corpus/internal/filesystem/backend/util"
	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type File struct {
	reader      *bytes.Reader
	readOnce    sync.Once
	readOnceErr error

	writer       *util.WriteBuffer
	writeOnce    sync.Once
	writeOnceErr error

	withConn withConnFunc
	name     string
}

// Close implements afero.File.
func (f *File) Close() error {
	if f.reader != nil {
		f.reader.Reset([]byte{})
	}
	f.readOnceErr = errors.WithStack(os.ErrClosed)

	writer := f.writer
	f.writer = nil
	f.writeOnceErr = errors.WithStack(os.ErrClosed)

	if writer != nil {
		err := f.withConn(func(conn *ftp.ServerConn) error {
			buff := bytes.NewBuffer(writer.Bytes())
			if err := conn.Stor(f.name, buff); err != nil {
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
	reader, err := f.getBufferedReader()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return reader.Read(p)
}

// ReadAt implements afero.File.
func (f *File) ReadAt(p []byte, off int64) (int, error) {
	reader, err := f.getBufferedReader()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return reader.ReadAt(p, off)
}

// Readdir implements afero.File.
func (f *File) Readdir(count int) ([]fs.FileInfo, error) {
	var fileInfos []fs.FileInfo

	err := f.withConn(func(conn *ftp.ServerConn) error {
		entries, err := conn.List(f.name)
		if err != nil {
			return errors.WithStack(err)
		}

		fileInfos = make([]fs.FileInfo, len(entries))
		for idx, entry := range entries {
			fileInfos[idx] = &FileInfo{entry}
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
	var names []string

	err := f.withConn(func(conn *ftp.ServerConn) error {
		entries, err := conn.List(f.name)
		if err != nil {
			return errors.WithStack(err)
		}

		names = make([]string, 0)
		for idx, entry := range entries {
			if n > 0 && idx >= n {
				break
			}

			names = append(names, entry.Name)
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
	reader, err := f.getBufferedReader()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return reader.Seek(offset, whence)
}

// Stat implements afero.File.
func (f *File) Stat() (fs.FileInfo, error) {
	var (
		fileInfo *FileInfo
		err      error
	)

	err = f.withConn(func(conn *ftp.ServerConn) error {
		fileInfo, err = getFileInfo(conn, f.name)
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

// Sync implements afero.File.
func (f *File) Sync() error {
	return nil
}

// Truncate implements afero.File.
func (f *File) Truncate(size int64) error {
	return errors.WithStack(backend.ErrNotImplemented)
}

// Write implements afero.File.
func (f *File) Write(p []byte) (int, error) {
	writer, err := f.getBufferedWriter()
	if err != nil {
		return 0, errors.WithStack(err)
	}

	return writer.Write(p)
}

// WriteAt implements afero.File.
func (f *File) WriteAt(p []byte, off int64) (int, error) {
	writer, err := f.getBufferedWriter()
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

func (f *File) getBufferedReader() (*bytes.Reader, error) {
	f.readOnce.Do(func() {
		err := f.withConn(func(conn *ftp.ServerConn) error {
			res, err := conn.Retr(f.name)
			if err != nil {
				return errors.WithStack(err)
			}

			defer func() {
				if err := res.Close(); err != nil {
					err = errors.WithStack(err)
					slog.Error("could not close response", slog.Any("error", err))
				}
			}()

			data, err := io.ReadAll(res)
			if err != nil {
				return errors.WithStack(err)
			}

			f.reader = bytes.NewReader(data)

			return nil
		})
		if err != nil {
			f.readOnceErr = errors.WithStack(err)
		}
	})
	if f.readOnceErr != nil {
		return nil, errors.WithStack(f.readOnceErr)
	}

	return f.reader, nil
}

func (f *File) getBufferedWriter() (*util.WriteBuffer, error) {
	f.writeOnce.Do(func() {
		f.writer = util.NewWriteBuffer()
	})
	if f.writeOnceErr != nil {
		return nil, errors.WithStack(f.readOnceErr)
	}

	return f.writer, nil
}

var _ afero.File = &File{}
