package originfuncs

import "net"

// Predicate function type for leg trust decisions. Returns true iff a leg is trusted.
//
// distance is the number of legs between the leg specified and the local
// (physical connection) leg. The local leg has a distance of 0, the preceding
// leg has a distance of 1, etc.
//
// The predicates defined in this package currently do not use distance.
type LegFunc func(leg *Leg, distance int) bool

// Trust last leg only.
func TrustLast(leg *Leg, distance int) bool {
	return distance == 0
}

// Trust X-Real-IP.
func TrustXRealIP(leg *Leg, distance int) bool {
	return TrustLast(leg, distance) || leg.From == FromXRealIP
}

// Trust any Forwarded headers which have a distance of at most d.
func TrustForwardedN(maxDistance int) LegFunc {
	return func(leg *Leg, distance int) bool {
		return TrustLast(leg, distance) || (leg.From == FromForwarded && distance <= maxDistance)
	}
}

// Returns a slice of the given slice which is the span of
// trusted legs.
//
// A leg is trusted if trustLegFunc returns true. By definition,
// all trusted legs are contiguously at the end of the slice,
// because these represent legs closer to this machine. For example:
//
//                                                       Trusted                   Trusted
//                                                      _________                 _________
//   {Client Machine} <- Leg -> {ISP Transparent Proxy} <- Leg -> {Load Balancer} <- Leg -> {This Machine}
//
// The information provided by machines you do not control, such as the ISP
// Transparent Proxy, is untrusted.  The information provided by your load
// balancer, and the local information on the 'physical' connection to this
// machine are trusted.
//
// In general, you want to trust the first trusted leg. Thus you inspect
// TrustedLegs(...)[0].
func TrustedLegs(legs []Leg, trustLegFunc LegFunc) []Leg {
	for i := 0; i < len(legs); i++ {
		if !trustLegFunc(&legs[len(legs)-1-i], i) {
			return legs[len(legs)-i:]
		}
	}

	return legs
}

// Returns true if any of the legs match the predicate.
func MatchAny(legs []Leg, legFunc LegFunc) bool {
	for i := 0; i < len(legs); i++ {
		if legFunc(&legs[len(legs)-1-i], i) {
			return true
		}
	}

	return false
}

// Returns true if all of the legs match the predicate.
func MatchAll(legs []Leg, legFunc LegFunc) bool {
	return len(TrustedLegs(legs, legFunc)) == len(legs)
}

// Returns true if any of the source IPs of any of the legs match one of the given prefixes.
//
// Since this examines untrusted legs, this should be used for blacklisting and
// never whitelisting.  i.e. it allows clients to volunteer to be blacklisted,
// e.g. by ISP transparent proxies. But it should never result in *increased*
// access.
func MatchAnyCIDR(legs []Leg, n ...net.IPNet) bool {
	return MatchAny(legs, func(leg *Leg, distance int) bool {
		return anyCIDR(leg.SourceIP, n...)
	})
}

// Returns true if all of the source IPs of all of the legs match one of given prefixes.
//
// Since ALL legs must match, this can be useful for whitelisting, e.g. to ensure
// a request originates from within your network.
func MatchAllCIDR(legs []Leg, n ...net.IPNet) bool {
	return MatchAll(legs, func(leg *Leg, distance int) bool {
		return anyCIDR(leg.SourceIP, n...)
	})
}

func anyCIDR(ip net.IP, nets ...net.IPNet) bool {
	for _, n := range nets {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}
