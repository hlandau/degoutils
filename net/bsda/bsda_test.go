package bsda_test

import "github.com/hlandau/degoutils/net/bsda"
import "testing"
import "bytes"

type test struct {
	Wire string
	Body []string
}

var bufs = []test{
	{Wire: "\x00\x00\x00\x00", Body: []string{""}},
	{Wire: "\x01\x00\x00\x00\x00", Body: []string{"\x00"}},
	{Wire: "\x02\x00\x00\x00\x00\x01", Body: []string{"\x00\x01"}},
	{Wire: "\x03\x00\x00\x00\x00\x01\x02", Body: []string{"\x00\x01\x02"}},
	{Wire: "\x04\x00\x00\x00\x00\x01\x02\x03", Body: []string{"\x00\x01\x02\x03"}},
	{Wire: "\x05\x00\x00\x00hello", Body: []string{"hello"}},
	{Wire: "\x05\x00\x00\x00hello\x03\x00\x00\x00xyz", Body: []string{"hello", "xyz"}},
}

func TestBSDA(t *testing.T) {
	for _, b := range bufs {
		s := bsda.NewReader(bytes.NewReader([]byte(b.Wire)))
		for _, body := range b.Body {
			fr, err := s.ReadFrame()
			if err != nil {
				t.Fatalf("failed to read frame")
			}

			if body != string(fr) {
				t.Fatalf("body doesn't match")
			}
		}

		buf := bytes.Buffer{}
		wr := bsda.NewWriter(&buf)
		for _, body := range b.Body {
			err := wr.WriteFrame([]byte(body))
			if err != nil {
				t.Fatalf("failed to write frame")
			}
		}

		if buf.String() != b.Wire {
			t.Fatalf("wire mismatch")
		}

		r := bsda.NewReader(bytes.NewReader(buf.Bytes()))
		for _, body := range b.Body {
			fr, err := r.ReadFrame()
			if err != nil {
				t.Fatalf("failed to read frame")
			}

			if string(fr) != body {
				t.Fatalf("body doesn't match")
			}
		}
	}
}
