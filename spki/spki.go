// Package spki provides marshalling functions for Ed25519 keys.
//
// Quick guide to Ed25519: Public keys are 32 bytes and should be treated as opaque
// 32-byte values. Private keys, by convention, are 64 bytes and consist of the
// 32 byte private key seed value followed by the 32 bytes of the public key.
//
// This means that you can obtain the public key from the private key (it's
// just privateKey[32:64]), you can reconstruct the full private key value from
// its first 32 bytes, and you can obtain the public key from the private key.
//
// Ed25519 private keys can be converted to Curve25519 private keys and
// Ed25519 public keys can be converted to Curve25519 public keys without
// knowledge of the corresponding private key.
//
// Note that the 'random seed value' constituting the Ed25519 private key
// actual is actually passed through SHA-512 before any elliptic curve
// operations (such as signing something or deriving the public key)
// are performed on it.
//
// This is a key difference from Curve25519. In fact, the converted Curve25519
// private key is obtained simply by performing the SHA-512 step (and some
// trivial bit masking). So Curve25519 conventionally works with post-hash
// private keys as inputs, whereas Ed25519 conventionally works with pre-hash
// private keys as inputs. This difference is to enable a hash
// collision-resistance property of Ed25519 signatures.
//
// Anyway, this makes the Ed25519 key marshalling code provided by this package
// also suitable for storing keys used for Curve25519 applications.
package spki

import "fmt"
import "github.com/hlandau/sx"
import "github.com/agl/ed25519/edwards25519"
import "golang.org/x/crypto/openpgp/armor"
import "crypto/sha512"
import "io"
import "io/ioutil"
import "bytes"

// Rederive an Ed25519 public key from a private key.
func Ed25519RederivePublic(privateKey *[64]byte) (publicKey *[32]byte) {
	h := sha512.New()
	h.Write(privateKey[:32])
	digest := h.Sum(nil)
	digest[0] &= 248
	digest[31] &= 127
	digest[31] |= 64

	var A edwards25519.ExtendedGroupElement
	var hBytes [32]byte
	copy(hBytes[:], digest)
	edwards25519.GeScalarMultBase(&A, &hBytes)
	publicKey = new([32]byte)
	A.ToBytes(publicKey)
	return
}

// Form an S-expression structure representing an Ed25519 private key.
func FormEd25519PrivateKey(privateKey *[64]byte) []interface{} {
	return []interface{}{
		[]interface{}{
			"private-key",
			[]interface{}{
				"ed25519",
				privateKey[:],
			},
		},
	}
}

// Form an S-expression structure representing an Ed25519 public key.
func FormEd25519PublicKey(publicKey *[32]byte) []interface{} {
	return []interface{}{
		[]interface{}{
			"public-key",
			[]interface{}{
				"ed25519",
				publicKey[:],
			},
		},
	}
}

// Form an S-expression structure representing an Ed25519 public key
// given the corresponding private key.
func FormEd25519PublicKeyFromPrivateKey(privateKey *[64]byte) []interface{} {
	var x [32]byte
	copy(x[:], privateKey[32:])
	return FormEd25519PublicKey(&x)
}

var ErrMalformedKey = fmt.Errorf("not a well-formed key S-expression structure")

// If isPrivate is false, the public key is at privateKey[32:].
//
// Otherwise the private key is at privateKey[0:32] and the public key is of course
// as always at privateKey[32:64].
func LoadEd25519Key(v []interface{}, privateKey *[64]byte) (isPrivate bool, err error) {
	r := sx.Q1bsyt(v, "private-key ed25519")
	if r == nil {
		r = sx.Q1bsyt(v, "public-key ed25519")
	} else {
		isPrivate = true
	}

	if r == nil {
		return false, ErrMalformedKey
	}

	b, ok := r[0].(string)
	if !ok || (isPrivate && len(b) != 64) || (!isPrivate && len(b) != 32) {
		return false, ErrMalformedKey
	}

	if isPrivate {
		copy(privateKey[:], b)
	} else {
		copy(privateKey[32:], b)
	}

	return
}

// Returns a reader which reads either the contents of an OpenPGP-armored file,
// or, if OpenPGP armor is not found, the file itself. If checkFunc is
// specified and OpenPGP armor is found, it is called with the block. Any
// errors returned from checkFunc short circuit and are returned.
func Dearmor(r io.Reader, checkFunc func(blk *armor.Block) error) (io.Reader, error) {
	abody, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, err
	}

	body := abody
	blk, err := armor.Decode(bytes.NewReader(abody))
	if err == nil {
		if checkFunc != nil {
			err = checkFunc(blk)
			if err != nil {
				return nil, err
			}
		}

		body, err = ioutil.ReadAll(blk.Body)
		if err != nil {
			return nil, err
		}
	}

	return bytes.NewReader(body), nil
}

// Load a keyfile. The file can contain either an OpenPGP-armored file or an
// unarmored S-expression.
func LoadKeyFile(r io.Reader, privateKey *[64]byte) (isPrivate bool, err error) {
	rr, err := Dearmor(r, func(blk *armor.Block) error {
		if blk.Type != "SPKI PRIVATE KEY" && blk.Type != "SPKI PUBLIC KEY" {
			return fmt.Errorf("wrong armor tag")
		}
		return nil
	})
	if err != nil {
		return false, err
	}

	body, err := ioutil.ReadAll(rr)
	if err != nil {
		return false, err
	}

	v, err := sx.SX.Parse(body)
	if err != nil {
		return false, err
	}

	return LoadEd25519Key(v, privateKey)
}
