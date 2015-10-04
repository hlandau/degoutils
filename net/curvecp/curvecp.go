// Package curvecp implements a CurveCP-esque protocol over stateful reliable
// ordered bidirectional point-to-point datagram streams.
package curvecp

import "io"
import "crypto/rand"
import "github.com/hlandau/degoutils/net/bsda"
import "golang.org/x/crypto/nacl/box"
import "golang.org/x/crypto/curve25519"
import "bytes"
import "fmt"
import "encoding/binary"
import "crypto/subtle"
import "sync"
import "sync/atomic"
import "net"

// Initiation parameters for CurveCP session.
type Config struct {
	IsServer bool

	Curvek [32]byte  // own private key
	CurveK *[32]byte // optional public key (optimization; avoids need for Curve25519 operation)
	CurveS [32]byte  // server's public key (required only if we are a client)

	Rand io.Reader // if nil, crypto/rand is used

	// Used only by Dial. If nil, net.Dial is used.
	DialFunc func(net, addr string) (net.Conn, error)
}

// CurveCP connection.
type Conn struct {
	cfg       Config
	conn      bsda.FrameReadWriterCloser
	closeOnce sync.Once
	closed    int32

	nonceC, nonceS                                                                          noncer
	curveS, curves, curveC, curvec, curveCt, curvect, curveSt, curvest, curveCtS, curveCtSt [32]byte
	//                                           Known by
	//                                               C  S
	// curveC             Client Permanent Public    *  *
	// curvec             Client Permanent Private   *
	// curveCt            Client Transient Public    *  *
	// curvect            Client Transient Private   *
	// curveS             Server Permanent Public    *  *
	// curves             Server Permanent Private      *
	// curveSt            Server Transient Public    *  *
	// curvest            Server Transient Private      *
	// curveCtSt          Symmetric Encryption Key   *  *
	// curveCtS           Hello Symmetric Enc Key    *  *
	// nonceC             Used for boxes from client *  *
	// nonceS             Used for boxes from server *  *
}

// Utility function to dial an address and create a connection on it.
//
// net should almost certainly be "tcp". You must set Curvek and CurveS.
func Dial(netw, addr string, cfg Config) (*Conn, error) {
	df := cfg.DialFunc
	if df == nil {
		df = net.Dial
	}

	conn, err := df(netw, addr)
	if err != nil {
		return nil, err
	}

	return New(bsda.New(conn), cfg)
}

// Initiate a CurveCP connection over a reliable ordered bidirectional
// point-to-point datagram stream.
func New(conn bsda.FrameReadWriterCloser, cfg Config) (*Conn, error) {
	c := &Conn{
		cfg:  cfg,
		conn: conn,
	}

	if c.cfg.Rand == nil {
		// use crypto/rand if no random source specified
		c.cfg.Rand = rand.Reader
	}

	err := c.handshake()
	if err != nil {
		c.conn.Close()
		return nil, err
	}

	return c, nil
}

//
type opcode byte

const (
	opClientHello    opcode = 0x00
	opServerHello           = 0x01
	opClientCommence        = 0x02
	opMessage               = 0x03
)

const clientHelloMagic uint32 = 0xb673b08d
const serverHelloMagic uint32 = 0x42d19719
const version uint16 = 0

// noncer generates 24-byte nonces from a 24-byte initial nonce and a counter
// value which is XORed with the nonce. (It is therefore assumed that the
// 24-byte nonce is generated randomly and would be sufficiently random if only
// 16 bytes. The randomness XORed with the 8-byte counter is simply a bonus,
// as most streams won't count to 0xFFFFFFFFFFFFFFFF messages.)
//
// Panics if the counter rolls over.
type noncer struct {
	initial [24]byte
	counter uint64
}

func xorNonce(v *[24]byte, counter uint64) {
	for i := uint(0); i < 8; i++ {
		v[i] = v[i] ^ byte((counter>>(i*8))&0xFF)
	}
}

