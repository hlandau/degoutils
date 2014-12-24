package net

import "os"
import gnet "net"

var hostname string

func init() {
	names, err := gnet.LookupAddr("127.0.0.1")
	if err != nil || len(names) == 0 {
		hn, err := os.Hostname()
		if err != nil {
			return
		}

		hostname = hn
		return
	}

	hostname = names[0]
}

// Returns the fully-qualified hostname for the local machine. May return "" if
// a hostname cannot be determined. Note that this may return a non-meaningful
// hostname like "localhost.localdomain".
func Hostname() string {
	return hostname
}
