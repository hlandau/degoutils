package session

import "fmt"
import "net/http"

//import "github.com/hlandau/degoutils/web"
import "github.com/hlandau/degoutils/web/session/storage"
import "github.com/hlandau/degoutils/log"
import "github.com/gorilla/context"
import "github.com/hlandau/degoutils/web/origin"

type Config struct {
	CookieName string
	SecretKey  []byte
	Store      storage.Store
}

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

var contextKey int

func (cfg *Config) InitHandler(h http.Handler) http.Handler {
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

func getContext(req *http.Request) *ctx {
	v := context.Get(req, &contextKey)
	if v == nil {
		panic("session context not initialized for this request")
	}

	return v.(*ctx)
}

var errBadSession = fmt.Errorf("bad session")

func (c *ctx) loadSession() error {
	if c.loaded {
		return nil
	}

	cookie, err := c.req.Cookie("fang_s")
	if err != nil {
		return errBadSession
	}

	sc, err := storage.DecodeCookie(cookie.Value, c.cfg.SecretKey)
	if err != nil {
		return errBadSession
	}

	c.id = sc.ID

	c.data, err = c.cfg.Store.Get(c.id)
	if err != nil {
		return errBadSession
	}

	epoch, ok := c.data["epoch"]
	if ok {
		epoch_i, ok := epoch.(uint32)
		if !ok {
			log.Debug("no epoch in session as uint32")
			return errBadSession
		}

		if epoch_i != sc.Epoch {
			log.Debug("wrong epoch")
			return errBadSession
		}
	}

	c.loaded = true
	return nil
}

func (c *ctx) writeSessionCookie(sc storage.Cookie) {
	cookie := http.Cookie{
		Name:     "fang_s",
		Value:    sc.Encode(c.cfg.SecretKey),
		Path:     "/",
		Secure:   origin.IsSSL(c.req),
		HttpOnly: true,
	}

	c.rw.Header().Add("Set-Cookie", cookie.String())
}

func (c *ctx) newSession() {
	sessionID, err := c.cfg.Store.Create()
	log.Panice(err, "cannot create session")

	c.id = sessionID

	sc := storage.Cookie{ID: sessionID}
	c.writeSessionCookie(sc)
	c.data = map[string]interface{}{
		"epoch": uint32(0),
	}
	c.loaded = true
	c.isNewSession = true
	c.dirty = true
}

func (c *ctx) BumpSession() {
	if c.isNewSession {
		return
	}

	epoch_i := c.data["epoch"].(uint32)
	epoch_i++
	c.data["epoch"] = epoch_i

	sc := storage.Cookie{ID: c.id, Epoch: epoch_i}
	c.writeSessionCookie(sc)
}

func (c *ctx) GetSession(k string) (interface{}, bool) {
	err := c.loadSession()
	if err != nil {
		return nil, false
	}

	v, ok := c.data[k]
	return v, ok
}

func (c *ctx) DeleteSession(k string) {
	err := c.loadSession()
	if err != nil {
		return
	}

	c.dirty = true
	delete(c.data, k)
}

func (c *ctx) SetSession(k string, v interface{}) error {
	err := c.loadSession()
	if err != nil {
		//log.Debug("creating new session because set: ", k)
		c.newSession()
	}

	c.dirty = true
	c.data[k] = v
	return nil
}

func (c *ctx) Save() {
	if !c.dirty {
		return
	}

	c.cfg.Store.Set(c.id, c.data)
	c.dirty = false
}
