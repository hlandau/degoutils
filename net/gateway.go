package net
import gnet "net"

// Get the addresses of default gateways for this host.
// Both IPv4 and IPv6 default gateways are returned and each protocol may have
// more than one default gateway. IPv4 default gateways are returned in
// IPv6-mapped format.
func GetGatewayAddrs() ([]gnet.IP, error) {
  return getGatewayAddrs()
}
