package assetmgr

import "net/http"
import "time"
import "fmt"
import "strings"
import "github.com/hlandau/degoutils/vfs"

var ErrNotFound = fmt.Errorf("static file not found")
var ErrUnsupportedMethod = fmt.Errorf("unsupported method")

// Strip any cachebuster prefix from the URL. Return the stripped path and the
// cachebuster tag value, if any.
func stripCachebuster(path string) (strippedPath, cachebuster string) {
	if !strings.HasPrefix(path, "/.c=") {
		return path, ""
	}

	rest := path[4:]
	idx := strings.IndexByte(rest, '/')
	if idx < 0 {
		return path, ""
	}

	return rest[idx:], rest[0:idx]
}

// Serve static files from assets.
func (m *Manager) TryHandle(rw http.ResponseWriter, req *http.Request) error {
	if req.Method != "GET" && req.Method != "HEAD" {
		return ErrUnsupportedMethod
	}

	path, cachebuster := stripCachebuster(req.URL.Path) // "/img/x.png", "foobar"

	info := m.Info(path[1:])
	if info == nil {
		// May as well not bother with anything the asset manager can't find.  The
		// asset manager ensures that the final path is within the static root.
		return ErrNotFound
	}

	fpath := info.FullPath()
	f, err := vfs.Open(fpath)
	if err != nil {
		return err
	}

	defer f.Close()
	rw.Header().Set("Vary", "Accept-Encoding")
	if cachebuster != "" {
		// At some point we should probably check this to prevent cache poisoning
		// but it will break long-expiry for resources referenced from CSS files
		// since they will be in the same cachebuster 'directory' and thus have the
		// wrong tag. Alternatively, could move to unpredictable tags (ones not
		// based on modification tags.)
		//   && cachebuster == info.Tag() {

		rw.Header().Set("Expires", time.Now().Add(28*24*time.Hour).UTC().Format(time.RFC1123))
		rw.Header().Set("Cache-Control", "public, max-age=2419200")
	}

	// Currently, disable CSP for assets under the assumption that they are safe
	// and trusted. This is unfortunately necessary because some served SVG files
	// have inline SVG, and externalizing it would be quite overkill.
	rw.Header().Del("Content-Security-Policy")
	rw.Header().Del("Content-Security-Policy-Report-Only")

	http.ServeContent(rw, req, fpath, info.ModTime(), f)
	return nil
}
