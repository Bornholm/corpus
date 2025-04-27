package ftp

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/jlaffaye/ftp"
	"github.com/pkg/errors"
	"github.com/spf13/afero"
)

type FileInfo struct {
	entry *ftp.Entry
}

// IsDir implements fs.FileInfo.
func (i *FileInfo) IsDir() bool {
	return i.entry.Type == ftp.EntryTypeFolder
}

// ModTime implements fs.FileInfo.
func (i *FileInfo) ModTime() time.Time {
	return i.entry.Time
}

// Mode implements fs.FileInfo.
func (i *FileInfo) Mode() fs.FileMode {
	return fs.ModePerm
}

// Name implements fs.FileInfo.
func (i *FileInfo) Name() string {
	return i.entry.Name
}

// Size implements fs.FileInfo.
func (i *FileInfo) Size() int64 {
	return int64(i.entry.Size)
}

// Sys implements fs.FileInfo.
func (*FileInfo) Sys() any {
	return nil
}

var _ fs.FileInfo = &FileInfo{}

func getFileInfo(conn *ftp.ServerConn, path string) (*FileInfo, error) {
	entry, err := conn.GetEntry(path)
	if err != nil && !isNotImplementedErr(err) && !isFileUnavailableErr(err) {
		return nil, errors.WithStack(err)
	}

	if entry != nil {
		return &FileInfo{entry}, nil
	}

	parentDir := filepath.Join(path, "..")

	siblings, err := conn.List(parentDir)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	for _, s := range siblings {
		if s != nil && s.Name == filepath.Base(path) {
			return &FileInfo{entry: s}, nil
		}
	}

	entry = &ftp.Entry{
		Name:   path,
		Target: "",
		Type:   0,
		Size:   0,
		Time:   time.Time{},
	}

	time, err := conn.GetTime(path)
	if err != nil {
		if isFileUnavailableErr(err) {
			return nil, os.ErrNotExist
		}

		return nil, errors.WithStack(err)
	}

	entry.Time = time

	size, err := conn.FileSize(path)
	if err != nil {
		if isFileUnavailableErr(err) {
			return nil, errors.WithStack(afero.ErrFileNotFound)
		}

		return nil, errors.WithStack(err)
	}

	entry.Size = uint64(size)

	if entry.Size > 0 {
		entry.Type = ftp.EntryTypeFile
	} else {
		entry.Type = ftp.EntryTypeFolder
	}

	return &FileInfo{entry}, nil
}
