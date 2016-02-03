package ac

import (
	"crypto/rand"
	"testing"
)

func TestAC(t *testing.T) {
	ak := make([]byte, 32)
	rand.Read(ak)

	ak2 := make([]byte, 32)
	copy(ak2, ak)
	ak2[5] = ak2[5] ^ 1

	ac := NewFor("foo", ak)
	if !VerifyFor("foo", ac, ak) {
		t.Fatal("verify")
	}

	ac2 := NewFor("foo", ak)
	if !VerifyFor("foo", ac2, ak) {
		t.Fatal("verify")
	}

	if VerifyFor("Foo", ac2, ak) {
		t.Fatal("verify")
	}

	if VerifyFor("foo", ac2, ak2) {
		t.Fatal("verify")
	}

	if ac == ac2 {
		t.Fatal("same")
	}

	t.Logf("ac: %v", ac)
	t.Logf("ac2: %v", ac2)
}
