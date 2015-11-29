package vfs

import "io"
import "os"

type Filesystem interface {
	Open(name string) (File, error)
	Create(name string) (File, error)
	OpenFile(name string, flag int, perm os.FileMode) (File, error)

	Stat(name string) (os.FileInfo, error)
	Lstat(name string) (os.FileInfo, error)

	Remove(name string) error
	RemoveAll(path string) error

	Rename(oldPath, newPath string) error

	Mkdir(name string, perm os.FileMode) error
	MkdirAll(name string, perm os.FileMode) error

	Link(oldPath, newPath string) error
	Symlink(oldPath, newPath string) error
	Readlink(name string) (string, error)

	Chmod(name string, mode os.FileMode) error
	Chown(name string, uid, gid int) error
	Lchown(name string, uid, gid int) error

	Truncate(name string, size int64) error

	Close() error
}

type File interface {
	io.Closer
	io.Reader
	io.ReaderAt
	io.Writer
	io.WriterAt
	io.Seeker
	Stat() (os.FileInfo, error)
	Sync() error
	Truncate(int64) error
	Readdir(int) ([]os.FileInfo, error)
	Readdirnames(int) ([]string, error)
}
