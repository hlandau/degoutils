package portmap

import gnet "net"
import "errors"
import "fmt"
import "time"
import "github.com/hlandau/degoutils/net"
import "github.com/hlandau/degoutils/log"
import "bytes"
import "encoding/binary"

const opcGetExternalAddr = 0
const natpmpHostToRouterPort = 5351

var natpmpRetryConfig = net.RetryConfig{
	MaxTries:           9,
	InitialDelay:       250,
	MaxDelay:           64000, // InitialDelay*8
	MaxDelayAfterTries: 8,
}

var natpmpErrTimeout = errors.New("Request timed out.")

func natpmpMakeRequest(dst gnet.IP, opcode byte, data []byte) (r []byte, err error) {
	conn, err := gnet.DialUDP("udp", nil, &gnet.UDPAddr{dst, natpmpHostToRouterPort, ""})
	if err != nil {
		return
	}

	defer conn.Close()

	msg := make([]byte, 2)
	msg[0] = 0      // Version 0
	msg[1] = opcode // Opcode
	msg = append(msg, data...)

	rconf := natpmpRetryConfig
	rconf.Reset()

	for {
		// here we use the 'delay' as the timeout
		maxtime := rconf.GetStepDelay()
		if maxtime == 0 {
			// max tries reached
			break
		}

		err = conn.SetDeadline(time.Now().Add(time.Duration(maxtime) * time.Millisecond))
		if err != nil {
			return
		}

		var n int
		n, err = conn.Write(msg)
		if err != nil {
			log.Info(fmt.Sprintf("couldn't write NAT-PMP packet: %+v", err))
			return
		}

		if n != len(msg) {
			err = errors.New("couldn't send whole datagram")
			return
		}

		var res []byte
		var uaddr *gnet.UDPAddr
		res, uaddr, err = net.ReadDatagramFromUDP(conn)
		if err != nil {
			if err.(gnet.Error).Timeout() {
				continue
			}
			//log.Info(fmt.Sprintf("couldn't read NAT-PMP packet: %+v", err))
			return
		}

		if !uaddr.IP.Equal(dst) || uaddr.Port != natpmpHostToRouterPort {
			continue
		}

		if len(res) < 4 {
			continue
		}

		if res[0] != 0 || res[1] != (0x80|opcode) {
			continue
		}

		rc := binary.BigEndian.Uint16(res[2:])

		if rc != 0 {
			err = errors.New(fmt.Sprintf("Default gateway responded to NAT-PMP request with nonzero error code %d", rc))
			return
		}

		r = res[4:]
		return
	}

	err = natpmpErrTimeout
	return
}

func natpmpGetExternalAddr(gwaddr gnet.IP) (extadr gnet.IP, err error) {
	r, err := natpmpMakeRequest(gwaddr, opcGetExternalAddr, []byte{})
	if err != nil {
		return
	}

	//time = r[0:4]
	extadr = r[4:8]
	return
}

const natpmpTCP = 6
const natpmpUDP = 17

const opcMapTCP = 1
const opcMapUDP = 2

func natpmpMap(gwaddr gnet.IP, protoNum int,
	internalPort, suggestedExternalPort uint16, lifetime uint32) (externalPort uint16, actualLifetime uint32, err error) {
	var opc byte
	if protoNum == natpmpTCP {
		opc = opcMapTCP
	} else if protoNum == natpmpUDP {
		opc = opcMapUDP
	} else {
		err = errors.New("unsupported protocol")
		return
	}

	b := bytes.NewBuffer(make([]byte, 0, 10))
	binary.Write(b, binary.BigEndian, uint16(0))
	binary.Write(b, binary.BigEndian, uint16(internalPort))
	binary.Write(b, binary.BigEndian, uint16(suggestedExternalPort))
	binary.Write(b, binary.BigEndian, uint32(lifetime))

	r, err := natpmpMakeRequest(gwaddr, opc, b.Bytes())
	if err != nil {
		return
	}

	if len(r) < 12 {
		err = errors.New("short response")
		return
	}

	// r[0: 4] // time
	// r[4: 6] // internal port
	// r[6: 8] // mapped external port
	// r[8:12] // lifetime
	externalPort = binary.BigEndian.Uint16(r[6:8])
	actualLifetime = binary.BigEndian.Uint32(r[8:12])

	return
}
