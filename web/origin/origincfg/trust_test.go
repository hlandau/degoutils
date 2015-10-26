package origincfg_test

import "github.com/hlandau/degoutils/web/origin/originfuncs"
import "net"
import "net/http"
import "crypto/tls"
import "testing"
import "reflect"

type test struct {
	req         *http.Request
	legs        []originfuncs.Leg
	trustedLegs int
}

var tests = []test{
	// ----------------------------------------------
	test{
		req: &http.Request{
			RemoteAddr: "192.0.2.1:4422",
		},
		legs: []originfuncs.Leg{
			{
				SourceIP:   net.ParseIP("192.0.2.1"),
				SourcePort: 4422,
				Scheme:     "http",
				From:       originfuncs.FromLocal,
			},
		},
	},
	// ----------------------------------------------
	test{
		req: &http.Request{
			RemoteAddr: "192.0.2.1:4422",
			TLS:        &tls.ConnectionState{},
		},
		legs: []originfuncs.Leg{
			{
				SourceIP:   net.ParseIP("192.0.2.1"),
				SourcePort: 4422,
				Scheme:     "https",
				From:       originfuncs.FromLocal,
			},
		},
	},
	// ----------------------------------------------
	test{
		req: &http.Request{
			RemoteAddr: "192.0.2.1:4422",
			Header: map[string][]string{
				"X-Forwarded-For": []string{
					"192.0.99.1", "192.0.99.2",
				},
				"X-Real-Ip": []string{
					"192.0.98.1",
				},
				"Forwarded": []string{
					"by=192.0.2.1:1234;for=192.0.3.1:9876;proto=http",
				},
			},
		},
		legs: []originfuncs.Leg{
			{
				SourceIP:        net.ParseIP("192.0.3.1"),
				SourcePort:      9876,
				DestinationIP:   net.ParseIP("192.0.2.1"),
				DestinationPort: 1234,
				Scheme:          "http",
				From:            originfuncs.FromForwarded,
			},
			{
				SourceIP: net.ParseIP("192.0.99.1"),
				From:     originfuncs.FromXForwardedFor,
			},
			{
				SourceIP: net.ParseIP("192.0.99.2"),
				From:     originfuncs.FromXForwardedFor,
			},
			{
				SourceIP: net.ParseIP("192.0.98.1"),
				From:     originfuncs.FromXRealIP,
			},
			{
				SourceIP:   net.ParseIP("192.0.2.1"),
				SourcePort: 4422,
				Scheme:     "http",
				From:       originfuncs.FromLocal,
			},
		},
	},
	// ----------------------------------------------
}

func TestOrigin(t *testing.T) {
	for i := range tests {
		legs := originfuncs.Parse(tests[i].req)
		if !reflect.DeepEqual(legs, tests[i].legs) {
			t.Fatalf("leg mismatch: %v != %v", legs, tests[i].legs)
		}
	}
}
