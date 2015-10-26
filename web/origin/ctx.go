package origin

import "net/http"
import "github.com/gorilla/context"
import "github.com/hlandau/degoutils/web/origin/originfuncs"
import "github.com/hlandau/degoutils/web/origin/origincfg"

var legsKey int

// Uses gorilla/context. Returns the legs for the request. Automatically parses
// the legs if they have not been parsed yet and caches the outcome using
// gorilla/context. If you aren't yourself using gorilla/context, remember to
// set it up to clear context data for exiting requests.
func Legs(req *http.Request) []originfuncs.Leg {
	v, ok := context.GetOk(req, &legsKey)
	if !ok {
		v = originfuncs.Parse(req)
		context.Set(req, &legsKey, v)
	}
	return v.([]originfuncs.Leg)
}

// Like Legs, but returns only trusted legs.
func TrustedLegs(req *http.Request) []originfuncs.Leg {
	return origincfg.TrustedLegs(Legs(req))
}

// Like Legs, but returns only the earliest trusted leg (or nil).
func EarliestTrustedLeg(req *http.Request) *originfuncs.Leg {
	tl := TrustedLegs(req)
	if len(tl) == 0 {
		panic("no trusted legs should not be a possible condition")
		//return nil
	}

	return &tl[0]
}

func IsSSL(req *http.Request) bool {
	return EarliestTrustedLeg(req).Scheme == "https"
}
