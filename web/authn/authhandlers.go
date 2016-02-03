// Package authn provides authentication functions.
package authn

import (
	"bytes"
	"crypto/rand"
	"github.com/gorilla/mux"
	"github.com/hlandau/captcha"
	"github.com/hlandau/degoutils/dbutil"
	"github.com/hlandau/degoutils/sendemail"
	webac "github.com/hlandau/degoutils/web/ac"
	"github.com/hlandau/degoutils/web/authz"
	"github.com/hlandau/degoutils/web/miscctx"
	"github.com/hlandau/degoutils/web/opts"
	"github.com/hlandau/degoutils/web/session"
	"github.com/hlandau/degoutils/web/tpl"
	"github.com/hlandau/xlog"
	"github.com/jackc/pgx"
	"gopkg.in/alexcesaro/quotedprintable.v3"
	"gopkg.in/hlandau/easyconfig.v1/cflag"
	"gopkg.in/hlandau/passlib.v1"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/mail"
	"net/textproto"
	"net/url"
	"regexp"
	"strings"
	"time"
)

var authGroup = cflag.NewGroup(nil, "auth")
var registerCAPTCHAFlag = cflag.Bool(authGroup, "registercaptcha", false, "Require CAPTCHA on register?")

var log, Log = xlog.New("web.auth")

type Backend interface {
	GetDatabase() *pgx.ConnPool
	GetCAPTCHA() *captcha.Config
}

type GetBackendFunc func(req *http.Request) Backend

var GetBackend GetBackendFunc

func Auth_Login_GET(rw http.ResponseWriter, req *http.Request) {
	tpl.MustShow(req, "auth/login", nil)
}

func Auth_Login_POST(rw http.ResponseWriter, req *http.Request) {
	email := req.PostFormValue("email")
	password := req.PostFormValue("password")
	userID, ak, isAdmin := ValidateUserEmailPassword(req, email, password)
	if userID == 0 {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Invalid e. mail address or password.",
		})
		Auth_Login_GET(rw, req)
		return
	}

	session.Set(req, "user_id", int(userID))
	session.Set(req, "user_ak", ak)
	session.Set(req, "user_is_admin", isAdmin)

	if req.PostFormValue("remember") != "" {
		session.Set(req, "session_lifetime", 90*24*time.Hour)
	}

	session.Bump(req)

	session.AddFlash(req, session.Flash{
		Severity: "success",
		Msg:      "You have been logged in.",
	})

	authz.ReturnRedirect(req, 302, authz.AfterLoginURL)
}

func Auth_Register_GET(rw http.ResponseWriter, req *http.Request) {
	args := map[string]interface{}{}

	if registerCAPTCHAFlag.Value() {
		ct, ok := session.Get(req, "captchaTime")
		if !ok || !solvedRecently(ct.(time.Time)) {
			inst := GetBackend(req).GetCAPTCHA().NewInstance()
			captchaKey := GetBackend(req).GetCAPTCHA().Key(&inst)
			args["captchaKey"] = captchaKey
		}
	}

	tpl.MustShow(req, "auth/register", args)
}

var re_validUsername = regexp.MustCompilePOSIX(`^[a-zA-Z][a-zA-Z0-9_-]{0,31}$`)
var re_stripShortname = regexp.MustCompilePOSIX(`[^a-zA-Z0-9]`)

func shortname(name string) string {
	name = strings.ToLower(name)
	name = re_stripShortname.ReplaceAllString(name, "")
	return name
}

