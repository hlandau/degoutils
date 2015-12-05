package net

import "errors"
import "net"

const absMaxDatagramSize = 2147483646 // 2**31-2

func getMaxDatagramSize() int {
	var m int = 65535
	ifs, err := net.Interfaces()
	if err != nil {
		for i := range ifs {
			if ifs[i].MTU > m {
				m = ifs[i].MTU
			}
		}
	}
	if m > absMaxDatagramSize {
		m = absMaxDatagramSize
	}
	return m
}

var maxDatagramSize int = getMaxDatagramSize()

// In order to call UDPConn.Read, we have to have a buffer of appropriate size.
// If the buffer is too short for the datagram received, it will be
// truncated. This is undesirable. There is no way to get the length of the
// next datagram without calling Read and go doesn't give us any way to pass
// the PEEK and TRUNC flags, which would allow us to get the size and then
// call Read again.
//
// The maximum size of an IPv6 packet is 2**32-1 using jumbograms. However,
// IPv6 fragmentation supports maximum packet sizes of 2**16-1, so a
// packet of size greater than 2**16-1 can only be sent via a sequence of
// links all having an MTU sufficient to enable unfragmented transmission.
// Therefore, the maximum size of an incoming datagram is 2**16-1 or the MTU,
// whichever is higher. Rather than try and tell which interface a datagram
// arrived on, we simply use the largest MTU of all interfaces on the system.
//
// If a link has an MTU in excess of 2**31-1, then it is possible for a
// datagram to be received of a size unrepresentable by the integer return
// type of the Read method, which is supposed to represent the number of
// bytes read. This is a bug in the Go standard library. Therefore, this
// implementation cannot support reception of datagrams larger than 2**31-1.
// The determined maximum receive size is capped at 2**31-2. Subtracting an
// extra byte from the maximum receive size allows one to determine whether a
// packet may have exceeded the limitation of the Go API; such packets can
// then be discarded rather than being processed. It is assumed that it is
// undesirable to process truncated packets.
//
// This function returns the determined maximum receive size.
func MaxDatagramSize() int {
	return maxDatagramSize
}

// Updates the determined maximum receive size. Necessary if the maximum MTU of
// all the interfaces configured on the system changes, and that value exceeds
// 2**16-1.
func UpdateMaxDatagramSize() int {
	maxDatagramSize = getMaxDatagramSize()
	return maxDatagramSize
}

var WasTruncated error = errors.New("datagram was truncated")

// Reads a datagram into a buffer using the determined maximum receive size,
// truncates the buffer to the length actually received and returns it.
//
// Returns error WasTruncated and an empty slice if the incoming packet may
// have been truncated.
func ReadDatagram(c net.Conn) (buf []byte, err error) {
	bufx := make([]byte, maxDatagramSize+1)
	n, err := c.Read(bufx)
	if n > maxDatagramSize {
		err = WasTruncated
		return
	}

	if n > 0 {
		buf = bufx[0:n]
	}

	return
}

func ReadDatagramFromUDP(c *net.UDPConn) (buf []byte, addr *net.UDPAddr, err error) {
	bufx := make([]byte, maxDatagramSize+1)
	n, addr, err := c.ReadFromUDP(bufx)
	if n > maxDatagramSize {
		err = WasTruncated
		return
	}

	if n > 0 {
		buf = bufx[0:n]
	}

	return
}
