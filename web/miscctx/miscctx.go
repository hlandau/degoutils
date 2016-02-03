// Package miscctx provides miscellaneous context items which can be associated
// with an HTTP request.
package miscctx

import (
	"github.com/gorilla/context"
	"net/http"
)

var responseWriterKey int

// Get the response writer which has been set as corresponding to the given
// request.
func GetResponseWriter(req *http.Request) http.ResponseWriter {
	return context.Get(req, &responseWriterKey).(http.ResponseWriter)
}

// Set the response writer corresponding to the given request.
func SetResponseWriter(rw http.ResponseWriter, req *http.Request) {
	context.Set(req, &responseWriterKey, rw)
}

var canOutputTimeKey int

func SetCanOutputTime(req *http.Request) {
	context.Set(req, &canOutputTimeKey, true)
}

func GetCanOutputTime(req *http.Request) bool {
	_, ok := context.GetOk(req, &canOutputTimeKey)
	return ok
}
