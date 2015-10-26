package miscctx

import "github.com/gorilla/context"
import "net/http"

var responseWriterKey int

func GetResponseWriter(req *http.Request) http.ResponseWriter {
	return context.Get(req, &responseWriterKey).(http.ResponseWriter)
}

func SetResponseWriter(rw http.ResponseWriter, req *http.Request) {
	context.Set(req, &responseWriterKey, rw)
}
