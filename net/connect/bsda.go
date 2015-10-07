package connect

import "github.com/hlandau/degoutils/net/bsda"
import "fmt"
import "io"

func wrapBSDA(c io.Closer, info *MethodInfo) (io.Closer, error) {
	if fcc, ok := c.(bsda.FrameReadWriterCloser); ok {
		return fcc, nil
	}

	if cc, ok := c.(io.ReadWriteCloser); ok {
		return bsda.New(cc), nil
	}

	return nil, fmt.Errorf("curvecp requires ReadWriteCloser (or FrameReadWriteCloser)")
}

func init() {
	RegisterMethod("bsda", true, wrapBSDA)
}