// Get the next nonce. The first nonce uses a counter value of zero and thus is
// the same as the initial nonce.
func (n *noncer) Next(v *[24]byte) {
	copy(v[:], n.initial[:])

	xorNonce(v, n.counter)

	n.counter++
	if n.counter == 0 {
		panic("counter rollover")
	}
}

//
var zeroKey [32]byte

func keyIsZero(k *[32]byte) bool {
	return bytes.Equal(k[:], zeroKey[:])
}

//
func (c *Conn) handshake() error {
	if c.cfg.IsServer {
		return c.handshakeAsServer()
	} else {
		return c.handshakeAsClient()
	}
}

// Server Handshaking

func (c *Conn) handshakeAsServer() error {
	// Check that a private key has actually been specified.
	c.curves = c.cfg.Curvek
	if keyIsZero(&c.curves) {
		return fmt.Errorf("Server private key not specified.")
	}

	// Derive server public key from server private key.
	if c.cfg.CurveK != nil {
		c.curveS = *c.cfg.CurveK
	} else {
		curve25519.ScalarBaseMult(&c.curveS, &c.curves)
	}

	err := c.hsReadClientHello()
	if err != nil {
		return err
	}

	err = c.hsWriteServerHello()
	if err != nil {
		return err
	}

	// Determine the shared secret key used for encryption.
	box.Precompute(&c.curveCtSt, &c.curveCt, &c.curvest)

	err = c.hsReadClientCommence()
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) hsReadClientHello() error {
	// Receive client hello message.
	data, err := c.conn.ReadFrame()
	if err != nil {
		return err
	}

	// Ensure client hello message is of adequate size.
	if len(data) < 81 {
		return fmt.Errorf("undersized hello")
	}

	// Ensure that the message is a client hello message.
	if opcode(data[0]) != opClientHello {
		return fmt.Errorf("unexpected non-hello op")
	}

	// Check hello magic.
	magic := binary.LittleEndian.Uint32(data[1:5])
	if magic != clientHelloMagic {
		return fmt.Errorf("hello op did not contain correct magic")
	}

	// Check version.
	if binary.LittleEndian.Uint16(data[5:7]) != version {
		return fmt.Errorf("unexpected protocol version")
	}

	// Store client transient public key and nonce.
	copy(c.curveCt[:], data[9:41])
	copy(c.nonceC.initial[:], data[41:65])

	// Ensure client nonce does not have last bit set.
	c.nonceC.initial[23] &= 0xFE

	// Take a client nonce and use it to open the for-future-use box.
	// N.B. An adversary can replay this box since it uses only a
	// client-specified nonce. Thus it is important not to do anything
	// regarding the contents of this box until the Commence command
	// proves ownership of ct.
	var bnonce [24]byte
	c.nonceC.Next(&bnonce)

	box.Precompute(&c.curveCtS, &c.curveCt, &c.curves)
	_, ok := box.OpenAfterPrecomputation(nil, data[65:], &bnonce, &c.curveCtS)
	if !ok {
		return fmt.Errorf("malformed box in client hello")
	}

	return nil
}

