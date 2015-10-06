package spki

import "hash"
import "crypto/sha256"
import "crypto/sha512"
import "github.com/dchest/blake2b"
import "github.com/hlandau/sx"
import "fmt"

// Represents a SPKI hash type.
//
// Hashes are represented as (hash HASH-TYPE |the hash|), e.g.  (hash blake2b
// |...|).
type HashType string

const (
	// Vanilla Blake2b, 512-bit output.
	Blake2b HashType = "blake2b"
	// SHA-512, 512-bit output.
	SHA512 = "sha512"
	// SHA-256, 256-bit output.
	SHA256 = "sha256"
)

// The currently preferred hash algorithm. May change at a later date.
var PreferredHashType = Blake2b

func (ht HashType) String() string {
	return string(ht)
}

func (ht HashType) New() hash.Hash {
	f, ok := validHashTypes[string(ht)]
	if !ok {
		return nil
	}
	return f()
}

// Returns a SPKI form representing a hash of the given HashType and a digest
// of b. Assumes that b is the output of the HashType.
func (ht HashType) Form(b []byte) []interface{} {
	return []interface{}{
		"hash",
		ht.String(),
		string(b),
	}
}

var ErrMalformedHash = fmt.Errorf("not a well-formed hash S-expression structure")

func LoadHash(x []interface{}) (HashType, []byte, error) {
	r := sx.Q1bsyt(x, "hash")
	if r == nil || len(r) < 2 {
		return "", nil, ErrMalformedHash
	}

	hts, ok := r[0].(string)
	if !ok {
		return "", nil, ErrMalformedHash
	}

	b, ok := r[1].(string)
	if !ok {
		return "", nil, ErrMalformedHash
	}

	ht, ok := ParseHashType(hts)
	if !ok {
		return "", nil, ErrMalformedHash
	}

	return ht, []byte(b), nil
}

var validHashTypes = map[string]func() hash.Hash{
	"blake2b": blake2b.New512,
	"sha256":  sha256.New,
	"sha512":  sha512.New,
}

func ParseHashType(hashType string) (HashType, bool) {
	_, ok := validHashTypes[hashType]
	if !ok {
		return "", false
	}

	return HashType(hashType), true
}
