// Package session provides session storage facilities.
package session

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/context"
	"github.com/hlandau/degoutils/web/origin"
	"github.com/hlandau/degoutils/web/session/storage"
	"gopkg.in/hlandau/easyconfig.v1/cflag"
)

var cookieNameFlag = cflag.String(nil, "sessioncookiename", "s", "Session cookie name")

// Session system configuration.
type Config struct {
	// The session store to use. Required.
	Store storage.Store

	// The name to be used for the session cookie. Defaults to configurable
	// "sessioncookiename", which itself defaults to "s".
	CookieName string

	// Secret key used to generate the HMAC signatures for session cookies.
	//
	// If not specified, a temporary random key will be generated automatically.
	SecretKey []byte
}

func (cfg *Config) setDefaults() {
	if cfg.CookieName == "" {
		cfg.CookieName = cookieNameFlag.Value()
	}
}

// Session information attached to a request. Used to provide for session
// state, including lazy session loading.
type ctx struct {
	cfg          *Config
	req          *http.Request
	rw           http.ResponseWriter
	data         map[string]interface{}
	id           storage.ID
	loadError    error
	loaded       bool
	dirty        bool
	isNewSession bool
}

// &contextKey is used as the key for the context item storing the *ctx value.
var contextKey int

// Returns an HTTP handler which initializes the session information for the
// request and passes processing to the given handler. Also ensures that the
// session is saved, including in the event of a panic. Session saving is best
// effort and errors are not noted.
//
// Sessions are loaded lazily; they are not loaded until information is
// actually requested from the session.
func (cfg *Config) InitHandler(h http.Handler) http.Handler {
	cfg.setDefaults()

	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		c := &ctx{
			req: req,
			rw:  rw,
			cfg: cfg,
		}

		context.Set(req, &contextKey, c)
		defer c.Save()

		h.ServeHTTP(rw, req)
	})
}

// Get context for the request.
func getContext(req *http.Request) *ctx {
	v := context.Get(req, &contextKey)
	if v == nil {
		panic("session context not initialized for this request")
	}

	return v.(*ctx)
}

// Called by Get().
func (c *ctx) Get(k string) (interface{}, bool) {
	// Load if not already loaded.
	err := c.loadSession()
	if err != nil {
		// If we failed to load, no session exists, thus the key cannot exist.
		return nil, false
	}

	v, ok := c.data[k] // Get item from loaded session.
	return v, ok
}

// Called by Delete().
func (c *ctx) Delete(k string) {
	// Load if not already loaded.
	err := c.loadSession()
	if err != nil {
		// If we failed to load, no session exists, thus the key cannot exist, so
		// this is a no-op.
		return
	}

	c.dirty = true
	delete(c.data, k)
}

// Called by Set().
func (c *ctx) Set(k string, v interface{}) error {
	// Load if not already loaded.
	err := c.loadSession()
	if err != nil {
		// If we failed to load, no session exists, so create a new session.
		c.newSession()
	}

	c.dirty = true // Session needs to be written at end of request processing.
	c.data[k] = v  // Set item.
	return nil
}

func (c *ctx) Save() {
	if !c.dirty {
		// Session has not been changed (or possibly not even loaded), so no need
		// to do anything.
		return
	}

	// Set new session map in storage backend.
	c.cfg.Store.Set(c.id, c.data)
	c.dirty = false
}

// Called by Bump().
func (c *ctx) Bump() {
	if c.isNewSession {
		// If the session has just been created, there is no point in bumping it.
		return
	}

	epoch_i := c.data["epoch"].(uint32) // Will default to zero value.
	epoch_i++
	c.data["epoch"] = epoch_i

	// Update cookie.
	sc := storage.Cookie{ID: c.id, Epoch: epoch_i}
	c.writeSessionCookie(sc)
}

var errBadSession = fmt.Errorf("bad session")
var errNoSession = fmt.Errorf("no session")

// Load session.
func (c *ctx) loadSession() error {
	err := c.loadSessionInner()
	if err == errBadSession {
		// If the session cookie is invalid or refers to a session which no longer
		// exists, erase it.
		c.writeSessionCookieRaw("")
	}

	return err
}

func (c *ctx) loadSessionInner() error {
	if c.loaded {
		// Already loaded, don't need to do anything.
		return nil
	}

	cookie, err := c.req.Cookie(c.cfg.CookieName)
	if err != nil {
		// No session cookie is set.
		return errNoSession
	}

	sc, err := storage.DecodeCookie(cookie.Value, c.cfg.SecretKey)
	if err != nil {
		// The cookie is malformed or has a bad MAC.
		return errBadSession
	}

	c.id = sc.ID

	// Get session data from store.
	c.data, err = c.cfg.Store.Get(c.id)
	if err != nil {
		// The session does not exist.
		return errBadSession
	}

	// Check that the epoch is correct.
	epoch, ok := c.data["epoch"]
	if ok {
		epochi, ok := epoch.(uint32)
		if !ok || epochi != sc.Epoch {
			return errBadSession
		}
	}

	// Loading complete.
	c.loaded = true
	return nil
}

// Write updated session cookie.
func (c *ctx) writeSessionCookie(sc storage.Cookie) {
	c.writeSessionCookieRaw(sc.Encode(c.cfg.SecretKey))
}

func (c *ctx) writeSessionCookieRaw(v string) {
	maxAge := 0
	if v == "" {
		maxAge = -1
	}

	ck := http.Cookie{
		Name:     cookieNameFlag.Value(),
		Value:    v,
		Path:     "/",
		MaxAge:   maxAge,
		Secure:   origin.IsSSL(c.req),
		HttpOnly: true,
	}

	replaceCookie(c.rw, &ck)
}

// Replaces a Set-Cookie directive in the given http.ResponseWriter, overriding
// any previously set Set-Cookie header for the same cookie.
func replaceCookie(rw http.ResponseWriter, c *http.Cookie) {
	if c.Name != "" {
		s := rw.Header()["Set-Cookie"]
		for i := 0; i < len(s); i++ {
			idx := strings.IndexByte(s[i], '=')
			if idx < 0 {
				continue
			}
			k := s[i][0:idx]
			if k == c.Name {
				s = append(s[0:i], s[i+1:]...)
			}
		}
		rw.Header()["Set-Cookie"] = s
	}

	http.SetCookie(rw, c)
}

// Create a new session.
func (c *ctx) newSession() {
	sessionID, err := c.cfg.Store.Create()
	if err != nil {
		panic(err) // explode
	}

	c.id = sessionID
	c.loaded = true
	c.isNewSession = true

	// Send cookie.
	sc := storage.Cookie{ID: sessionID}
	c.writeSessionCookie(sc)

	// Initial data.
	c.data = map[string]interface{}{
		"epoch": uint32(0),
	}

	// The session will have been created by Create(), but we need to make sure we save the epoch.
	c.dirty = true
}
