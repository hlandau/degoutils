package auth

import "net/http"
import deweb "github.com/hlandau/degoutils/web"
import "github.com/hlandau/degoutils/web/session"

func MustLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		userID := session.Int(req, "user_id", 0)
		if userID == 0 {
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "You must log in to access this resource.",
			})
			deweb.RedirectTo(req, 302, "/auth/login")
			return
		}

		mustChangePassword := session.Bool(req, "must_change_password", false)
		if mustChangePassword && req.RequestURI != "/auth/chpw" {
			session.AddFlash(req, session.Flash{
				Severity: "success",
				Msg:      "You must change your password before proceeding.",
			})
			deweb.RedirectTo(req, 302, "/auth/chpw")
			return
		}

		if req.RequestURI == "/auth/chpw" || req.RequestURI == "/auth/logout" {
			h.ServeHTTP(rw, req)
			return
		}

		h.ServeHTTP(rw, req)
	})
}

func MustLoginFunc(h func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return MustLogin(http.HandlerFunc(h))
}

var AfterLoginURL = "/"

func MustNotLogin(h http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		userID := session.Int(req, "user_id", 0)
		if userID != 0 {
			deweb.RedirectTo(req, 302, AfterLoginURL)
			return
		}

		h.ServeHTTP(rw, req)
	})
}

func MustNotLoginFunc(h func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return MustNotLogin(http.HandlerFunc(h))
}

func MustAdmin(h, notFound http.Handler) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		isAdmin := session.Bool(req, "user_is_admin", false)
		if !isAdmin {

		}

		MustLogin(h).ServeHTTP(rw, req)
	})
}
