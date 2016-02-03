// Package weberror provides facilities for showing generic error pages based
// on HTTP status codes.
package weberror

import (
	"fmt"
	"github.com/hlandau/degoutils/web/miscctx"
	"github.com/hlandau/degoutils/web/tpl"
	"net/http"
)

func Handler(errorCode int) http.Handler {
	tplName := fmt.Sprintf("error/%d", errorCode)

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		rw.WriteHeader(errorCode)
		tpl.MustShow(req, tplName, nil)
	})
}

func ShowRW(rw http.ResponseWriter, req *http.Request, errorCode int) {
	Handler(errorCode).ServeHTTP(rw, req)
}

func Show(req *http.Request, errorCode int) {
	ShowRW(miscctx.GetResponseWriter(req), req, errorCode)
}
