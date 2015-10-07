package connect

//import "net"
import "github.com/hlandau/degoutils/net/curvecp"
import "github.com/hlandau/degoutils/net/bsda"
import "crypto/rand"
import "fmt"
import "golang.org/x/crypto/nacl/box"
import "bytes"
import "io"

func wrapCurveCP(c io.Closer, info *MethodInfo) (io.Closer, error) {
	cc, ok := c.(io.ReadWriteCloser)
	fcc, ok2 := c.(bsda.FrameReadWriterCloser)
	if !ok && !ok2 {
		return nil, fmt.Errorf("curvecp requires ReadWriteCloser (or FrameReadWriteCloser)")
	}

	cfg, ok := info.Pragma["curvecp"].(*curvecp.Config)
	if !ok {
		cfg = &curvecp.Config{}
	}

	if keyIsZero(&cfg.CurveS) {
		if info.URL.User == nil {
			return nil, fmt.Errorf("no server key specified for curvecp")
		}

		username := info.URL.User.Username()
		skey, err := curvecp.DecodeKey(username)
		if err != nil {
			return nil, err
		}

		copy(cfg.CurveS[:], skey[:])
	}

	if keyIsZero(&cfg.Curvek) {
		pk, sk, err := box.GenerateKey(rand.Reader)
		if err != nil {
			return nil, err
		}

		cfg.Curvek = *sk
		cfg.CurveK = pk
	}

	var bc bsda.FrameReadWriterCloser
	if ok2 {
		bc = fcc
	} else {
		bc = bsda.New(cc)
	}

	c2, err := curvecp.New(bc, *cfg)
	if err != nil {
		return nil, err
	}

	return c2, nil
}

func init() {
	RegisterMethod("curvecp", true, wrapCurveCP)
}

var zeroKey [32]byte

func keyIsZero(k *[32]byte) bool {
	return bytes.Equal(k[:], zeroKey[:])
}
