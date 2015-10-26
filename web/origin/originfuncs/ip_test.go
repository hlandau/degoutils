package originfuncs_test

import "github.com/hlandau/degoutils/web/origin/originfuncs"
import "net"
import "net/http"
import "testing"
import "reflect"
import "strings"

type test struct {
	In     []string
	Result []originfuncs.Leg
}

var tests = []test{
	{
		In: []string{`by=192.0.2.1;for=192.0.2.2;proto=https`, `by="192.0.3.1";for="[2001::beef]";proto="http";host="this\"host"`, `by=192.0.4.1;for=192.0.4.2;proto=https`},
		Result: []originfuncs.Leg{
			{
				SourceIP:      net.ParseIP("192.0.2.2"),
				DestinationIP: net.ParseIP("192.0.2.1"),
				Scheme:        "https",
				From:          originfuncs.FromForwarded,
			},
			{
				SourceIP:      net.ParseIP("2001::beef"),
				DestinationIP: net.ParseIP("192.0.3.1"),
				Scheme:        "http",
				Host:          `this"host`,
				From:          originfuncs.FromForwarded,
			},
			{
				SourceIP:      net.ParseIP("192.0.4.2"),
				DestinationIP: net.ParseIP("192.0.4.1"),
				Scheme:        "https",
				From:          originfuncs.FromForwarded,
			},
		},
	},

	{
		In: []string{`by=192.0.2.1:1234;for=192.0.2.2:65535;proto=https`, `by="192.0.3.1:3637";for="[2001::beef]:3636";proto="http";host="this\"host"`, `by=192.0.4.1:24;for=192.0.4.2:555;proto=https`},
		Result: []originfuncs.Leg{
			{
				SourceIP:        net.ParseIP("192.0.2.2"),
				SourcePort:      65535,
				DestinationIP:   net.ParseIP("192.0.2.1"),
				DestinationPort: 1234,
				Scheme:          "https",
				From:            originfuncs.FromForwarded,
			},
			{
				SourceIP:        net.ParseIP("2001::beef"),
				SourcePort:      3636,
				DestinationIP:   net.ParseIP("192.0.3.1"),
				DestinationPort: 3637,
				Scheme:          "http",
				Host:            `this"host`,
				From:            originfuncs.FromForwarded,
			},
			{
				SourceIP:        net.ParseIP("192.0.4.2"),
				SourcePort:      555,
				DestinationIP:   net.ParseIP("192.0.4.1"),
				DestinationPort: 24,
				Scheme:          "https",
				From:            originfuncs.FromForwarded,
			},
		},
	},
}

func TestForwarded(t *testing.T) {
	for _, tst := range tests {
		for i := 0; i < 2; i++ {
			h := http.Header{}
			if i == 0 {
				h.Set("Forwarded", strings.Join(tst.In, ", "))
			} else {
				for _, v := range tst.In {
					h.Add("Forwarded", v)
				}
			}
			legs, err := originfuncs.ParseRFC7239(h)
			if err != nil {
				t.Fatalf("err: %v", err)
			}

			if !reflect.DeepEqual(legs, tst.Result) {
				t.Fatalf("not equal: %v, %v", legs, tst.Result)
			}
		}
	}
}
