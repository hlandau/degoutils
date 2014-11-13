// Low-level SSDP package which provides a channel streaming SSDP events.
//
// Use package ssdpreg instead of this package.
package ssdp

import gnet "net"
import "time"
import "github.com/hlandau/degoutils/net"
import "github.com/hlandau/degoutils/log"
import "net/http"
import "bytes"
import "net/url"
import "bufio"

const ssdpBroadcastInterval = 60 // seconds

func SSDPBroadcastInterval() int {
	return ssdpBroadcastInterval
}

type SSDPEvent struct {
	Location *url.URL
	ST       string
	USN      string
}

type SSDPClient interface {
	Chan() chan SSDPEvent
	WaitForEvent() SSDPEvent
	Stop()
}

type empty struct{}

type ssdpClient struct {
	stopChan  chan empty
	eventChan chan SSDPEvent
	conn      *gnet.UDPConn
}

func (self *ssdpClient) Stop() {
	for {
		select {
		case self.stopChan <- empty{}:

		default:
			return
		}
	}
}

func (self *ssdpClient) Chan() chan SSDPEvent {
	return self.eventChan
}

func (self *ssdpClient) WaitForEvent() SSDPEvent {
	return <-self.eventChan
}

func (self *ssdpClient) broadcastLoop() {
	defer self.conn.Close()

	ssdpAddr, err := gnet.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return
	}

	ticker := time.NewTicker(time.Duration(ssdpBroadcastInterval) * time.Second)
	defer ticker.Stop()

	discoBuf := bytes.NewBufferString(
		"M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			"ST: ssdp:all\r\n" +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 2\r\n\r\n").Bytes()

	for {
		log.Info("SSDP: Broadcasting discovery packet.")

		_, err = self.conn.WriteToUDP(discoBuf, ssdpAddr)
		select {
		case <-ticker.C:

		case <-self.stopChan:
			return
		}
	}
}

func (self *ssdpClient) handleResponse(res *http.Response) {
	if res.StatusCode != 200 {
		return
	}

	// LOCATION: ...
	// ST: searchTarget
	// USN: ...

	st := res.Header.Get("ST")
	if st == "" {
		return
	}

	loc, err := res.Location()
	if err != nil {
		return
	}

	usn := res.Header.Get("USN")
	if usn == "" {
		usn = loc.String()
	}

	ev := SSDPEvent{
		Location: loc,
		ST:       st,
		USN:      usn,
	}

	select {
	// events not being waited for are simply dropped
	case self.eventChan <- ev:
	default:
	}
}

func (self *ssdpClient) recvLoop() {
	for {
		buf, _, err := net.ReadDatagramFromUDP(self.conn)
		if err != nil {
			return
		}

		r := bytes.NewReader(buf)
		rbio := bufio.NewReader(r)
		res, err := http.ReadResponse(rbio, nil)
		if err == nil {
			self.handleResponse(res)
		}

		select {
		case <-self.stopChan:
			return
		default:
		}
	}
}

func NewClient() (SSDPClient, error) {
	conng, err := gnet.ListenPacket("udp4", ":0")
	if err != nil {
		return nil, err
	}

	conn := conng.(*gnet.UDPConn)

	c := ssdpClient{
		stopChan:  make(chan empty, 2),
		eventChan: make(chan SSDPEvent, 10),
		conn:      conn,
	}

	go c.broadcastLoop()
	go c.recvLoop()

	return &c, nil
}
