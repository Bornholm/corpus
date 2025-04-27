package webdav

import (
	"io/fs"
	"path/filepath"
	"time"
)

type FileInfo struct {
	isDir   bool
	modTime time.Time
	mode    fs.FileMode
	name    string
	size    int64
	sys     any
}

// IsDir implements fs.FileInfo.
func (fi *FileInfo) IsDir() bool {
	return fi.isDir
}

// ModTime implements fs.FileInfo.
func (fi *FileInfo) ModTime() time.Time {
	return fi.modTime
}

// Mode implements fs.FileInfo.
func (fi *FileInfo) Mode() fs.FileMode {
	return fi.mode
}

// Name implements fs.FileInfo.
func (fi *FileInfo) Name() string {
	return fi.name
}

// Size implements fs.FileInfo.
func (fi *FileInfo) Size() int64 {
	return fi.size
}

// Sys implements fs.FileInfo.
func (fi *FileInfo) Sys() any {
	return fi.sys
}

var _ fs.FileInfo = &FileInfo{}

func fromFileInfo(path string, stat fs.FileInfo) *FileInfo {
	fileInfo := &FileInfo{
		isDir:   stat.IsDir(),
		modTime: stat.ModTime().UTC().Round(0),
		mode:    stat.Mode(),
		name:    stat.Name(),
		size:    stat.Size(),
		sys:     stat.Sys(),
	}

	if fileInfo.name == "" {
		fileInfo.name = filepath.Base(path)
	}

	return fileInfo
}
