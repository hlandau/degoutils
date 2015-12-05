package os

import "os"

func OpenNoSymlinks(path string) (*os.File, error) {
	return openNoSymlinks(path)
}