func Auth_Register_POST(rw http.ResponseWriter, req *http.Request) {
	username := strings.TrimSpace(req.PostFormValue("username"))
	email := req.PostFormValue("email")

	password := req.PostFormValue("password")
	passwordConfirm := req.PostFormValue("password_confirm")

	if registerCAPTCHAFlag.Value() {
		ct, ok := session.Get(req, "captchaTime")
		if !ok || !solvedRecently(ct.(time.Time)) {
			captchaValue := req.PostFormValue("captcha")
			captchaKey := req.PostFormValue("captchak")
			captchaInstance, err := GetBackend(req).GetCAPTCHA().DecodeInstance(captchaKey)
			if err != nil || !GetBackend(req).GetCAPTCHA().Verify(captchaInstance, captchaValue) {
				session.AddFlash(req, session.Flash{
					Severity: "error",
					Msg:      "Invalid CAPTCHA.",
				})

				Auth_Register_GET(rw, req)
				return
			}

			session.Set(req, "captchaTime", time.Now())
		}
	}

	username = strings.Trim(username, " \t\r\n")
	if username == "" {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "You must specify a username.",
		})

		Auth_Register_GET(rw, req)
		return
	}

	if !re_validUsername.MatchString(username) {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Usernames must consist of letters, numbers, underscores and dashes, must begin with a letter and must not exceed 32 characters.",
		})

		Auth_Register_GET(rw, req)
		return
	}

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "You must specify a valid e. mail address.",
		})

		Auth_Register_GET(rw, req)
		return
	}

	if len(password) < 8 {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Password must be at least eight characters long.",
		})

		Auth_Register_GET(rw, req)
		return
	}

	if password != passwordConfirm {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Passwords do not match.",
		})

		Auth_Register_GET(rw, req)
		return
	}

	tx, err := GetBackend(req).GetDatabase().Begin()
	log.Panice(err)

	defer tx.Rollback()

	ak := make([]byte, 32)
	rand.Read(ak)

	pwhash, err := passlib.Hash(password)
	log.Panice(err)

	sn := shortname(username)

	var userID int64
	err = dbutil.InsertKVR(tx, "node", "node_id",
		"shortname", sn,
		"longname", username,
		"type", "user",
	).Scan(&userID)
	if err != nil {
		log.Debuge(err, "can't insert user node")
		if dbutil.IsUniqueViolation(err) {
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "Username already in use.",
			})
		} else {
			log.Panice(err)
		}

		Auth_Register_GET(rw, req)
		return
	}
	_, err = dbutil.InsertKV(tx, "n_user",
		"node_id", userID,
		"email", addr.Address,
		"password_plain", pwhash,
		"ak", ak,
	)
	if err != nil {
		log.Debuge(err, "can't insert user")
		if dbutil.IsUniqueViolation(err) {
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "E. mail address already in use.",
			})
		} else {
			log.Panice(err)
		}

		Auth_Register_GET(rw, req)
		return
	}

	err = tx.Commit()
	if err != nil {
		log.Errore(err, "commit registration transaction")
		Auth_Register_GET(rw, req)
		return
	}

	err = sendVerificationEmail(addr.Address, ak, false)
	if err != nil {
		log.Errore(err, "cannot send verification e. mail")
		Auth_Register_GET(rw, req)
		return
	}

	session.Set(req, "user_id", int(userID))
	session.Set(req, "user_ak", ak)
	session.Set(req, "user_is_admin", false)
	session.AddFlash(req, session.Flash{
		Severity: "success",
		Msg:      "You have successfully been signed up.",
	})

	miscctx.SeeOther(req, "/")
	return
}

func Auth_Verify_GET(rw http.ResponseWriter, req *http.Request) {
	ac := req.FormValue("ac")
	reset_s := req.FormValue("r")
	email := req.FormValue("e")

	var userID int64
	var ak []byte
	var verified bool
	var isAdmin bool
	err := GetBackend(req).GetDatabase().QueryRow("SELECT node_id, ak, is_admin, email_verified FROM \"n_user\" WHERE email=$1 LIMIT 1", email).Scan(&userID, &ak, &isAdmin, &verified)
	log.Panice(err, "find ak for e. mail verify")

	if !webac.VerifyFor("verify-email/"+reset_s+"/"+email, ac, ak) {
		rw.WriteHeader(400)
		tpl.MustShow(req, "front/400", nil)
		return
	}

	if !verified {
		_, err = dbutil.UpdateKV(GetBackend(req).GetDatabase(), "n_user", dbutil.Set{"email_verified": true}, dbutil.Where{"node_id": userID})
		log.Panice(err)
	}

	if reset_s == "1" {
		_, err = rand.Read(ak)
		log.Panice(err)

		_, err = dbutil.UpdateKV(GetBackend(req).GetDatabase(), "n_user", dbutil.Set{"ak": ak}, dbutil.Where{"node_id": userID})
	}

	// log user in
	if reset_s == "0" {
		if verified {
			// non-reset links cannot be used to get a free login to an already-verified account
			rw.WriteHeader(400)
			tpl.MustShow(req, "front/400", nil)
			return
		}

		session.AddFlash(req, session.Flash{
			Severity: "success",
			Msg:      "Your e. mail address has been verified.",
		})
	} else {
		session.Set(req, "must_change_password", true)
	}
	session.Set(req, "user_id", int(userID))
	session.Set(req, "user_ak", ak)
	session.Set(req, "user_is_admin", isAdmin)
	session.Bump(req)

	miscctx.SeeOther(req, "/panel")
}

