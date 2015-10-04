package curvecp

import "github.com/hlandau/degoutils/net/bsda"
import "net"
import "testing"
import "crypto/rand"
import "golang.org/x/crypto/nacl/box"
import "io"
import "bytes"

func getLoopbackPair() (net.Conn, net.Conn, error) {
	c1, c2 := net.Pipe()
	return c1, c2, nil
	/*
	     // The following creates a TCP-based pipe to allow for pcap-based debugging
	   	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 4442})
	   	if err != nil {
	   		return nil, nil, err
	   	}

	   	defer listener.Close()
	   	lport := listener.Addr().(*net.TCPAddr).Port
	   	var c1 *net.TCPConn
	   	syncChan := make(chan struct{})
	   	var err2 error
	   	go func() {
	   		c1, err2 = net.DialTCP("tcp", nil, &net.TCPAddr{net.ParseIP("127.0.0.1"), lport, ""})
	   		close(syncChan)
	   	}()

	   	c2, err := listener.AcceptTCP()
	   	if err != nil {
	   		return nil, nil, err
	   	}

	   	<-syncChan
	   	if err2 != nil {
	   		c2.Close()
	   		return nil, nil, err
	   	}

	   	return c2, c1, nil
	*/
}

func fillrand(b []byte) {
	_, err := io.ReadFull(rand.Reader, b)
	if err != nil {
		panic(err)
	}
}

func TestA(t *testing.T) {
	conn1, conn2, err := getLoopbackPair()
	if err != nil {
		panic(err)
	}

	bsda1, bsda2 := bsda.New(conn1), bsda.New(conn2)

	curveS, curves, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic("cannot generate key")
	}

	_, curvec, err := box.GenerateKey(rand.Reader)
	if err != nil {
		panic("cannot generate key")
	}

	errChan := make(chan error, 1)
	go func() {
		// server
		c1, err := New(bsda1, Config{
			IsServer: true,
			Curvek:   *curves,
		})
		errChan <- err
		if err != nil {
			return
		}
		for {
			f, err := c1.ReadFrame()
			if err != nil {
				return
			}
			err = c1.WriteFrame(f)
			if err != nil {
				return
			}
		}
	}()

	// client
	c1, err := New(bsda2, Config{
		Curvek: *curvec,
		CurveS: *curveS,
	})
	serr := <-errChan
	if err != nil {
		t.Logf("client instantiation failed: %v", err)
	}
	if serr != nil {
		t.Logf("server instantiation failed: %v", serr)
	}
	if err != nil || serr != nil {
		t.Fatalf("instantiation failed")
	}

	for i := 0; i < 32; i++ {
		randbuf := make([]byte, i)
		fillrand(randbuf)

		err = c1.WriteFrame(randbuf)
		if err != nil {
			t.Fatalf("write frame error: %v", err)
		}

		f, err := c1.ReadFrame()
		if err != nil {
			t.Fatalf("read frame error: %v", err)
		}

		if !bytes.Equal(randbuf, f) {
			t.Fatalf("buffers are not equal")
		}
	}
}

func TestNoncer(t *testing.T) {
	var n noncer
	fillrand(n.initial[:])

	reuse := map[string]struct{}{}

	for i := 0; i < 300; i++ {
		var nonce [24]byte
		n.Next(&nonce)
		s := string(nonce[:])
		_, ok := reuse[s]
		if ok {
			t.Fatalf("nonce reuse")
		}

		reuse[s] = struct{}{}
	}
}
