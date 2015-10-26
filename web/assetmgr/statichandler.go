package assetmgr

import "net/http"
import "os"
import "time"
import "fmt"
import "strings"

var ErrNotFound = fmt.Errorf("static file not found")
var ErrUnsupportedMethod = fmt.Errorf("unsupported method")

func stripCachebuster(path string) (strippedPath string, hadCachebuster bool) {
	if !strings.HasPrefix(path, "/c=") {
		return path, false
	}

	rest := path[3:]
	idx := strings.IndexByte(rest, '/')
	if idx < 0 {
		return path, false
	}

	return rest[idx:], true
}

func (m *Manager) TryHandle(rw http.ResponseWriter, req *http.Request) error {
	if req.Method != "GET" && req.Method != "HEAD" {
		return ErrUnsupportedMethod
	}

	path := req.URL.Path // "/img/x.png"
	path, hadCachebuster := stripCachebuster(path)

	info := m.Info(path[1:])
	if info == nil {
		// May as well not bother with anything the asset manager can't find.  The
		// asset manager ensures that the final path is within the static root.
		return ErrNotFound
	}

	fpath := info.FullPath()
	f, err := os.Open(fpath)
	if err != nil {
		return err
	}

	defer f.Close()
	rw.Header().Set("Vary", "Accept-Encoding")
	if hadCachebuster {
		rw.Header().Set("Expires", time.Now().Add(28*24*time.Hour).UTC().Format(time.RFC1123))
		rw.Header().Set("Cache-Control", "public, max-age=2419200")
	}

	rw.Header().Del("Content-Security-Policy")

	http.ServeContent(rw, req, fpath, info.ModTime(), f)
	return nil
}