func Auth_LostPW_GET(rw http.ResponseWriter, req *http.Request) {
	tpl.MustShow(req, "auth/lostpw", nil)
}

func Auth_LostPW_POST(rw http.ResponseWriter, req *http.Request) {
	email := req.PostFormValue("email")
	var userID int64
	var ak []byte
	err := GetBackend(req).GetDatabase().QueryRow("SELECT id, ak FROM \"user\" WHERE email=$1 LIMIT 1", email).Scan(&userID, &ak)
	if err != nil {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "No user with that e. mail address was found.",
		})
		Auth_LostPW_GET(rw, req)
		return
	}

	err = sendVerificationEmail(email, ak, true)
	if err != nil {
		log.Errore(err, "cannot send verification e. mail")
		Auth_LostPW_GET(rw, req)
		return
	}

	session.AddFlash(req, session.Flash{
		Severity: "success",
		Msg:      "A password recovery e. mail has been sent; please follow the instructions therein.",
	})

	Auth_LostPW_GET(rw, req)
}

func Auth_Logout_POST(rw http.ResponseWriter, req *http.Request) {
	session.Delete(req, "user_id")
	session.Delete(req, "user_ak")
	session.Delete(req, "user_is_admin")
	session.Delete(req, "signup_flow")
	session.AddFlash(req, session.Flash{
		Severity: "success",
		Msg:      "You have been logged out.",
	})
	authz.ReturnRedirect(req, 302, "/")
}

func Auth_ChangePassword_GET(rw http.ResponseWriter, req *http.Request) {
	tpl.MustShow(req, "auth/chpw", map[string]interface{}{
		"must_change_password": session.Bool(req, "must_change_password", false),
	})
}

func Auth_ChangePassword_POST(rw http.ResponseWriter, req *http.Request) {
	userID := session.Int(req, "user_id", 0)
	curPassword := req.PostFormValue("cur_password")
	password := req.PostFormValue("password")
	passwordConfirm := req.PostFormValue("password_confirm")

	if password == passwordConfirm {
		if len(password) >= 8 {
			mustChangePassword := session.Bool(req, "must_change_password", false)

			var err error
			var passwordPlain string
			if !mustChangePassword {
				err = GetBackend(req).GetDatabase().QueryRow("SELECT password_plain FROM \"n_user\" WHERE node_id=$1", userID).Scan(&passwordPlain)
				log.Panice(err)

				_, err = passlib.Verify(curPassword, passwordPlain)
			}

			if err == nil {
				newHash, err := passlib.Hash(password)
				log.Panice(err)

				newAK := make([]byte, 32)
				rand.Read(newAK)

				_, err = GetBackend(req).GetDatabase().Exec("UPDATE \"n_user\" SET password_plain=$1, ak=$2 WHERE node_id=$3", newHash, newAK, userID)
				log.Panice(err)

				session.Set(req, "user_ak", newAK)

				if mustChangePassword {
					session.Set(req, "must_change_password", false)
				}

				session.AddFlash(req, session.Flash{
					Severity: "success",
					Msg:      "Password changed.",
				})
				miscctx.SeeOther(req, "/")
				return
			} else {
				session.AddFlash(req, session.Flash{
					Severity: "error",
					Msg:      "Password incorrect.",
				})
			}
		} else {
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "Password must be at least 8 characters long.",
			})
		}
	} else {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Passwords do not match.",
		})
	}

	Auth_ChangePassword_GET(rw, req)
}

