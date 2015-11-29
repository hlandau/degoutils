package zipfs

import "testing"
import "os"
import "io/ioutil"
import "fmt"
import "crypto/sha256"
import "encoding/hex"
import "os/exec"
import "bytes"
import "github.com/hlandau/degoutils/vfs"

func TestZipArc(t *testing.T) {
	pipeR, pipeW, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	cmd := exec.Command("xz", "-dc", "zip1.xz")
	cmd.Stdout = pipeW
	cmd.Stderr = os.Stderr
	err = cmd.Start()
	if err != nil {
		panic(err)
	}
	pipeW.Close()
	b, err := ioutil.ReadAll(pipeR)
	if err != nil {
		panic(err)
	}
	pipeR.Close()
	err = cmd.Wait()
	if err != nil {
		panic(err)
	}

	testZipArc(bytes.NewReader(b))
}

type reader struct {
	*bytes.Reader
}

func (r *reader) Close() error {
	return nil
}

func testZipArc(f *bytes.Reader) {
	za, err := New(&reader{f}, f.Size())
	if err != nil {
		panic(err)
	}

	compareFile(za, "a.txt", "This is a file.\n")
	compareFile(za, "k/l/m/a.txt", "k-l-m-a!\n")
	compareFile(za, "z.txt", "This is also a file.\n")

	d, err := za.Open("a")
	if err != nil {
		panic(err)
	}

	dfi, err := d.Readdir(0)
	if err != nil {
		panic(err)
	}

	h := sha256.New()
	for _, x := range dfi {
		s := fmt.Sprintf("%v\t%v\t%v\t%v\n", x.Name(), x.IsDir(), x.Mode(), x.Size())
		h.Write([]byte(s))
	}
	hs := hex.EncodeToString(h.Sum(nil))
	if hs != "93d5c5b0a7a5205b5ad687a8726dbec1a355a3937151ca96f7e704675ae1e536" {
		panic("hash mismatch: " + hs)
	}
}

func compareFile(za vfs.Filesystem, fn string, v string) {
	f, err := za.Open(fn)
	if err != nil {
		panic(err)
	}

	defer f.Close()
	b, err := ioutil.ReadAll(f)
	if err != nil {
		panic(err)
	}

	if string(b) != v {
		panic("mismatch: " + fn + ": " + string(b))
	}
}
