package spki_test

import "github.com/hlandau/degoutils/spki"
import "testing"

func TestBabbleprint(t *testing.T) {
	s := spki.Babbleprint([]byte("alphbetagamadelt"))
	if s != "ximek-sesuk-memak-hityk-cynok-cyrak-cenek-hurul-gyxox" {
		t.Fatalf("bad babbleprint")
	}
}
