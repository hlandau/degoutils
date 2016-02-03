package session

import "net/http"

// Gets a boolean value from the session for the given request under the given
// key. If the value is not present in the session, or it is not a boolean
// value, returns def.
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

// Gets an int value from the session for the given request under the given
// key. If the value is not present in the session, or it is not an int value,
// returns def.
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

// Gets a string value from the session for the given request under the given
// key. If the value is not present in the session, or it is not a string
// value, returns def.
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

// Gets a []byte value from the session for the given request under the given
// key. If the value is not present in the session, or it is not of type []byte,
// returns def.
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
