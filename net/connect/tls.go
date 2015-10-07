package connect

import "io"
import "fmt"
import "net"
import "crypto/tls"

func wrapTLS(c io.Closer, info *MethodInfo) (io.Closer, error) {
	conn, ok := c.(net.Conn)
	if !ok {
		return nil, fmt.Errorf("TLS requires net.Conn")
	}

	cfg, ok := info.Pragma["tls"].(*tls.Config)
	if !ok {
		cfg = &tls.Config{}
	}

	if cfg.ServerName == "" {
		cfg.ServerName = info.Hostname
	}

	c2 := tls.Client(conn, cfg)
	err := c2.Handshake()
	if err != nil {
		return nil, err
	}

	return c2, nil
}

func init() {
	RegisterMethod("tls", true, wrapTLS)
}
