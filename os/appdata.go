package os

import "os"
import "path/filepath"

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

func CacheFilename(filename string) string {
	d := CacheDir()
	if d == "" {
		return ""
	}

	return filepath.Join(d, filename)
}
