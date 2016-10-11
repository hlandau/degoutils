// +build go1.7

// Package unixhttp provides utilities to allow HTTP connections to be made
// over Unix domain sockets.
//
// URLs should take the following form:
//
//   "http://localhost:!var!run!somesocket/..."
//
// The format is "localhost:PATH". The hostname must be "localhost" exactly (in lowercase).
// The path is specified as the port. For the path, forward slashes cannot
// appear in the hostname part of the URL (at least with Go's URL parser), so
// the following escaping scheme is used:
//
//   "!": escapes "/". This is equivalent to "&2f". For a literal "!", use "&21".
//   "&xx": escapes character with hex code XX
//     (same scheme as '%xx' URL encoding, but with '&' instead)
//
// Normal HTTP URLs are unaffected.
//
// For abstract paths (a separate, non-filesystem namespace for Unix domain
// sockets supported by Linux), the Go convention is that these paths begin
// with '@', for example '@foo'. (The system call expects these paths to begin
// with a zero byte; the Go "net" library converts the leading '@' to the
// expected zero byte.) If you specify '@' in an URL, that which precedes it
// will be taken to be userinfo; therefore, you must escape '@' as '&40':
//
//   "http://localhost:&40foo/..."
//
package unixhttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// Unmangles a mangled Unix domain socket path such as
// "localhost:!foo!bar!baz". Returns the path ("/foo/bar/baz") if the input is
// such a mangled path, or "" otherwise (e.g. if it is a normal hostname:port
// value).
func UnmangleUnix(host string) string {
	if !strings.HasPrefix(host, "localhost:") {
		return ""
	}

	host = host[10:]
	host = strings.Replace(host, "!", "/", -1)
	host = strings.Replace(host, "&", "%", -1)
	host, err := url.QueryUnescape(host)
	if err != nil {
		return ""
	}

	return host
}

// Returns true iff the given hostname:port specification is a mangled Unix
// domain socket path.
func IsUnixAddress(addr string) bool {
	return UnmangleUnix(addr) != ""
}

// Wraps a DialContext function such that Unix domain socket paths mangled into
// the form "localhost:!foo!bar!baz" are supported. (See package comment).
func WrapDialContext(f func(context.Context, string, string) (net.Conn, error)) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, net, addr string) (net.Conn, error) {
		addr2 := UnmangleUnix(addr)
		if addr2 != "" {
			net = "unix"
			addr = addr2
		}
		return f(ctx, net, addr)
	}
}

func reqIsUnix(req *http.Request) bool {
	h := req.URL.Host
	if req.Host != "" {
		h = req.Host
	}
	return IsUnixAddress(h)
}

// Wraps a http.Client.CheckRedirect function such that redirects to Unix
// domain socket paths are prevented if there is any request in the sequence
// (whether the initial request or a previous redirect) which uses non-Unix
// transport.
func WrapCheckRedirect(f func(req *http.Request, via []*http.Request) error) func(req *http.Request, via []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if reqIsUnix(req) {
			for _, viaReq := range via {
				if !reqIsUnix(viaReq) {
					return errors.New("cannot redirect to Unix address via non-Unix address")
				}
			}
		}

		if f == nil {
			// copied from net/http.defaultCheckRedirect
			if len(via) >= 10 {
				return errors.New("stopped after 10 redirects")
			}
			return nil
		}

		return f(req, via)
	}
}

// Enable the passed http.Transport for Unix domain socket connections.
func EnableTransport(t *http.Transport) {
	t.DialContext = WrapDialContext(t.DialContext)
}

// Wraps a client using WrapCheckRedirect to prevent net-to-Unix redirects.
func RestrictRedirects(c *http.Client) {
	c.CheckRedirect = WrapCheckRedirect(c.CheckRedirect)
}

// Enable the global net/http DefaultTransport for Unix domain socket
// connections.
//
// Also calls RestrictRedirects on the default client. Note that enabling
// a transport for Unix socket address is a really bad idea if you also
// use that transport to access untrusted resources unless you also
// take measures to inhibit net-to-Unix redirects via RestrictRedirects.
func EnableGlobally() {
	EnableTransport(http.DefaultTransport.(*http.Transport))
	RestrictRedirects(http.DefaultClient)
}
