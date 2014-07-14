package net
import gnet "net"

// Hacky function to determine whether an error returned from UDPConn.Write
// corresponds to an ICMP Port Unreachable message.
func ErrorIsPortUnreachable(e error) bool {
  xo, ok := e.(*gnet.OpError)
  if !ok {
    return false
  }
  
  return xo.Err.Error() == "connection refused" // XXX
}
