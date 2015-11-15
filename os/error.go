package os

import "errors"

var ErrNotEmpty = errors.New("directory not empty")

// Test for ENOTEMPTY.
func IsNotEmpty(err error) bool {
	return isNotEmpty(err)
}
