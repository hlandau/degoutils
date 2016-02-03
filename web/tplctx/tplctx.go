// Package tplctx provides basic amenities to be provided to templates as a
// context object. It may be composed to provide further methods.
package tplctx

import (
	"encoding/base64"
	"github.com/flosch/pongo2"
	webac "github.com/hlandau/degoutils/web/ac"
	"github.com/hlandau/degoutils/web/assetmgr"
	"github.com/hlandau/degoutils/web/forms"
	"github.com/hlandau/degoutils/web/miscctx"
	"github.com/hlandau/degoutils/web/opts"
	"github.com/hlandau/degoutils/web/session"
	"net/http"
)

// Context functions for use in templates.
type Ctx struct {
	Req *http.Request
}

// Is application running in dev mode?
func (c *Ctx) IsDevMode() bool {
	return opts.DevMode
}

// Generate an action code for the given action. If action is "", the current
// path is used.
func (c *Ctx) AC(action string) string {
	if action == "" {
		action = c.Req.URL.Path
	}

	return webac.New(c.Req, action)
}

// Get flash messages. The flash messages are erased.
func (c *Ctx) Flash() []session.Flash {
	return session.GetFlash(c.Req)
}

// Returns true iff user is an administrator.
func (c *Ctx) IsAdmin() bool {
	return session.Bool(c.Req, "user_is_admin", false)
}

// Returns true iff user is logged in.
func (c *Ctx) IsLoggedIn() bool {
	return session.Int(c.Req, "user_id", 0) != 0
}

func (c *Ctx) CSPNonce() string {
	return miscctx.GetCSPNonce(c.Req)
}

func (c *Ctx) AssetURL(path string) string {
	p := "assets/" + path
	tag := assetmgr.Default.Tag(p)
	if tag != "" {
		return "/.c=" + tag + "/" + p
	} else {
		return "/" + p
	}
}

func (c *Ctx) AssetIntegrity(path string) string {
	p := "assets/" + path
	h := assetmgr.Default.SHA256(p)
	if h == nil {
		return ""
	}

	return "sha256-" + base64.StdEncoding.EncodeToString(h)
}

func (c *Ctx) Fields(f interface{}) *pongo2.Value {
	return forms.FormHTML(f, c.Req, forms.GridHTML)
}
