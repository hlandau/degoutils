// +build linux freebsd openbsd netbsd dragonfly solaris darwin

package os

import (
	"os"
	"syscall"
)

func openNoSymlinks(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
}
