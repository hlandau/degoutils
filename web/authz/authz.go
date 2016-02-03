// Package authz provides login requirement wrappers for HTTP handlers.
//
// The following session variables are used:
//
//   "user_id": int.
//   If not set or zero, not logged in.
//
//   "must_change_password": bool.
//   If set to true, redirect all requests with a path other than
//   ChangePasswordPath to ChangePasswordPath.
//
//   "user_is_admin": bool.
//   Used for MustAdmin. A user is considered to be an admin
//   if this is set to true.
//
package authz

import (
	webac "github.com/hlandau/degoutils/web/ac"
	"github.com/hlandau/degoutils/web/miscctx"
	"github.com/hlandau/degoutils/web/opts"
	"github.com/hlandau/degoutils/web/session"
	"github.com/hlandau/degoutils/web/weberror"
	"net/http"
	"net/url"
)

var (
	// The only URL path which can be loaded if "must_change_password" is set.
	// Requests for all other paths redirect to this.
	ChangePasswordPath = "/auth/chpw"

	// Relative or absolute URL to redirect to if login is required.
	LoginURL = "/auth/login"

	// Relative or absolute URL to redirect to after login if there is no
	// specific page to return to.
	AfterLoginURL = "/"
)

// Redirect to a given URL with the given status code, such that the user agent
// can eventually be redirected back to the current URL, unless a return URL
// has already been provided in the current request, in which case that return
// URL is used.
func RedirectWithReturn(req *http.Request, statusCode int, targetURL string) {
	ak := opts.VariantSecretKey("redirect")
	ustr := req.URL.String()

	r, rac := req.FormValue("r"), req.FormValue("rac")
	if r == "" || !webac.VerifyFor("redirect/"+r, rac, ak) {
		r = ustr
		rac = webac.NewFor("redirect/"+r, ak)
	}

	tgt, err := req.URL.Parse(targetURL)
	if err == nil {
		q := tgt.Query()
		q.Set("r", r)
		q.Set("rac", rac)
		tgt.RawQuery = q.Encode()
		targetURL = tgt.String()
	}

	miscctx.RedirectTo(req, statusCode, targetURL)
}

// Redirect back to a return URL, if specified; otherwise, redirect to
// defaultURL.
func ReturnRedirect(req *http.Request, statusCode int, defaultURL string) {
	ak := opts.VariantSecretKey("redirect")
	r := req.FormValue("r")

	if r != "" && webac.VerifyFor("redirect/"+r, req.FormValue("rac"), ak) {
		if _, err := url.Parse(r); err == nil {
			defaultURL = r
		}
	}

	miscctx.RedirectTo(req, statusCode, defaultURL)
}

// Redirects to LoginURL unless session value "user_id" is a nonzero integer.
//
// If "must_change_password" is set to true, any request for a path other than
// ChangePasswordPath is redirected to that path.
func MustLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		userID := session.Int(req, "user_id", 0)
		if userID == 0 {
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "You must log in to access this resource.",
			})
			RedirectWithReturn(req, 302, LoginURL)
			return
		}

		mustChangePassword := session.Bool(req, "must_change_password", false)
		if mustChangePassword && req.URL.Path != ChangePasswordPath {
			session.AddFlash(req, session.Flash{
				Severity: "success",
				Msg:      "You must change your password before proceeding.",
			})
			RedirectWithReturn(req, 302, ChangePasswordPath)
			return
		}

		h.ServeHTTP(rw, req)
	})
}

// Like MustLogin, but takes a function as the wrapped handler.
func MustLoginFunc(h func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return MustLogin(http.HandlerFunc(h))
}

// Ensures that a user is not logged in. Session value "user_id" must be absent
// or zero. If a user is logged in, redirects to AfterLoginURL.
func MustNotLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		userID := session.Int(req, "user_id", 0)
		if userID != 0 {
			ReturnRedirect(req, 302, AfterLoginURL)
			return
		}

		h.ServeHTTP(rw, req)
	})
}

// Like MustNotLogin, but takes a function as the wrapped handler.
func MustNotLoginFunc(h func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return MustNotLogin(http.HandlerFunc(h))
}

// Ensures that "user_is_admin" is set to true.
func MustAdmin(h, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		isAdmin := session.Bool(req, "user_is_admin", false)
		if !isAdmin {
			weberror.ShowRW(rw, req, 404)
			return
		}

		MustLogin(h).ServeHTTP(rw, req)
	})
}
