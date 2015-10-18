// Package connect provides functions for connecting to services using SRV
// records, TLS, etc.
//
// Connection Method Description String
//
// A connection method descripton string consists of a series of application
// name descriptors, which correspond to a scheme specified in an URL.
//
//   XALPHA = ALPHA / "_" / "-"
//   NAME = XALPHA *(XALPHA / DIGIT)
//   NZDIGIT = %x31-39
//   port = NZDIGIT *DIGIT
//   PALPHA = XALPHA / "+"
//   PNAME = ALPHA *(PALPHA / DIGIT)
//
//   cmds = app *(1*SP app)
//
//   app  = PNAME "=" [meta_method ";"] *(method ";") method
//   method = conn_method / "fail"
//   meta_method = "@" app_name
//   conn_method = (app_proto_name / port) [implicit_method] explicit_method
//   implicit_method = "$" implicit_method_name
//   explicit_method = "+" explicit_method_name
//
//   app_proto_name = NAME
//   app_name = NAME
//
//   implicit_method_name = "tls" / "zmq"
//     // (all allocated security method names shall also match NAME)
//   explicit_method_name = "tcp" / "udp" / "sctp"
//     // (all allocated security method names shall also match NAME)
//
// The principal difference between implicit methods and explicit methods is
// that implicit methods are not used in the DNS SRV hierarchy.
// Always use the method name sigil specified above for a given method name.
//
// Meta methods support the following lookup convention to correct for
// deficiencies in the design of the DNS SRV system:
//
// DNS SRV looks up *application protocol name/transport protocol name* pairs.
// However what we really want to look up is an *application/service name*,
// which may be provided via many different *application and transport
// protocols*. (For example, the World Wide Web service may run over HTTP or
// SPDY, so there is a distinction between the WWW service and HTTP, which is a
// specific means of realisation.)
//
// In order to correct this shortcoming, the following scheme is proposed:
//
// An application/service name is looked up by prepending an underscore to the
// service name, then appending "._svc." followed by the domain name, and doing
// a lookup for PTR records. Each PTR record, if present, will point to a DNS
// SRV record name. For example:
//
//   _www._svc  IN PTR  _spdys._tcp
//   _www._svc  IN PTR  _https._tcp
//   _www._svc  IN PTR  _http._tcp
//
//   _spdys._tcp IN SRV ...
//   _https._tcp IN SRV ...
//   _http._tcp  IN SRV ...
//
// Because PTR records cannot be used to express a preference, the preference
// of each protocol is determined by the application. This is desirable anyway.
//
// Examples:
//
//   "www=https$tls+tcp;http+tcp;443$tls+tcp;80+tcp"
//   Meaning:
//     To connect to an URL of scheme www, first try SRV at _https._tcp and use
//     TLS. Failing that, try SRV at _http._tcp, plain TCP. Failing that, try a
//     direct connection to the hostname on port 443 using TLS and TCP. Failing
//     that, try a direct connection to the hostname on port 80 using TCP.
//     Failing that, fail.
//
//   "www=@www;spdy$tls+tcp;https$tls+tcp;http+tcp;443$tls+tcp;80+tcp"
//   Meaning:
//     As above, but the PTR lookup scheme above is done for the service name
//     "www". The results are used to crop the SRV lookup methods specified.
//     SRV methods specified in the PTR results but not in the CMDS will not be
//     used.
//
// If an URL is specified with a port, this is interpreted using the last
// method specified in the CMDS for that scheme. The port number specified in
// the method is substituted for the port number specified by the user. Thus
// when a port number is specified it customizes the last method entry. If the
// last method entry does not name a port, connection fails; the URL cannot be
// used with an explicitly specified port.
//
// Connection fails implicitly when the end of the method list is reached.
// However failure can also be specified explicitly using the "fail" method.
// This is useful if a different method should be used when a port is
// explicitly specified, as explicit port specification always customizes the
// last entry, even if it is after a fail method.  If fail is specified as the
// last entry, explicit port specification is not possible, because explicit
// port specification can only occur when the last entry names a port.
//
// All SRV and PTR methods fail when an IP address is specified. In some cases
// failure of a SRV method may result in execution of the methods not
// continuing, e.g. when a non-zero number of SRV records exists for a method
// but connection to all of them fails.
//
// Currently not implemented: _svc, ZMQ, SCTP.
package connect

