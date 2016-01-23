package os

import "os"
import "path/filepath"

// Returns a directory suitable for storing cached data.
//
// On Windows, this is the 'local' appdata directory. On *NIX,
// it is XDG_CACHE_DIR (or failing that, $HOME/.cache).
//
// Returns "" if suitable path cannot be identified.
func CacheDir() string {
	d, ok := os.LookupEnv("XDG_CACHE_DIR")
	if ok {
		return d
	}

	d, ok = os.LookupEnv("LOCALAPPDATA")
	if ok {
		return d
	}

	d, ok = os.LookupEnv("HOME")
	if ok {
		return filepath.Join(d, ".cache")
	}

	return ""
}

// Returns a filename under the directory returned by CacheDir.
func CacheFilename(filename string) string {
	d := CacheDir()
	if d == "" {
		return ""
	}

	return filepath.Join(d, filename)
}
