package main

import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/spki"
import "github.com/hlandau/sx"
import "github.com/agl/ed25519"
import "crypto/rand"
import "gopkg.in/alecthomas/kingpin.v2"
import "os"
import "fmt"
import "golang.org/x/crypto/openpgp/armor"
import "github.com/calmh/randomart"
import "io/ioutil"
import "strings"
import "bytes"
import "github.com/agl/ed25519/extra25519"

var (
	root               = kingpin.New("spkitool", "SPKI tool")
	generateKey        = root.Command("generate-key", "Generate a private key")
	generateKeyEd25519 = generateKey.Command("ed25519", "Generate an Ed25519 private key")

	keyInfo    = root.Command("key-info", "View key info")
	kiFilename = keyInfo.Arg("filename", "Path to the key file").Required().String()
)

func encHex(b byte) rune {
	if b < 10 {
		return rune('0' + b)
	} else {
		return rune('a' + b - 10)
	}
}

func hexFormat(b []byte) string {
	s := ""
	first := true
	for _, c := range b {
		if first {
			first = false
		} else {
			s += ":"
		}
		s += string((encHex((c >> 4) & 0x0F)))
		s += string((encHex(c & 0x0F)))
	}
	return s
}

func prefix(s string) string {
	parts := strings.Split(s, "\n")
	out := ""
	for _, p := range parts {
		out += ";; "
		out += p
		out += "\n"
	}
	return out
}

func doGenerateKeyEd25519() {
	_, sk, err := ed25519.GenerateKey(rand.Reader)
	log.Fatale(err, "cannot generate key")

	s, err := sx.SXCanonical.String(spki.FormEd25519PrivateKey(sk))
	log.Fatale(err, "cannot serialize key")

	fmt.Fprintf(os.Stderr, ";; Fingerprint: %v\n;;\n", hexFormat(sk[32:64]))

	fmt.Fprintf(os.Stderr, "%s", prefix(randomart.Generate(sk[32:64], " Ed25519").String()))

	w, err := armor.Encode(os.Stdout, "SPKI PRIVATE KEY", map[string]string{
		"Version": "Davka",
	})
	log.Fatale(err)

	defer w.Close()
	w.Write([]byte(s))
}

func doKeyInfo() {
	b, err := ioutil.ReadFile(*kiFilename)
	log.Fatale(err, "cannot read keyfile")

	var sk [64]byte
	isPrivate, err := spki.LoadKeyFile(bytes.NewReader(b), &sk)
	log.Fatale(err, "cannot decode keyfile")

	if isPrivate {
		fmt.Fprintf(os.Stderr, ";; Ed25519 Private Key\n")
	} else {
		fmt.Fprintf(os.Stderr, ";; Ed25519 Public Key\n")
	}
	fmt.Fprintf(os.Stderr, ";; Fingerprint: %v\n", hexFormat(sk[32:64]))
	fmt.Fprintf(os.Stderr, ";; Fingerprint (b32): %v\n", spki.EncodeB32(sk[32:64]))
	fmt.Fprintf(os.Stderr, "%s", prefix(randomart.Generate(sk[32:64], " Ed25519").String()))

	var cpk [32]byte
	var edpub [32]byte
	copy(edpub[:], sk[32:64])
	if !extra25519.PublicKeyToCurve25519(&cpk, &edpub) {
		log.Fatal("Cannot derive Curve25519 public key.")
	}

	fmt.Fprintf(os.Stderr, ";; Curve25519 Fingerprint: %v\n", hexFormat(cpk[:]))
	fmt.Fprintf(os.Stderr, ";; Curve25519 Fingerprint (b32): %v\n", spki.EncodeB32(cpk[:]))
}

func main() {
	switch kingpin.MustParse(root.Parse(os.Args[1:])) {
	case generateKeyEd25519.FullCommand():
		doGenerateKeyEd25519()
	case keyInfo.FullCommand():
		doKeyInfo()
	}
}