import "net"
import "net/url"
import "errors"
import "fmt"
import "io"
import "github.com/hlandau/degoutils/net/bsda"

// Information passed to a method function.
type MethodInfo struct {
	// Method-specific information. Shared between all methods.
	Pragma map[string]interface{}

	// The logical hostname, suitable for use with e.g. TLS for server name
	// validation.
	Hostname string

	// The actual hostname:port or IP:port string. For explicit methods, this
	// is used to create the connection.
	NetAddress string

	// The connection URL.
	URL *url.URL
}

type MethodFunc func(conn io.Closer, info *MethodInfo) (io.Closer, error)

var implicitMethodRegistry = map[string]MethodFunc{}
var explicitMethodRegistry = map[string]MethodFunc{}

// Registers a connection method function.
//
// For implicit methods, a function will be called once a connection has been
// established. The function can then wrap the connection as it wishes, and
// should return the wrapped connection.
//
// For explicit methods, the net.Conn passed will be nil. The method function
// should initiate a connection and return net.Conn or an error, using the
// information passed in MethodInfo, particularly NetAddress.
func RegisterMethod(name string, implicit bool, f MethodFunc) {
	r := explicitMethodRegistry
	if implicit {
		r = implicitMethodRegistry
	}

	_, ok := r[name]
	if ok {
		panic("method already registered")
	}

	r[name] = f
}

// A dialer used to make underlying network connections.
type Dialer interface {
	Dial(network, addr string) (net.Conn, error)
}

// Connection configuration information.
type Config struct {
	// The connection method description string.
	MethodDescriptor string

	// If nil, a zero net.Dialer is used.
	Dialer Dialer

	// Method-specific information.
	//
	// Items for known methods:
	//
	//   "tls": *tls.Config.
	//     If not set, a zero value will be used.
	//     If ServerName is not set, a default will be used.
	//
	//   "curvecp": *curvecp.Config.
	//     If client private key is not set, a random one will be generated.
	//     Server key must be set in config or failing that, in URL.
	//
	//   "ws-headers": http.Header.
	//     Websocket request headers.
	//
	Pragma map[string]interface{}
}

type connector struct {
	cfg             Config
	url             *url.URL
	uhost           string
	uport           string
	cmdsApp         cmdsApp
	inhibitFallback bool
}

// This connects to an URL using the information specified in Config.  It's
// like Dial on steroids. It can automatically do SRV lookups and fallback to a
// conventional hostname lookup as appropriate. It can also automatically wrap
// the connection in TLS.
//
// The connection process is primarily controlled via a Connection Method
// Description String, which describes how to connect to various URL schemes.
func Connect(urlString string, cfg Config) (io.Closer, error) {
	u, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	uhost, uport, err := net.SplitHostPort(u.Host)
	if err != nil {
		uhost = u.Host
		uport = ""
	}

	cmds, err := parseCmds(cfg.MethodDescriptor)
	if err != nil {
		return nil, err
	}

	app, ok := cmds[u.Scheme]
	if !ok {
		return nil, errors.New("unsupported scheme")
	}

	c := &connector{
		cfg:     cfg,
		url:     u,
		uhost:   uhost,
		uport:   uport,
		cmdsApp: app,
	}

	if c.cfg.Dialer == nil {
		c.cfg.Dialer = &net.Dialer{}
	}

	conn, err := c.connectionAttempt()
	if err != nil {
		return nil, err
	}

	return conn, nil
}

