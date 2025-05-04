package minio

import (
	"io/fs"
	"os"
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
func (f *FileInfo) IsDir() bool {
	return f.isDir
}

// ModTime implements fs.FileInfo.
func (f *FileInfo) ModTime() time.Time {
	return f.modTime
}

// Mode implements fs.FileInfo.
func (f *FileInfo) Mode() fs.FileMode {
	return f.mode
}

// Name implements fs.FileInfo.
func (f *FileInfo) Name() string {
	return f.name
}

// Size implements fs.FileInfo.
func (f *FileInfo) Size() int64 {
	return f.size
}

// Sys implements fs.FileInfo.
func (f *FileInfo) Sys() any {
	return f.sys
}

var _ os.FileInfo = &FileInfo{}
