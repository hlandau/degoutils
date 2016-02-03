// Package servicenexus provides prefixed access to the net/http
// DefaultServeMux, gated to admit only internal requests.
package servicenexus

import (
	denet "github.com/hlandau/degoutils/net"
	"github.com/hlandau/degoutils/web/origin"
	"github.com/hlandau/degoutils/web/origin/originfuncs"
	"net/http"
	"strings"
)

func Handler(notFoundHandler http.Handler) http.Handler {
	if notFoundHandler == nil {
		notFoundHandler = http.HandlerFunc(http.NotFound)
	}

	// Don't use http.StripPrefix since we need to determine if it begins with
	// the prefix first and fallback if it doesn't.
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		p := strings.TrimPrefix(req.URL.Path, "/.service-nexus")
		if len(p) >= len(req.URL.Path) || !CanAccess(req) {
			notFoundHandler.ServeHTTP(rw, req)
			return
		}

		if p == "" {
			p = "/"
		}

		req.URL.Path = p
		http.DefaultServeMux.ServeHTTP(rw, req)
	})
}

// Used to determine whether the service nexus can be accessed. Returns true if
// ALL of the legs in the request have loopback or RFC1918 source IPs.
func CanAccess(req *http.Request) bool {
	return originfuncs.MatchAll(origin.Legs(req), ipCanAccess)
}

func ipCanAccess(leg *originfuncs.Leg, distance int) bool {
	return leg.SourceIP != nil && (leg.SourceIP.IsLoopback() || denet.IsRFC1918(leg.SourceIP))
}
