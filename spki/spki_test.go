package spki_test

import "github.com/hlandau/degoutils/spki"
import "github.com/agl/ed25519"
import "testing"
import "crypto/rand"
import "github.com/hlandau/sx"
import "bytes"
import "fmt"

func TestSPKI(t *testing.T) {
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	outs, err := sx.SXCanonical.String(spki.FormEd25519PrivateKey(sk))
	if err != nil {
		panic(err)
	}

	v, err := sx.SX.Parse([]byte(outs))
	if err != nil {
		panic(fmt.Sprintf("failed to parse: %v: %#v", err, outs))
	}

	var sk2 [64]byte
	isPrivate, err := spki.LoadEd25519Key(v, &sk2)
	if err != nil {
		panic(fmt.Sprintf("failed to load: %v: %#v", err, v))
	}

	if !isPrivate {
		panic("should be private")
	}

	if !bytes.Equal(sk[:], sk2[:]) {
		panic("should be equal")
	}

	// public key

	outs, err = sx.SXCanonical.String(spki.FormEd25519PublicKeyFromPrivateKey(sk))
	if err != nil {
		panic(err)
	}

	v, err = sx.SX.Parse([]byte(outs))
	if err != nil {
		panic(err)
	}

	var sk3 [64]byte
	isPrivate, err = spki.LoadEd25519Key(v, &sk3)
	if err != nil {
		panic(err)
	}

	if isPrivate {
		panic("should be public")
	}

	if !bytes.Equal(pk[:], sk3[32:]) {
		panic("should be equal")
	}
}
