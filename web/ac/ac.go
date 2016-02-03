// Package ac provides facilities for generating and verifying anti-CSRF
// authentication codes.
package ac

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"

	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"golang.org/x/crypto/salsa20/salsa"
	"sync/atomic"

	"github.com/hlandau/degoutils/web/session"
	"github.com/hlandau/degoutils/web/weberror"
	"net/http"
)

var randKey [32]byte
var counter uint64

func init() {
	rand.Read(randKey[:])
}

// Generate an action code using the given action name, action key and 32-byte
// nondeterminism mask. Place the raw, unencoded 64-byte action code in b.
func genWithMaskRaw(b []byte, action string, ak, mask []byte) {
	h := hmac.New(sha256.New, ak)
	h.Write([]byte(action))
	h.Sum(b[0:0])

	copy(b[32:64], mask[0:32])
	xorBytes(b[0:32], mask[0:32])
}

// Generate an action code using the given action name, action key and 32-byte
// nondeterminism mask. Return the string.
func genWithMask(action string, ak, mask []byte) string {
	var b [64]byte
	genWithMaskRaw(b[:], action, ak, mask)
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// Xor x with y and store the result in x. x and y must be the same length.
func xorBytes(x, y []byte) {
	for i := 0; i < len(x); i++ {
		x[i] = x[i] ^ y[i]
	}
}

// Generate a nondeterministic action code using the given action name and action key.
func NewFor(action string, ak []byte) string {
	var mask [32]byte
	generateMask(mask[:])

	return genWithMask(action, ak, mask[:])
}

// Generate a psuedorandom 32-byte mask and put it in mask. mask must be
// zeroes when this is called.
func generateMask(mask []byte) {
	var nonce [16]byte
	newCounter := atomic.AddUint64(&counter, 1)
	binary.LittleEndian.PutUint64(nonce[0:8], newCounter)

	salsa.XORKeyStream(mask[0:32], mask[0:32], &nonce, &randKey)
}

// Get the action key to use in relation to a given request.
func GetAK(req *http.Request) []byte {
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

	return ak
}

// Generate an action code for the given action in relation to a given request.
func New(req *http.Request, action string) string {
	return NewFor(action, GetAK(req))
}

// Verify an action code for the given action and action key. Returns true iff
// valid.
func VerifyFor(action, ac string, ak []byte) bool {
	givenAC, err := base64.RawURLEncoding.DecodeString(ac)
	if err != nil || len(givenAC) != 64 {
		return false
	}

	var correctAC [64]byte
	genWithMaskRaw(correctAC[:], action, ak, givenAC[32:64])
	return subtle.ConstantTimeCompare(correctAC[:], givenAC) == 1
}

// Verify an action code for the given action in relation to a given request.
// Returns true iff valid.
func VerifyStr(req *http.Request, action, ac string) bool {
	return VerifyFor(action, ac, GetAK(req))
}

// Verify an action code in relation to a given request. The action is the path
// of the request URL. Returns true iff valid.
func Verify(req *http.Request, ac string) bool {
	return VerifyStr(req, req.URL.Path, ac)
}

// http.Handler wrapper that bails if a valid action key for the request URL's path
// is not found in GET/POST variable "ac".
func Protect(f func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return Protectn("ac", f)
}

// http.Handler wrapper that bails if a valid action key for the request URL's
// path is not found in the GET/POST variable whose name is specified in
// fieldName.
func Protectn(fieldName string, f func(rw http.ResponseWriter, req *http.Request)) http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		ac := req.FormValue(fieldName)
		if !IsSafeMethod(req.Method) && !Verify(req, ac) {
			weberror.ShowRW(rw, req, 400)
			return
		}

		f(rw, req)
	})
}

func IsSafeMethod(methodName string) bool {
	switch methodName {
	case "GET", "HEAD":
		return true
	default:
		return false
	}
}