func ConnectConn(urlString string, cfg Config) (net.Conn, error) {
	cl, err := Connect(urlString, cfg)
	if err != nil {
		return nil, err
	}

	if conn, ok := cl.(net.Conn); ok {
		return conn, nil
	}

	cl.Close()
	return nil, fmt.Errorf("net.Conn not supported")
}

func ConnectFrame(urlString string, cfg Config) (bsda.FrameReadWriterCloser, error) {
	cl, err := Connect(urlString, cfg)
	if err != nil {
		return nil, err
	}

	if conn, ok := cl.(bsda.FrameReadWriterCloser); ok {
		return conn, nil
	}

	cl.Close()
	return nil, fmt.Errorf("net.Conn not supported")
}

func (c *connector) connectionAttempt() (io.Closer, error) {
	ms := c.cmdsApp.methods
	if c.uport != "" {
		// If a port is explicitly specified, use only the last method.
		ms = ms[len(ms)-1:]
	}

	c.inhibitFallback = false

	for _, m := range ms {
		conn, err := c.connectMethod(m)
		if err == nil {
			// done
			return conn, nil
		}
	}

	return nil, errors.New("All methods exhausted")
}

func (c *connector) connectMethod(m cmdsMethod) (io.Closer, error) {
	if m.methodType == cmdsMT_FAIL {
		return nil, errors.New("fail directive reached")
	}

	if m.name != "" {
		return c.connectSRV(m)
	}

	if c.inhibitFallback {
		return nil, errors.New("not using hostname fallback because a nonzero number of SRV records were received")
	}

	if m.port != -1 {
		port := fmt.Sprintf("%v", m.port)
		if c.uport != "" {
			port = c.uport
		}
		return c.connectDial(m, c.uhost, port)
	}

	return nil, errors.New("unknown connection method type")
}

func (c *connector) connectSRV(m cmdsMethod) (io.Closer, error) {
	if hostnameIsIP(c.uhost) {
		return nil, errors.New("cannot do SRV lookup on an IP address")
	}

	_, addrs, err := net.LookupSRV(m.name, m.explicitMethodName, c.uhost)
	if err != nil {
		return nil, err
	}

	c.inhibitFallback = c.inhibitFallback || len(addrs) > 0

	for _, a := range addrs {
		if a.Target == "." {
			continue
		}

		conn, err := c.connectDial(m, a.Target, fmt.Sprintf("%d", a.Port))
		if err != nil {
			continue
		}

		return conn, nil
	}

	return nil, errors.New("all SRV endpoints failed")
}

func (c *connector) connectDial(m cmdsMethod, host, port string) (io.Closer, error) {
	addr := net.JoinHostPort(host, port)

	mi := &MethodInfo{
		Pragma:     c.cfg.Pragma,
		Hostname:   c.uhost,
		NetAddress: addr,
		URL:        c.url,
	}

	var conn io.Closer
	var err error
	if f := explicitMethodRegistry[m.explicitMethodName]; f != nil {
		conn, err = f(nil, mi)
	} else {
		conn, err = c.cfg.Dialer.Dial(m.explicitMethodName, addr)
	}
	if err != nil {
		return nil, err
	}

	// go backwards so the syntax order is more logical
	for i := len(m.implicitMethodName) - 1; i >= 0; i-- {
		implicitMethodName := m.implicitMethodName[i]
		f := implicitMethodRegistry[implicitMethodName]
		if f == nil {
			conn.Close()
			return nil, fmt.Errorf("unknown implicit method %#v", implicitMethodName)
		}

		conn2, err := f(conn, mi)
		if err != nil {
			conn.Close()
			return nil, err
		}

		conn = conn2
	}

	return conn, nil
}

func hostnameIsIP(hostname string) bool {
	if hostname == "" {
		return true
	}
	if hostname[0] == '[' {
		return true
	}
	return net.ParseIP(hostname) != nil
}
