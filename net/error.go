package net

import gnet "net"
import "syscall"

func eq(a error, b error) bool {
	xa, ok := a.(*gnet.OpError)
	if !ok {
		return a == b
	}

	return xa.Err == b
}

func ErrorIsConnRefused(e error) bool {
	return eq(e, syscall.ECONNREFUSED)
	//return xo.Err == syscall.ECONNREFUSED // .Error() == "connection refused" // XXX
}

func ErrorIsConnReset(e error) bool {
	return eq(e, syscall.ECONNRESET)
}

func ErrorIsConnAborted(e error) bool {
	return eq(e, syscall.ECONNABORTED)
}

// Hacky function to determine whether an error returned from UDPConn.Write
// corresponds to an ICMP Port Unreachable message.
func ErrorIsPortUnreachable(e error) bool {
	return ErrorIsConnRefused(e)
}
