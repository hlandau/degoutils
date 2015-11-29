package vfs

import "os"

// Default filesystem. May be changed.
//
// Defaults to RealRel(".").
var Default Filesystem

func init() {
	Default, _ = RealRel(".")
}

// Helper functions.

func Open(name string) (File, error) {
	return Default.Open(name)
}

func Create(name string) (File, error) {
	return Default.Create(name)
}

func OpenFile(name string, flag int, perm os.FileMode) (File, error) {
	return Default.OpenFile(name, flag, perm)
}

func Stat(name string) (os.FileInfo, error) {
	return Default.Stat(name)
}

func Lstat(name string) (os.FileInfo, error) {
	return Default.Lstat(name)
}

func Remove(name string) error {
	return Default.Remove(name)
}

func RemoveAll(path string) error {
	return Default.RemoveAll(path)
}

func Rename(oldPath, newPath string) error {
	return Default.Rename(oldPath, newPath)
}

func Mkdir(name string, perm os.FileMode) error {
	return Default.Mkdir(name, perm)
}

func MkdirAll(name string, perm os.FileMode) error {
	return Default.MkdirAll(name, perm)
}

func Link(oldPath, newPath string) error {
	return Default.Link(oldPath, newPath)
}

func Symlink(oldPath, newPath string) error {
	return Default.Symlink(oldPath, newPath)
}

func Readlink(name string) (string, error) {
	return Default.Readlink(name)
}

func Chmod(name string, mode os.FileMode) error {
	return Default.Chmod(name, mode)
}

func Chown(name string, uid, gid int) error {
	return Default.Chown(name, uid, gid)
}

func Lchown(name string, uid, gid int) error {
	return Default.Lchown(name, uid, gid)
}

func Truncate(name string, size int64) error {
	return Default.Truncate(name, size)
}
