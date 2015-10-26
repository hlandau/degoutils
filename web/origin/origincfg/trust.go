package origincfg

import "github.com/hlandau/degoutils/web/origin/originfuncs"
import "gopkg.in/hlandau/easyconfig.v1/cflag"

// The trust function to be used. Can be set by configurable.
var TrustPolicy = "last"
var trustForwardedFlag = cflag.StringVar(nil, &TrustPolicy, "trustforwarded", "last", "What Forwarded headers to trust? (last|forwarded/1|x-real-ip)")

var trustFuncs = map[string]originfuncs.LegFunc{}

func RegisterTrustFunc(name string, tf originfuncs.LegFunc) {
	trustFuncs[name] = tf
}

func init() {
	RegisterTrustFunc("forwarded/1", originfuncs.TrustForwardedN(1))
	RegisterTrustFunc("x-real-ip", originfuncs.TrustXRealIP)
	RegisterTrustFunc("last", originfuncs.TrustLast)
}

func TrustByConfig(leg *originfuncs.Leg, distance int) bool {
	v := trustForwardedFlag.Value()
	f, ok := trustFuncs[v]
	return ok && f(leg, distance)
}

func TrustedLegs(legs []originfuncs.Leg) []originfuncs.Leg {
	return originfuncs.TrustedLegs(legs, TrustByConfig)
}
