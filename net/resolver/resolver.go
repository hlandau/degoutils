package resolver

import (
	"fmt"
	denet "github.com/hlandau/degoutils/net"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
	"net"
	"sync"
)

// Given a domain name, find the apex for the zone enclosing the name.  e.g. if
// example.com. is a zone, and "www.example.com." is not delegated, given
// "www.example.com.", returns "example.com." If a zone SOA was found during the
// process, return that too (not guaranteed).
func FindZoneApex(name string, ctx context.Context) (apex string, zoneSOA *dns.SOA, err error) {
	msg, err := Query(dns.Question{
		Name:   dns.Fqdn(name),
		Qtype:  dns.TypeSOA,
		Qclass: dns.ClassINET,
	}, ctx)
	if err != nil {
		return nil, nil, err
	}

	for _, a := range msg.Answer {
		if soa, ok := a.(*dns.SOA); ok {
			return soa.Hdr.Name, soa, nil
		}
	}

	// No answer, so the name is not at a zone apex. We should get information on
	// the zone apex in the form of either SOA or NS records or both in the
	// authority section. Since a nameserver can return either SOA or NS records,
	// we can only rely on having the owner name available, which is all we need
	// anyway.
	for _, a := range msg.Ns {
		switch v := a.(type) {
		case *dns.SOA:
			return v.Hdr.Name, v, nil
		case *dns.NS:
			return v.Hdr.Name, nil, nil
		default:
		}
	}

	return "", nil, fmt.Errorf("cannot determine apex name for %s", name)
}

func LookupSOA(name string, ctx context.Context) (*dns.SOA, error) {
	msg, err := Query(dns.Question{
		Name:   dns.Fqdn(name),
		Qtype:  dns.TypeSOA,
		Qclass: dns.ClassINET,
	}, ctx)
	if err != nil {
		return nil, err
	}

	var soas []*dns.SOA
	for _, a := range msg.Answer {
		if soa, ok := a.(*dns.SOA); ok {
			soas = append(soas, soa)
		}
	}

	if len(soas) != 1 {
		return nil, fmt.Errorf("didn't receive SOA (got %d)", len(soas))
	}

	return soas[0], nil
}

func Query(q dns.Question, ctx context.Context) (*dns.Msg, error) {
	servers, err := getServers()
	if err != nil {
		return nil, err
	}

	return DirectedQuery(servers, true, q, ctx)
}

func DirectedQuery(servers []string, rd bool, q dns.Question, ctx context.Context) (*dns.Msg, error) {
	cl := dns.Client{
		Net: "tcp",
	}

	m := &dns.Msg{
		MsgHdr: dns.MsgHdr{
			Id:               dns.Id(),
			RecursionDesired: rd,
		},
		Compress: true,
		Question: []dns.Question{q},
	}

	m = m.SetEdns0(4096, false)

	type txResult struct {
		Response *dns.Msg
		Err      error
	}

	maxTries := len(servers)
	if maxTries < 3 {
		maxTries = 3
	}

	var mainErr error
	for i := 0; i < maxTries; i++ {
		s := servers[i%len(servers)]

		host, port, err := denet.FuzzySplitHostPort(s)
		if err != nil {
			return nil, err
		}
		if port == "" {
			port = "53"
		}

		txResultChan := make(chan txResult, 1)

		go func() {
			r, _, err := cl.Exchange(m, net.JoinHostPort(host, port))
			txResultChan <- txResult{r, err}
		}()

		select {
		case <-ctx.Done():
			return nil, ctx.Err()

		case txResult := <-txResultChan:
			if txResult.Err == nil {
				return txResult.Response, nil
			}

			mainErr = txResult.Err
		}
	}

	return nil, mainErr
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
