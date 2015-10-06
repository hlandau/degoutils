package spki_test

import "github.com/hlandau/degoutils/spki"
import "github.com/hlandau/sx"
import "testing"
import "strings"
import "reflect"

func TestFile(t *testing.T) {
	r := strings.NewReader("this is a file")
	fi, err := spki.HashFile(r)
	if err != nil {
		panic(err)
	}

	outs, err := sx.SXCanonical.String([]interface{}{fi.Form()})
	if err != nil {
		panic(err)
	}

	v, err := sx.SX.Parse([]byte(outs))
	if err != nil {
		panic(err)
	}

	fi2, err := spki.LoadFileInfo(v)
	if err != nil {
		panic(err)
	}

	if !reflect.DeepEqual(fi, fi2) {
		t.Fatalf("not equal: %#v,\n\n%#v", fi, fi2)
	}
}