func (c *Conn) hsWriteServerHello() error {
	// Generate a random server nonce.
	_, err := io.ReadFull(c.cfg.Rand, c.nonceS.initial[:])
	if err != nil {
		return err
	}

	// Ensure server nonce has last bit set so that no server nonce will ever
	// equal a client nonce; this is important as messages in both directions
	// will end up using the same key.
	c.nonceS.initial[23] |= 1

	// Generate our transient public and private key.
	St, st, err := box.GenerateKey(c.cfg.Rand)
	if err != nil {
		return err
	}

	c.curveSt = *St
	c.curvest = *st

	// Send server hello
	shbuf := make([]byte, 29, 45)
	shbuf[0] = byte(opServerHello)
	binary.LittleEndian.PutUint32(shbuf[1:5], serverHelloMagic)
	copy(shbuf[5:29], c.nonceS.initial[:])

	var nonce [24]byte
	c.nonceS.Next(&nonce)
	shbuf = box.SealAfterPrecomputation(shbuf, c.curveSt[:], &nonce, &c.curveCtS)

	err = c.conn.WriteFrame(shbuf)
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) hsReadClientCommence() error {
	// Inner Nc is taken first
	var nonce [24]byte
	c.nonceC.Next(&nonce)

	// Client commence frame is encrypted normally, like a message.
	data, opc, err := c.readFrame()
	if err != nil {
		return err
	}

	// Ensure that client commence message is of adequate size.
	if len(data) < 112 {
		return fmt.Errorf("undersized client commence")
	}

	// Ensure that message is a client commence message.
	if opc != opClientCommence {
		return fmt.Errorf("unexpected non-client commence op")
	}

	// Store client permanent public key.
	copy(c.curveC[:], data[0:32])

	// Open vouch box proving possession of client's permanent private key.
	vbuf, ok := box.Open(nil, data[32:], &nonce, &c.curveC, &c.curvest)
	if !ok {
		return fmt.Errorf("malformed vouch box in client commence")
	}

	// Ensure that the box contains the correct Ct value being vouched for.
	if subtle.ConstantTimeCompare(vbuf[0:32], c.curveCt[:]) != 1 {
		return fmt.Errorf("incorrect Ct value in vouch box")
	}

	// Ensure that the box contains the correct S value.
	if subtle.ConstantTimeCompare(vbuf[32:64], c.curveS[:]) != 1 {
		return fmt.Errorf("invalid S value in vouch box")
	}

	return nil
}

// Client Handshaking

func (c *Conn) handshakeAsClient() error {
	// Check that keys have actually been specified.
	c.curvec = c.cfg.Curvek
	c.curveS = c.cfg.CurveS
	if keyIsZero(&c.curvec) {
		return fmt.Errorf("Client private key not specified.")
	}
	if keyIsZero(&c.curveS) {
		return fmt.Errorf("Server public key not specified.")
	}

	// Derive client permanent public key from private key.
	if c.cfg.CurveK != nil {
		c.curveC = *c.cfg.CurveK
	} else {
		curve25519.ScalarBaseMult(&c.curveC, &c.curvec)
	}

	err := c.hcWriteClientHello()
	if err != nil {
		return err
	}

	err = c.hcReadServerHello()
	if err != nil {
		return err
	}

	// Determine the shared secret key used for encryption.
	box.Precompute(&c.curveCtSt, &c.curveSt, &c.curvect)

	err = c.hcWriteClientCommence()
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) hcWriteClientHello() error {
	// Generate a random client nonce.
	_, err := io.ReadFull(c.cfg.Rand, c.nonceC.initial[:])
	if err != nil {
		return err
	}

	// Ensure client nonce does not have last bit set.
	c.nonceC.initial[23] &= 0xFE

	// Generate our transient public and private key.
	Ct, ct, err := box.GenerateKey(c.cfg.Rand)
	if err != nil {
		return err
	}

	c.curveCt = *Ct
	c.curvect = *ct

	// Send client hello
	b := make([]byte, 65, 81)
	b[0] = byte(opClientHello)
	binary.LittleEndian.PutUint32(b[1:5], clientHelloMagic)
	copy(b[9:41], c.curveCt[:])
	copy(b[41:65], c.nonceC.initial[:])

	var nonce [24]byte
	c.nonceC.Next(&nonce)
	box.Precompute(&c.curveCtS, &c.curveS, &c.curvect)
	b = box.SealAfterPrecomputation(b, nil, &nonce, &c.curveCtS)

	return c.conn.WriteFrame(b)
}

func (c *Conn) hcReadServerHello() error {
	// Receive server hello message.
	data, err := c.conn.ReadFrame()
	if err != nil {
		return err
	}

	// Ensure server hello message is of adequate size.
	if len(data) < 77 {
		return fmt.Errorf("undersized server hello")
	}

	if opcode(data[0]) != opServerHello {
		return fmt.Errorf("unexpected non-server helo op")
	}

	if binary.LittleEndian.Uint32(data[1:5]) != serverHelloMagic {
		return fmt.Errorf("wrong server hello magic")
	}

	// Store server nonce.
	copy(c.nonceS.initial[:], data[5:29])

	// Open box to get St.
	var nonce [24]byte
	c.nonceS.Next(&nonce)
	b, ok := box.OpenAfterPrecomputation(nil, data[29:77], &nonce, &c.curveCtS)
	if !ok {
		return fmt.Errorf("malformed box in server hello")
	}

	copy(c.curveSt[:], b[0:32])
	return nil
}