func Auth_ChangeEmail_GET(rw http.ResponseWriter, req *http.Request) {
	tpl.MustShow(req, "auth/chemail", nil)
}

func Auth_ChangeEmail_POST(rw http.ResponseWriter, req *http.Request) {
	userID := session.Int(req, "user_id", 0)
	curPassword := req.PostFormValue("cur_password")
	email := req.PostFormValue("email")

	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Name != "" {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Invalid e. mail address.",
		})
		Auth_ChangeEmail_GET(rw, req)
		return
	}

	var passwordPlain string
	var oldEmail string

	tx, err := GetBackend(req).GetDatabase().Begin()
	log.Panice(err)
	defer tx.Rollback()

	err = tx.QueryRow("SELECT password_plain, email FROM \"n_user\" WHERE node_id=$1 LIMIT 1", userID).
		Scan(&passwordPlain, &oldEmail)
	log.Panice(err)

	_, err = passlib.Verify(curPassword, passwordPlain)
	if err != nil {
		session.AddFlash(req, session.Flash{
			Severity: "error",
			Msg:      "Passwords do not match.",
		})
		Auth_ChangeEmail_GET(rw, req)
		return
	}

	//_, err = tx.Exec("INSERT INTO security_log (type,user_id,message) VALUES ($1,$2,$3)", "change_email", userID, fmt.Sprintf("%s -> %s", oldEmail, addr.Address))
	//log.Panice(err)

	_, err = tx.Exec("UPDATE \"n_user\" SET email=$1, email_verified='f' WHERE node_id=$2", addr.Address, userID)
	if err != nil {
		if perr, ok := err.(pgx.PgError); ok && perr.Code == "23505" { // unique constraint violation
			session.AddFlash(req, session.Flash{
				Severity: "error",
				Msg:      "That e. mail address is already in use.",
			})
			Auth_ChangeEmail_GET(rw, req)
			return
		} else {
			log.Panice(err)
		}
	}

	ak, _ := session.Get(req, "user_ak")
	err = sendVerificationEmail(addr.Address, ak.([]byte), false)
	log.Panice(err)

	err = tx.Commit()
	log.Panice(err)

	session.AddFlash(req, session.Flash{
		Severity: "success",
		Msg:      "E. mail address changed.",
	})

	miscctx.SeeOther(req, "/")
}

func appendPart(w *multipart.Writer, headers func(h textproto.MIMEHeader), body string) {
	if body == "" {
		return
	}

	h := textproto.MIMEHeader{}
	h.Set("Content-Transfer-Encoding", "quoted-printable")
	headers(h)
	partW, err := w.CreatePart(h)
	log.Panice(err, "create MIME part")
	quoW := quotedprintable.NewWriter(partW)
	defer quoW.Close()
	io.WriteString(quoW, body)
}

func sendPlainEmail(email, subject, plainBody string) error {
	sendemail.SendAsync(&sendemail.Email{
		To: []string{email},
		Headers: map[string][]string{
			"Subject": []string{subject},
		},
		Body: plainBody,
	})

	return nil
}

