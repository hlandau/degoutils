package mocknet

import (
	"fmt"
	denet "github.com/hlandau/degoutils/net"
	"net"
	"sync"
	"sync/atomic"
)

// A mock internet.
type Internet struct {
	mutex     sync.Mutex
	listeners map[string]*udpConn
	cfg       InternetConfig
}

type InternetConfig struct {
	RxPacketLimit int
}

func NewInternet(cfg *InternetConfig) *Internet {
	if cfg == nil {
		cfg = &InternetConfig{}
	}

	inet := &Internet{
		listeners: map[string]*udpConn{},
		cfg:       *cfg,
	}

	if inet.cfg.RxPacketLimit == 0 {
		inet.cfg.RxPacketLimit = 50
	}

	return inet
}

func (inet *Internet) ListenUDP(net string, laddr *net.UDPAddr) (denet.UDPConn, error) {
	inet.mutex.Lock()
	defer inet.mutex.Unlock()

	_, ok := inet.listeners[laddr.String()]
	if ok {
		return nil, fmt.Errorf("listener already exists for %v", laddr)
	}

	c := &udpConn{
		inet:   inet,
		addr:   *laddr,
		rxChan: make(chan udpPacket, inet.cfg.RxPacketLimit),
	}

	inet.listeners[laddr.String()] = c
	return c, nil
}

// UDP mock connection.
type udpConn struct {
	rxMutex sync.Mutex
	inet    *Internet
	addr    net.UDPAddr
	closed  uint32
	rxChan  chan udpPacket
}

type udpPacket struct {
	Data                []byte
	Source, Destination net.UDPAddr
}

func (c *udpConn) Close() error {
	c.inet.mutex.Lock()
	defer c.inet.mutex.Unlock()
	atomic.StoreUint32(&c.closed, 1)
	delete(c.inet.listeners, c.addr.String())
	close(c.rxChan)
	return nil
}

func (c *udpConn) WriteToUDP(b []byte, a *net.UDPAddr) (int, error) {
	if atomic.LoadUint32(&c.closed) != 0 {
		return 0, fmt.Errorf("connection is closed")
	}

	c.inet.mutex.Lock()
	defer c.inet.mutex.Unlock()
	dst := c.inet.listeners[a.String()]
	if dst == nil {
		return len(b), nil // drop packet
	}

	b2 := make([]byte, len(b))
	copy(b2, b)

	select {
	case dst.rxChan <- udpPacket{
		Data:        b2,
		Source:      c.addr,
		Destination: *a,
	}:
	default:
		// rx buffer full, drop packet
	}

	return len(b), nil
}

func (c *udpConn) ReadFromUDP(b []byte) (int, *net.UDPAddr, error) {
	pkt, ok := <-c.rxChan
	if !ok {
		return 0, nil, fmt.Errorf("connection is closed")
	}

	copy(b, pkt.Data)
	n := len(pkt.Data)
	if n > len(b) {
		n = len(b)
	}

	return n, &pkt.Source, nil
}
