package tplctx

import "net/http"
import "github.com/hlandau/degoutils/web/opts"
import "github.com/hlandau/degoutils/web/auth"
import "github.com/hlandau/degoutils/web/session"

type Ctx struct {
	Req *http.Request
}

func (c *Ctx) DevMode() bool {
	return opts.DevMode
}

func (c *Ctx) GenAC(s string) string {
	return auth.GenAC(c.Req, s)
}

func (c *Ctx) GetFlash() []session.Flash {
	return session.GetFlash(c.Req)
}

func (c *Ctx) IsAdmin() bool {
	return session.Bool(c.Req, "user_is_admin", false)
}

func (c *Ctx) LoggedIn() bool {
	return session.Int(c.Req, "user_id", 0) != 0
}
