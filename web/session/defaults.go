package session

import "net/http"

func Bool(req *http.Request, k string, def bool) bool {
	v, ok := Get(req, k)
	if !ok {
		return def
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return def
}

func Int(req *http.Request, k string, def int) int {
	v, ok := Get(req, k)
	if !ok {
		return def
	}
	if i, ok := v.(int); ok {
		return i
	}
	return def
}

func String(req *http.Request, k string, def string) string {
	v, ok := Get(req, k)
	if !ok {
		return def
	}
	if i, ok := v.(string); ok {
		return i
	}
	return def
}

func Bytes(req *http.Request, k string, def []byte) []byte {
	v, ok := Get(req, k)
	if !ok {
		return def
	}
	if i, ok := v.([]byte); ok {
		return i
	}
	return def
}
