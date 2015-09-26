// +build !linux,!windows

package net

import gnet "net"
import "errors"

var errNotSupportedGw = errors.New("GetGatewayAddrs is not supported on this platform")

func getGatewayAddrs() (gwaddr []gnet.IP, err error) {
	return nil, errNotSupportedGw
}
