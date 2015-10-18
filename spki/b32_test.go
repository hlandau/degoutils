package spki_test

import "github.com/hlandau/degoutils/spki"
import "testing"

func TestB32(t *testing.T) {
	const c = "rtum6hh5iddpaytvcqq4akx6dem4p66tb4dcrp4avssymxmkkvka"

	b, err := spki.DecodeB32(c)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	s := spki.EncodeB32(b)
	if s != c {
		t.Fatalf("mismatch")
	}
}
