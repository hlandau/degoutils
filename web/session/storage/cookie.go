package storage

import "crypto/hmac"
import "crypto/sha256"
import "crypto/subtle"
import "encoding/binary"
import "encoding/base64"
import "fmt"

// Represents a session reference to be encoded into a cookie.
// Such a cookie is a 2-tuple (ID, Epoch) which is HMAC-signed.
//
// The purpose of the epoch is to change the session cookie without changing
// the session ID used for backend storage purposes. This may be done, for
// example, on login or logout.  The epoch is stored in the session storage
// backend, so a session cookie can be considered invalid if the epoch does not
// match. The epoch is a monotonously increasing counter, and the sole
// operation which should be performed on it is to increment it.
type Cookie struct {
	ID    ID     // Session ID as assigned by storage backend.
	Epoch uint32 // Session Epoch, starts at zero.
}

// Increment the epoch.
func (sc *Cookie) Bump() {
	sc.Epoch++
}

// Encodes the cookie into a form suitable for use as a cookie value (i.e.
// text). The cookie is MAC'd against tampering. The secret key to use for the
// HMAC signature must be passed.
func (sc *Cookie) Encode(secretKey []byte) string {
	// The cookie format is:
	//   [session ID][epoch as 4-byte unsigned little endian integer][HMAC]

	buf := make([]byte, len(sc.ID)+4, len(sc.ID)+4+32)
	copy(buf[0:len(sc.ID)], sc.ID)
	binary.LittleEndian.PutUint32(buf[len(sc.ID):len(sc.ID)+4], sc.Epoch)

	h := hmac.New(sha256.New, secretKey)
	h.Write(buf)
	return base64.StdEncoding.EncodeToString(h.Sum(buf))
}

var errBadCookie = fmt.Errorf("bad cookie")

// Decodes a session cookie value. The HMAC signature is verified using the
// secret key given, and the cookie is returned.
func DecodeCookie(s string, secretKey []byte) (Cookie, error) {
	buf, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return Cookie{}, err
	}

	if len(buf) < 32 {
		return Cookie{}, errBadCookie
	}

	h := hmac.New(sha256.New, secretKey)
	h.Write(buf[0 : len(buf)-32])
	correctMAC := h.Sum(nil)
	if subtle.ConstantTimeCompare(buf[len(buf)-32:], correctMAC) != 1 {
		return Cookie{}, errBadCookie
	}

	c := Cookie{}
	c.ID = ID(string(buf[0 : len(buf)-32-4]))
	c.Epoch = binary.LittleEndian.Uint32(buf[len(buf)-32-4 : len(buf)-32])

	return c, nil
}
