package resolver

import (
	"fmt"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"sync"
)

func Query(q dns.Question, ctx context.Context) (*dns.Msg, error) {
	cl := dns.Client{
		Net: "tcp",
	}

	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:               dns.Id(),
			RecursionDesired: true,
		},
		Compress: true,
		Question: []dns.Question{q},
	}

	m = m.SetEdns0(4096, false)

	servers, err := getServers()
	if err != nil {
		return nil, err
	}

	var r *dns.Msg
	for _, s := range servers {
		// TODO: use context
		r, _, err = cl.Exchange(m, s+":53")
		if err == nil {
			return r, nil
		}
	}

	return nil, err
}

func getServers() ([]string, error) {
	err := loadConfig()
	if err != nil {
		return nil, err
	}

	if len(clientConfig.Servers) == 0 {
		return nil, fmt.Errorf("no DNS resolvers configured")
	}

	return clientConfig.Servers, nil
}

var loadConfigOnce sync.Once
var clientConfig *dns.ClientConfig
var configLoadError error

func loadConfig() error {
	loadConfigOnce.Do(func() {
		clientConfig, configLoadError = dns.ClientConfigFromFile("/etc/resolv.conf")
	})

	return configLoadError
}
