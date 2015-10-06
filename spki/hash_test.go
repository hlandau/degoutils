package spki_test

import "github.com/hlandau/degoutils/spki"
import "github.com/hlandau/sx"
import "testing"
import "bytes"

func TestHashing(t *testing.T) {
	h := spki.Blake2b.New()

	h.Write([]byte{0})
	b := h.Sum(nil)

	form := spki.Blake2b.Form(b)
	outs, err := sx.SXCanonical.String(form)
	if err != nil {
		panic(err)
	}

	const corr = "4:hash7:blake2b64:\x2F\xA3\xF6\x86\xDF\x87\x69\x95\x16\x7E\x7C\x2E\x5D\x74\xC4\xC7\xB6\xE4\x8F\x80\x68\xFE\x0E\x44\x20\x83\x44\xD4\x80\xF7\x90\x4C\x36\x96\x3E\x44\x11\x5F\xE3\xEB\x2A\x3A\xC8\x69\x4C\x28\xBC\xB4\xF5\xA0\xF3\x27\x6F\x2E\x79\x48\x7D\x82\x19\x05\x7A\x50\x6E\x4B"

	if outs != corr {
		t.Fatalf("does not match:\n%#v\n%#v", outs, corr)
	}

	ht, b2, err := spki.LoadHash([]interface{}{form})
	if err != nil {
		panic(err)
	}

	if ht != spki.Blake2b || !bytes.Equal(b, b2) {
		panic("mismatch on load")
	}
}
