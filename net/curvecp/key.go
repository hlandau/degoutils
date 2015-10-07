package curvecp

import "encoding/base32"
import "fmt"

// Encodes a public key to a base32 string.
func EncodeKey(pk *[32]byte) string {
	return base32.StdEncoding.EncodeToString(pk[:])
}

func DecodeKey(pk string) (*[32]byte, error) {
	b, err := base32.StdEncoding.DecodeString(pk)
	if err != nil {
		return nil, err
	}

	if len(b) != 32 {
		return nil, fmt.Errorf("decoded key was wrong length")
	}

	b2 := new([32]byte)
	copy(b2[:], b)

	return b2, nil
}
