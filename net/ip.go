package net

import gnet "net"

func IsRFC1918(ip gnet.IP) bool {
	if ip4 := ip.To4(); ip4 != nil {
		return ip4[0] == 10 || (ip4[0] == 192 && ip4[1] == 168) ||
			(ip4[0] == 172 && (ip4[1]&0xF0) == 16)
	}

	return false
}
