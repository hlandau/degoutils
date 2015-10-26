package originfuncs

import "net"
import "net/http"
import "strings"
import "strconv"
import "github.com/golang/gddo/httputil/header"
import denet "github.com/hlandau/degoutils/net"

type LegFrom byte

const (
	FromForwarded LegFrom = iota
	FromXForwardedFor
	FromXRealIP
	FromLocal
)

// A leg represents a leg of a request. A leg has source and destination IPs
// and ports and uses either HTTP or HTTPS.
type Leg struct {
	SourceIP        net.IP  // nil if unknown
	SourcePort      uint16  // 0 if unknown
	DestinationIP   net.IP  // nil if unknown
	DestinationPort uint16  // 0 if unknown
	Scheme          string  // "" (unknown) or "http" or "https"
	Host            string  // HTTP host
	From            LegFrom // information source ("X-Forwarded-For" or "X-Real-IP" or "Forwarded" or "local")
}

func Parse(req *http.Request) []Leg {
	legs, _ := ParseRFC7239(req.Header)
	legs2, _ := ParseXForwardedFor(req.Header)
	legs = append(legs, legs2...)
	legs2, _ = ParseXRealIP(req.Header)
	legs = append(legs, legs2...)
	legs2, _ = ParseLocal(req)
	legs = append(legs, legs2...)
	return legs
}

// Return the local request leg.
func ParseLocal(req *http.Request) (legs []Leg, err error) {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}

	ip, port, err := denet.FuzzySplitHostPortIPI("", req.RemoteAddr)
	if err != nil {
		return nil, err
	}

	return []Leg{Leg{
		SourceIP:   ip,
		SourcePort: port,
		Host:       req.Host,
		Scheme:     scheme,
		From:       FromLocal,
	}}, nil
}

// Parse the "X-Real-IP"/"X-Real-Protocol"/"X-Local-IP" header.
func ParseXRealIP(hdr http.Header) (legs []Leg, err error) {
	r := strings.TrimSpace(hdr.Get("X-Real-IP"))
	if r == "" {
		return nil, nil
	}

	ip, port, err := denet.FuzzySplitHostPortIPI("", r)
	if err != nil {
		return nil, err
	}

	p := strings.TrimSpace(hdr.Get("X-Real-Protocol"))

	r2 := strings.TrimSpace(hdr.Get("X-Local-IP"))
	var ip2 net.IP
	var port2 uint16
	if r2 != "" {
		ip2, port2, err = denet.FuzzySplitHostPortIPI("", r2)
		if err != nil {
			ip2 = nil
			port2 = 0
		}
	}

	return []Leg{Leg{
		SourceIP:        ip,
		SourcePort:      port,
		DestinationIP:   ip2,
		DestinationPort: port2,
		Scheme:          p,
		From:            FromXRealIP,
	}}, nil
}

// Parse the "X-Forwarded-For" header.
func ParseXForwardedFor(hdr http.Header) (legs []Leg, err error) {
	parts := header.ParseList(hdr, "X-Forwarded-For")

	for _, p := range parts {
		p = strings.TrimSpace(p)
		ip, port, err := denet.FuzzySplitHostPortIPI("", p)
		if err != nil {
			return nil, err
		}

		legs = append(legs, Leg{
			SourceIP:   ip,
			SourcePort: port,
			From:       FromXForwardedFor,
		})
	}

	return
}

// Parse the RFC 7239 "Forwarded" header and returns the legs described in the
// header.
func ParseRFC7239(hdr http.Header) (legs []Leg, err error) {
	return parseForwarded(header.ParseList(hdr, "Forwarded"))
}

func parseForwarded(forwarded []string) (legs []Leg, err error) {
	for _, f := range forwarded {
		leg := Leg{
			From: FromForwarded,
		}

		for len(f) > 0 {
			k, v, rest, err := parsePart(f)
			if err != nil {
				break
			}
			switch k {
			case "for":
				ip, port, err := denet.FuzzySplitHostPortIPI("", v)
				if err != nil {
					return nil, err
				}

				leg.SourceIP, leg.SourcePort = ip, port

			case "by":
				ip, port, err := denet.FuzzySplitHostPortIPI("", v)
				if err != nil {
					return nil, err
				}

				leg.DestinationIP, leg.DestinationPort = ip, port

			case "proto":
				leg.Scheme = v

			case "host":
				leg.Host = v
			}
			f = rest
		}

		legs = append(legs, leg)
	}

	return
}

func parsePart(part string) (k, v, rest string, err error) {
	// `key=value;otherKey=otherValue`
	// `key="value";otherKey=otherValue`
	idx := strings.IndexByte(part, '=')
	if idx < 0 {
		err = strconv.ErrSyntax
		return
	}

	k, v = part[0:idx], part[idx+1:]
	// k=`key`, v=`value;otherKey=otherValue`
	// k=`key`, v=`"value";otherKey=otherValue`
	if len(v) > 0 && v[0] == '"' {
		v, rest, err = unquote(v)
		if err != nil {
			return
		}
	} else {
		idx = strings.IndexByte(v, ';')
		if idx >= 0 {
			v, rest = v[0:idx], v[idx+1:]
		}
	}

	if len(rest) > 0 && rest[0] == ';' {
		rest = rest[1:]
	}

	return
}

func unquote(s string) (t, rest string, err error) {
	if len(s) < 2 || s[0] != '"' {
		return "", s, strconv.ErrSyntax
	}

	s = s[1:]
	buf := make([]byte, 0, len(s))
	for {
		if s == "" {
			return "", s, strconv.ErrSyntax
		}

		if s[0] == '\\' {
			s = s[1:]
			if s == "" {
				return "", s, strconv.ErrSyntax
			}
		} else if s[0] == '"' {
			return string(buf), s[1:], nil
		}

		buf = append(buf, s[0])
		s = s[1:]
	}
}
