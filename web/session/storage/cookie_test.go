package storage

import "testing"
import "crypto/rand"
import "encoding/hex"

func TestSessionCookie(t *testing.T) {
	b := make([]byte, 16)
	k := make([]byte, 32)
	rand.Read(b)
	rand.Read(k)

	sc := Cookie{
		ID: ID(b),
	}
	cstr := sc.Encode(k)
	sc2, err := DecodeCookie(cstr, k)
	if err != nil {
		t.Error("failed to decode cookie: ", err)
		return
	}

	if sc2.ID != ID(b) {
		t.Errorf("wrong ID after decode, original: %+v, decoded: %+v, encoded: %s", hex.EncodeToString(b),
			hex.EncodeToString([]byte(sc2.ID)), cstr)
		return
	}

	if sc2.Epoch != sc.Epoch {
		t.Error("wrong epoch after decode")
		return
	}
}
