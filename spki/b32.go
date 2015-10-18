package spki

import "encoding/base32"
import "strings"

func EncodeB32(b []byte) string {
	return strings.ToLower(strings.TrimRight(base32.StdEncoding.EncodeToString(b), "="))
}

func repad(s string) string {
	return s + "======"[len(s)%8-2:]
}

func DecodeB32(s string) ([]byte, error) {
	b, err := base32.StdEncoding.DecodeString(repad(strings.ToUpper(strings.TrimSpace(s))))
	if err != nil {
		return nil, err
	}

	return b, nil
}
