package session

import "net/http"

// Get a value under the given key for the session for the given request.
//
// The second parameter indicates whether the key exists.
func Get(req *http.Request, k string) (interface{}, bool) {
	c := getContext(req)
	return c.Get(k)
}

// Set a value under the given key for the session for the given request.
func Set(req *http.Request, k string, v interface{}) error {
	c := getContext(req)
	return c.Set(k, v)
}

// Save the session for the given request. No-op if the session has
// not been modified since load or since last calling this. Since this
// is called by InitHandler, you shouldn't need to call this yourself.
func Save(req *http.Request) {
	c := getContext(req)
	c.Save()
}

// Delete the given key for the session for the given request, if it exists.
func Delete(req *http.Request, k string) {
	c := getContext(req)
	c.Delete(k)
}

// Bump the epoch of the session, changing the cookie that the user agent must
// present and sending the new cookie to the user agent in the response.
func Bump(req *http.Request) {
	c := getContext(req)
	c.Bump()
}
