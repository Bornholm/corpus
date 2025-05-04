package filesystem

import (
	"log/slog"
	"os"
	"time"

	"github.com/spf13/afero"
)

type LoggerFunc func(message string, attrs ...slog.Attr)

type FsLogger struct {
	fs  afero.Fs
	log LoggerFunc
}

// Chmod implements afero.Fs.
func (l *FsLogger) Chmod(name string, mode os.FileMode) error {
	l.log("FS.Chmod", slog.String("name", name), slog.Any("mode", mode))
	return l.fs.Chmod(name, mode)
}

// Chown implements afero.Fs.
func (l *FsLogger) Chown(name string, uid int, gid int) error {
	l.log("FS.Chown", slog.String("name", name), slog.Int("uid", uid), slog.Int("gid", gid))
	return l.fs.Chown(name, uid, gid)
}

// Chtimes implements afero.Fs.
func (l *FsLogger) Chtimes(name string, atime time.Time, mtime time.Time) error {
	l.log("FS.Chtimes", slog.String("name", name), slog.Time("atime", atime), slog.Time("mtime", mtime))
	return l.fs.Chtimes(name, atime, mtime)
}

// Create implements afero.Fs.
func (l *FsLogger) Create(name string) (afero.File, error) {
	l.log("FS.Create", slog.String("name", name))
	file, err := l.fs.Create(name)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: file, log: l.log}, nil
}

// Mkdir implements afero.Fs.
func (l *FsLogger) Mkdir(name string, perm os.FileMode) error {
	l.log("FS.Mkdir", slog.String("name", name), slog.Any("perm", perm))
	return l.fs.Mkdir(name, perm)
}

// MkdirAll implements afero.Fs.
func (l *FsLogger) MkdirAll(path string, perm os.FileMode) error {
	l.log("FS.MkdirAll", slog.String("path", path), slog.Any("perm", perm))
	return l.fs.MkdirAll(path, perm)
}

// Name implements afero.Fs.
func (l *FsLogger) Name() string {
	return l.fs.Name()
}

// Open implements afero.Fs.
func (l *FsLogger) Open(name string) (afero.File, error) {
	l.log("FS.Open", slog.String("name", name))
	file, err := l.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: file, log: l.log}, nil
}

// OpenFile implements afero.Fs.
func (l *FsLogger) OpenFile(name string, flag int, perm os.FileMode) (afero.File, error) {
	l.log("FS.OpenFile", slog.String("name", name), slog.Int("flag", flag), slog.Any("perm", perm))
	file, err := l.fs.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	return &FileLogger{file: file, log: l.log}, nil
}

// Remove implements afero.Fs.
func (l *FsLogger) Remove(name string) error {
	l.log("FS.Remove", slog.String("name", name))
	return l.fs.Remove(name)
}

// RemoveAll implements afero.Fs.
func (l *FsLogger) RemoveAll(path string) error {
	l.log("FS.RemoveAll", slog.String("path", path))
	return l.fs.RemoveAll(path)
}

// Rename implements afero.Fs.
func (l *FsLogger) Rename(oldname string, newname string) error {
	l.log("FS.Rename", slog.String("oldname", oldname), slog.String("newname", newname))
	return l.fs.Rename(oldname, newname)
}

// Stat implements afero.Fs.
func (l *FsLogger) Stat(name string) (os.FileInfo, error) {
	l.log("FS.Stat", slog.String("name", name))
	return l.fs.Stat(name)
}

var _ afero.Fs = &FsLogger{}

func NewLogger(fs afero.Fs, log LoggerFunc) *FsLogger {
	return &FsLogger{
		fs:  fs,
		log: log,
	}
}

type FileLogger struct {
	file afero.File
	log  LoggerFunc
}

// Close implements afero.File.
func (l *FileLogger) Close() error {
	l.log("File.Close", slog.String("name", l.file.Name()))
	return l.file.Close()
}

// Name implements afero.File.
func (l *FileLogger) Name() string {
	return l.file.Name()
}

// Read implements afero.File.
func (l *FileLogger) Read(p []byte) (n int, err error) {
	l.log("File.Read", slog.Any("bufferSize", len(p)), slog.String("name", l.file.Name()))
	return l.file.Read(p)
}

// ReadAt implements afero.File.
func (l *FileLogger) ReadAt(p []byte, off int64) (n int, err error) {
	l.log("File.ReadAt", slog.Any("bufferSize", len(p)), slog.Int64("offset", off), slog.String("name", l.file.Name()))
	return l.file.ReadAt(p, off)
}

// Readdir implements afero.File.
func (l *FileLogger) Readdir(count int) ([]os.FileInfo, error) {
	l.log("File.Readdir", slog.Int("count", count), slog.String("name", l.file.Name()))
	return l.file.Readdir(count)
}

// Readdirnames implements afero.File.
func (l *FileLogger) Readdirnames(n int) ([]string, error) {
	l.log("File.Readdirnames", slog.Int("n", n), slog.String("name", l.file.Name()))
	return l.file.Readdirnames(n)
}

// Seek implements afero.File.
func (l *FileLogger) Seek(offset int64, whence int) (int64, error) {
	l.log("File.Seek", slog.Int64("offset", offset), slog.Int("whence", whence), slog.String("name", l.file.Name()))
	return l.file.Seek(offset, whence)
}

// Stat implements afero.File.
func (l *FileLogger) Stat() (os.FileInfo, error) {
	l.log("File.Stat", slog.String("name", l.file.Name()))
	return l.file.Stat()
}

// Sync implements afero.File.
func (l *FileLogger) Sync() error {
	l.log("File.Sync", slog.String("name", l.file.Name()))
	return l.file.Sync()
}

// Truncate implements afero.File.
func (l *FileLogger) Truncate(size int64) error {
	l.log("File.Truncate", slog.Int64("size", size), slog.String("name", l.file.Name()))
	return l.file.Truncate(size)
}

// Write implements afero.File.
func (l *FileLogger) Write(p []byte) (n int, err error) {
	l.log("File.Write", slog.Int("bufferSize", len(p)), slog.String("name", l.file.Name()))
	return l.file.Write(p)
}

// WriteAt implements afero.File.
func (l *FileLogger) WriteAt(p []byte, off int64) (n int, err error) {
	l.log("File.WriteAt", slog.Int("bufferSize", len(p)), slog.Int64("offset", off), slog.String("name", l.file.Name()))
	return l.file.WriteAt(p, off)
}

// WriteString implements afero.File.
func (l *FileLogger) WriteString(s string) (ret int, err error) {
	l.log("File.WriteString", slog.Int("strSize", len(s)), slog.String("name", l.file.Name()))
	return l.file.WriteString(s)
}

var _ afero.File = &FileLogger{}
