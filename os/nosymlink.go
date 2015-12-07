package os

import "os"

func OpenFileNoSymlinks(path string, flags int, mode os.FileMode) (*os.File, error) {
	return openFileNoSymlinks(path, flags, mode)
}

func OpenNoSymlinks(path string) (*os.File, error) {
	return OpenFileNoSymlinks(path, os.O_RDONLY, 0)
}
