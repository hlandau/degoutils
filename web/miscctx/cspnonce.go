// Package miscctx provides miscellaneous context items which can be associated
// with an HTTP request.
package miscctx

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"github.com/gorilla/context"
	"golang.org/x/crypto/salsa20/salsa"
	"net/http"
	"sync/atomic"
)

var cspNonceKey int

var cspNonceGenerationKey [32]byte
var cspNonceCounter uint64

func init() {
	rand.Read(cspNonceGenerationKey[:])
}

func GetCSPNonce(req *http.Request) string {
	v, ok := context.Get(req, &cspNonceKey).(string)
	if ok {
		return v
	}

	var b [8]byte
	var nonce [16]byte
	binary.LittleEndian.PutUint64(nonce[0:8], atomic.AddUint64(&cspNonceCounter, 1))
	salsa.XORKeyStream(b[:], b[:], &nonce, &cspNonceGenerationKey)

	v = base64.RawURLEncoding.EncodeToString(b[:])
	context.Set(req, &cspNonceKey, v)
	return v
}
