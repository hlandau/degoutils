package auth

import "hash"
import "crypto/hmac"
import "crypto/sha256"
import "crypto/subtle"
import "crypto/rand"
import "encoding/base64"
import "github.com/hlandau/degoutils/web/session"
import "github.com/hlandau/degoutils/web/tpl"
import "net/http"

func GenACFor(target string, ak []byte) string {
	var h hash.Hash
	h = hmac.New(sha256.New, ak)
	h.Write([]byte(target))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func GenAC(req *http.Request, target string) string {
	// The key is for authenticated users a 32-byte random value stored for their
	// account. For anonymous users it is a 32-byte secret value, the first 16
	// bytes of which are XORed with the v6-mapped IP address. (currently not used)
	ak := session.Bytes(req, "user_ak", nil)
	if len(ak) == 0 {
		ak = make([]byte, 32)
		_, err := rand.Read(ak)
		if err != nil {
			panic(err)
		}

		session.Set(req, "user_ak", ak)
	}

	return GenACFor(target, ak)
}

func VerifyACFor(target, ac string, ak []byte) bool {
	correctAC := GenACFor(target, ak)
	return subtle.ConstantTimeCompare([]byte(correctAC), []byte(ac)) == 1 && correctAC != ""
}

func VerifyACStr(req *http.Request, target, ac string) bool {
	correctAC := GenAC(req, target)
	return subtle.ConstantTimeCompare([]byte(correctAC), []byte(ac)) == 1 && correctAC != ""
}

func VerifyAC(req *http.Request, ac string) bool {
	return VerifyACStr(req, req.URL.Path, ac)
}

func ProtectAC(f func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return ProtectACn("ac", f)
}

func ProtectACn(fieldName string, f func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ac := req.FormValue(fieldName)
		if !VerifyAC(req, ac) {
			rw.WriteHeader(400)
			tpl.Show(req, "error/400", nil)
			return
		}

		f(rw, req)
	})
}
