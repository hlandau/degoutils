package net

import "net"
import "strconv"

// Like net.SplitHostPort, but supports absence of port. For example,
// "192.0.2.1" is accepted.
func FuzzySplitHostPort(hostport string) (host, port string, err error) {
	host, port, err = net.SplitHostPort(hostport)
	if err != nil {
		// YUCK
		if e, ok := err.(*net.AddrError); ok && e.Err == "missing port in address" {
			return net.SplitHostPort(hostport + ":")
		}
	}

	return
}

// Like FuzzySplitHostPort, but returns an integer for the port. Supports
// symbolic port names (e.g. "http"). If the port name is unknown, returns error.
//
// "net" must be the port namespace (e.g. "tcp" or "udp"). If "", the port must
// be given as an integer.
//
// If a port was not specified, port is 0.
func FuzzySplitHostPortI(network, hostport string) (host string, port uint16, err error) {
	host, ports, err := FuzzySplitHostPort(hostport)
	if err != nil {
		return
	}

	if ports == "" {
		return
	}

	px, err := strconv.ParseUint(ports, 10, 16)
	p := int(px)
	if err != nil {
		if network == "" {
			err = &net.AddrError{Err: "port must be numeric", Addr: ports}
			return
		}

		p, err = net.LookupPort(network, ports)
		if err != nil {
			return
		}
	}

	if p < 0 || p > 0xFFFF {
		err = &net.AddrError{Err: "invalid port", Addr: ports}
		return
	}

	port = uint16(p)
	return
}

// Like FuzzySplitHostPortIPI, but the host part must be an IP address, not a hostname.
func FuzzySplitHostPortIPI(network, hostport string) (host net.IP, port uint16, err error) {
	h, port, err := FuzzySplitHostPortI(network, hostport)
	if err != nil {
		return
	}

	ip := net.ParseIP(h)
	if ip == nil {
		err = &net.AddrError{Err: "invalid IP", Addr: h}
		return
	}

	host = ip
	return
}
