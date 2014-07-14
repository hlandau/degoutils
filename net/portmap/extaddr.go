package portmap
import "github.com/hlandau/degoutils/net"
import gnet "net"

// Attempt to obtain the external IP address from the default gateway.
//
// Currently this only tries NAT-PMP and does not attempt to learn the external
// IP address via UPnP.
//
// This function is not very useful because the IP address returned may still
// be an RFC1918 address, due to the possibility of a double NAT setup. There
// are better solutions for obtaining one's public IP address, such as STUN.
func GetExternalAddr() (extaddr gnet.IP, err error) {
  gwa, err := net.GetGatewayAddrs()
  if err != nil {
    return
  }

  for _, gw := range gwa {
    extaddr, err = natpmpGetExternalAddr(gw)
    if err == nil {
      return
    }
  }

  return
}