func sendHTMLEmail(email, subject, plainBody, htmlBody string) error {
	if htmlBody == "" {
		return sendPlainEmail(email, subject, plainBody)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	appendPart(w, func(h textproto.MIMEHeader) {
		h.Set("Content-Type", "text/plain; charset=utf-8")
	}, plainBody)

	appendPart(w, func(h textproto.MIMEHeader) {
		h.Set("Content-Type", "text/html; charset=utf-8")
	}, htmlBody)

	sendemail.SendAsync(&sendemail.Email{
		To: []string{email},
		Headers: map[string][]string{
			"Subject":      []string{subject},
			"MIME-Version": []string{"1.0"},
			"Content-Type": []string{"multipart/alternative; boundary=" + w.Boundary()},
		},
		Body: string(buf.Bytes()),
	})

	return nil
}

func sendVerificationEmail(email string, ak []byte, reset bool) error {
	rstr := "0"
	if reset {
		rstr = "1"
	}
	verifyAC := webac.NewFor("verify-email/"+rstr+"/"+email, ak)
	subject := "Violations DB: verify your e. mail address"

	url := opts.BaseURL + "/auth/verify?" + url.Values{
		"e":  []string{email},
		"ac": []string{verifyAC},
		"r":  []string{rstr},
	}.Encode()

	escapedURL := html.EscapeString(url)

	body := `Greetings.

You, or someone else, has created a Violations DB account with this e. mail address.

If you requested this, please verify your e. mail address by following the following link:

  <` + url + `#>

If you did not request this message, please ignore it.
`
	htmlBody := `<p>Greetings.</p>

<p>You, or someone else, has created a Violations DB account with this e. mail address.</p>

<p>If you requested this, please <a href="` + escapedURL + `">click here to verify your e. mail address</a>.</p>

<p>If you did not request this message, please ignore it.</p>
`

	if reset {
		subject = "Violations DB: password recovery request"
		body = `Greetings.

You, or someone else, has requested password recovery for an account registered
to this e. mail address.

To reset the password for this account, please visit the following URL:

  <` + url + `#>

If you did not request this message, please ignore it.
`

		htmlBody = `<p>Greetings.</p>

<p>You, or someone else, has requested password recovery for an account registered to this e. mail address.</p>

<p><a href="` + escapedURL + `">Please click here to reset the password for this account.</a></p>

<p>If you did not request this message, please ignore it.</p>
`
	}

	return sendHTMLEmail(email, subject, body, htmlBody)
}

func ValidateUserEmailPassword(req *http.Request, email, password string) (int64, []byte, bool) {
	var userID int64
	var passwordPlain string
	var ak []byte
	var isAdmin bool
	err := GetBackend(req).GetDatabase().QueryRow("SELECT node_id, password_plain, ak, is_admin FROM \"n_user\" WHERE email=$1", email).
		Scan(&userID, &passwordPlain, &ak, &isAdmin)
	if err != nil {
		return 0, nil, false
	}

	newHash, err := passlib.Verify(password, passwordPlain)
	if err != nil {
		return 0, nil, false
	}

	if newHash != "" {
		GetBackend(req).GetDatabase().Exec("UPDATE \"n_user\" SET password_plain=$1 WHERE id=$2", newHash, userID)
		// ignore errors
	}

	if len(ak) == 0 {
		ak = make([]byte, 32)
		rand.Read(ak)
		GetBackend(req).GetDatabase().Exec("UPDATE \"n_user\" SET ak=$1 WHERE id=$2", ak, userID)
		// ignore errors
	}

	return userID, ak, isAdmin
}

func solvedRecently(t time.Time) bool {
	return !t.Before(time.Now().Add(-10 * time.Minute))
}

func Register(router *mux.Router) {
	router.Handle("/auth/chpw", authz.MustLoginFunc(Auth_ChangePassword_GET)).Methods("GET")
	router.Handle("/auth/chpw", authz.MustLogin(webac.Protect(Auth_ChangePassword_POST))).Methods("POST")
	router.Handle("/auth/chemail", authz.MustLoginFunc(Auth_ChangeEmail_GET)).Methods("GET")
	router.Handle("/auth/chemail", authz.MustLogin(webac.Protect(Auth_ChangeEmail_POST))).Methods("POST")
	router.Handle("/auth/login", authz.MustNotLoginFunc(Auth_Login_GET)).Methods("GET")
	router.Handle("/auth/login", authz.MustNotLoginFunc(Auth_Login_POST)).Methods("POST")
	router.Handle("/auth/register", authz.MustNotLoginFunc(Auth_Register_GET)).Methods("GET")
	router.Handle("/auth/register", authz.MustNotLoginFunc(Auth_Register_POST)).Methods("POST")
	router.Handle("/auth/lostpw", authz.MustNotLoginFunc(Auth_LostPW_GET)).Methods("GET")
	router.Handle("/auth/lostpw", authz.MustNotLoginFunc(Auth_LostPW_POST)).Methods("POST")
	router.Handle("/auth/logout", webac.Protect(Auth_Logout_POST)).Methods("POST")
	router.HandleFunc("/auth/verify", Auth_Verify_GET).Methods("GET")
}