func (c *Conn) hcWriteClientCommence() error {
	// vouch box
	vb := make([]byte, 64)
	copy(vb[0:32], c.curveCt[:])
	copy(vb[32:64], c.curveS[:])

	var nonce [24]byte
	c.nonceC.Next(&nonce)
	vbox := box.Seal(nil, vb, &nonce, &c.curveSt, &c.curvec)

	// outer box
	b2 := make([]byte, 112)
	copy(b2[0:32], c.curveC[:])
	copy(b2[32:112], vbox)

	return c.writeFrame(opClientCommence, b2)
}

// Read a frame.
func (c *Conn) ReadFrame() ([]byte, error) {
	if atomic.LoadInt32(&c.closed) != 0 {
		return nil, ErrClosed
	}

	b, opc, err := c.readFrame()
	if err == nil && opc != opMessage {
		c.failConn()
		return nil, fmt.Errorf("unexpected non-message opcode")
	} else if err != nil {
		c.failConn()
	}

	return b, err
}

func (c *Conn) readFrame() ([]byte, opcode, error) {
	b, err := c.conn.ReadFrame()
	if err != nil {
		return nil, 0, err
	}

	if len(b) < 17 {
		return nil, 0, fmt.Errorf("unexpected non-message opcode")
	}

	var nonce [24]byte
	if c.cfg.IsServer {
		c.nonceC.Next(&nonce)
	} else {
		c.nonceS.Next(&nonce)
	}
	b2, ok := box.OpenAfterPrecomputation(nil, b[1:], &nonce, &c.curveCtSt)
	if !ok {
		return nil, 0, fmt.Errorf("invalid msg box received")
	}

	if len(b2) == 0 || b2[0] != 0 {
		return nil, 0, fmt.Errorf("nonzero type byte in msg box")
	}

	return b2[1:], opcode(b[0]), nil
}

var ErrClosed = fmt.Errorf("connection is closed")

// Write a frame.
func (c *Conn) WriteFrame(b []byte) error {
	if atomic.LoadInt32(&c.closed) != 0 {
		return ErrClosed
	}

	err := c.writeFrame(opMessage, b)
	if err != nil {
		c.failConn()
		return err
	}

	return nil
}

func (c *Conn) writeFrame(op opcode, b []byte) error {
	b2 := make([]byte, len(b)+1)
	b2[0] = 0
	copy(b2[1:], b)
	var nonce [24]byte
	if c.cfg.IsServer {
		c.nonceS.Next(&nonce)
	} else {
		c.nonceC.Next(&nonce)
	}
	out := make([]byte, 1, len(b)+18)
	out[0] = byte(op)
	out = box.SealAfterPrecomputation(out, b2, &nonce, &c.curveCtSt)
	err := c.conn.WriteFrame(out)
	if err != nil {
		return err
	}

	return nil
}

func (c *Conn) failConn() {
	c.Close()
}

// Close connection.
func (c *Conn) Close() error {
	var err error
	c.closeOnce.Do(func() {
		err = c.conn.Close()
		atomic.StoreInt32(&c.closed, 1)
	})
	return err
}

// Returns the peer's permanent public key.
func (c *Conn) PeerPublicKey() [32]byte {
	if c.cfg.IsServer {
		return c.curveC
	} else {
		return c.curveS
	}
}

// Returns the peer's transient public key. Rarely needed.
func (c *Conn) PeerTemporaryPublicKey() [32]byte {
	if c.cfg.IsServer {
		return c.curveCt
	} else {
		return c.curveSt
	}
}
