package servicenexus

import "net/http"
import denet "github.com/hlandau/degoutils/net"
import "github.com/hlandau/degoutils/web/origin"
import "github.com/hlandau/degoutils/web/origin/originfuncs"

func Handler(notFoundHandler http.Handler) http.Handler {
	if notFoundHandler == nil {
		notFoundHandler = http.HandlerFunc(http.NotFound)
	}

	sp := http.StripPrefix("/.service-nexus", http.DefaultServeMux)
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		if !CanAccess(req) {
			notFoundHandler.ServeHTTP(rw, req)
			return
		}

		sp.ServeHTTP(rw, req)
	})
}

func CanAccess(req *http.Request) bool {
	return originfuncs.MatchAll(origin.Legs(req), ipCanAccess)
}

func ipCanAccess(leg *originfuncs.Leg, distance int) bool {
	return leg.SourceIP != nil && (leg.SourceIP.IsLoopback() || denet.IsRFC1918(leg.SourceIP))
}
