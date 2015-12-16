package web

import "net/http"
import "html"
import "fmt"
import "github.com/hlandau/degoutils/web/miscctx"

func RedirectTo(req *http.Request, code int, url string) {
	rw := miscctx.GetResponseWriter(req)

	rw.Header().Set("Location", url)
	if req.Method == "GET" {
		rw.Header().Set("Content-Type", "text/html; charset=utf-8")
	}

	rw.WriteHeader(code)
	if req.Method == "GET" {
		eurl := html.EscapeString(url)
		rw.Write([]byte(fmt.Sprintf(`<!DOCTYPE html><html xmlns="http://www.w3.org/1999/xhtml" lang="en"><head><meta charset="utf-8"/><title>Moved</title></head><body><p>This resource has moved to <a href="%s">%s</a>.</p></body></html>`, eurl, eurl)))
	}
}

func SeeOther(req *http.Request, url string) {
	RedirectTo(req, 303, url)
}
