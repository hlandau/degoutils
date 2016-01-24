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

	type txResult struct {
		Response *dns.Msg
		Err      error
	}

	maxTries := len(servers)
	if maxTries < 3 {
		maxTries = 3
	}

	for i := 0; i < maxTries; i++ {
		s := servers[i%len(servers)]

		txResultChan := make(chan txResult, 1)

		go func() {
			r, _, err := cl.Exchange(m, s+":53")
			txResultChan <- txResult{r, err}
		}()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case txResult := <-txResultChan:
			if txResult.Err == nil {
				return txResult.Response, nil
			}

			err = txResult.Err
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
