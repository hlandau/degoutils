// A utility for automatically connecting to services using SRV records, TLS, ZeroMQ, etc.
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
//   "zorg=@zorg;zorgzmq$zmq+tcp;11011$zmq+tcp"
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
// last entry, even if it is after a fail method.
// If fail is specified as the last entry, explicit port specification is not
// possible, because explicit port specification can only occur when the last
// entry names a port.
//
// All SRV and PTR methods fail when an IP address is specified.
// In some cases failure of a SRV method may result in execution of the methods
// not continuing, e.g. when a non-zero number of SRV records exists for a
// method but connection to all of them fails.
//
// If a ZMQ connection occurs, the following authentication methods are
// attempted, in order:
//   CURVE
//     Attempted if a "username" is specified in the URL and a client private
//     key is specified in the ConnectConfig. The "username" must be the public
//     key in lowercase base32 encoding with no base32 padding.
//     e.g. scheme://urmbzh7aqu5kqojvrywowq6mfw74blighkxz4j5fcrqmbrhh3rua@hostname
//
//   PLAIN
//     Attempted if a username and password are specified in the URL in the
//     form scheme://username:password@hostname
//   NULL
//     Attempted if no other method succeeds.
//
// If connection is made via ZMQ, the path in the URL is passed to ZMQ as the
// resource string.
//
// TODO
//
// Currently not implemented: _svc, ZMQ, SCTP.
//
package connect

import zmq "github.com/pebbe/zmq4"
import "errors"
import "net/url"
import gnet "net"
import "crypto/tls"
import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/net"
import "fmt"

const (
  // An event of no particular external significance, but which provides
  // updated progress information.
  CET_ProgressInfo    = 1

  // The Connector has finished connecting and a socket is now available.
  CET_Connected       = 2

  // A single method tried in a single attempt has failed. (Each attempt may
  // involve trying several methods.)
  CET_MethodFailure   = 3

  // The Connector has failed a single attempt; the number of attempts has been
  // incremented.
  CET_AttemptFailure  = 4

  // The Connector has exhausted all attempts and has given up.
  CET_FinalFailure    = 5
  )

type ConnEx interface {
  gnet.Conn
}

type ConnectionEvent struct {
  // The type of the connection event. See the CET_* constants.
  Type int

  TransportLayerConnectionAttemptNo int
  ServiceAttemptNo int

  // A string summarising the task currently being attempted or just completed.
  ProgressInfo string

  // CET_Connected only: The successfully created connection.
  Conn ConnEx
}

type Connector interface {
  // Yields the underlying channel used to receive ConnectionEvents from the
  // connection process goroutine.
  Chan() chan ConnectionEvent

  // Wait for the next ConnectionEvent and return it.
  //
  // This function will fail with an error if the Connector is stale. The Connector becomes
  // stale after a Final Failure or Connected ConnectionEvent. This is necessary
  // because no further events will be sent after such an event, so waiting for
  // an event would simply cause the caller to wait forever.
  Event() (ConnectionEvent, error)

  // Wait for a socket to become available or for connection to fail finally.
  //
  // Note that if an unlimited number of retries are configured and connection
  // never succeeds, this function may never return.
  //
  // The Connector is stale once this function returns.
  Sock() (ConnEx, error)

  // Abort the connection effort at the next available opportunity.
  //
  // A FinalFailure event will be delivered when abortion is complete.
  Abort()
}

// Function prototype for the ZMQ Configurator function. See ConnectConfig.
type ZMQConfigurator func(sock *zmq.Socket) error

type ConnectConfig struct {
  // Controls how many times connection will be attempted, and the delay
  // between each attempt.
  RetryConfig         net.RetryConfig

  // The Connection Method Description String.
  MethodDescriptor    string

  // The Dialer used to initiate actual Transport-layer connections.
  Dialer              gnet.Dialer

  // The TLS Configuration used for any TLS connection made.
  TLSConfig          *tls.Config

  // If ZeroMQ is used, this function is called on any ZeroMQ socket
  // constructed immediately after its construction. This allows you to set
  // arbitrary settings on that socket. May be nil, in which case no function
  // is called.
  ZMQConfigurator     ZMQConfigurator

  //CurveZMQPrivateKey  string // z85
  //ZMQNoNullAuth       bool
  //ZMQNoPlainAuth      bool
  //zmqIdentity         string
}

type connector struct {
  ch chan ConnectionEvent
  abortCh chan int
  url *url.URL
  urlHostname string
  urlPort int
  cc ConnectConfig
  cmdsApp cmdsApp
  inhibitFallback bool
  stale bool
}

func (self *connector) Chan() chan ConnectionEvent {
  return self.ch
}

func (self *connector) Event() (ConnectionEvent, error) {
  if self.stale {
    return ConnectionEvent{}, errors.New("Connector is stale.")
  }

  x := <-self.ch

  if x.Type == CET_FinalFailure {
    self.stale = true
  }

  return x, nil
}

func (self *connector) Sock() (c ConnEx, err error) {
  var e ConnectionEvent

  if self.stale {
    return nil, errors.New("Final failure already received; Connector is stale.")
  }

  for {
    e, err = self.Event()
    if err != nil {
      return
    }
    if e.Type == CET_Connected {
      c = e.Conn
      self.stale = true
      return
    }
    if e.Type == CET_FinalFailure {
      err = errors.New("final connect failure")
      self.stale = true
      return
    }
  }
}

func (self *connector) Abort() {
  self.abortCh <- 1
}

func (self *connector) asyncNotifyConnected(ce ConnEx) {
  var ev ConnectionEvent = ConnectionEvent {
    Type: CET_Connected,
    ProgressInfo: "Connected",
    Conn: ce,
  }
  log.Info(fmt.Sprintf("async connect: connected: %+v", ev))
  self.ch <- ev
}

func (self *connector) asyncNotifyInterim(t int, progressInfo string) {
  var ev ConnectionEvent = ConnectionEvent {
    Type: t,
    ProgressInfo: progressInfo,
    ServiceAttemptNo: self.cc.RetryConfig.CurrentTry,
  }
  log.Info(fmt.Sprintf("async connect: interim: %+v", ev))
  self.ch <- ev
}

func (self *connector) asyncConnectMethodPort(m cmdsMethod, hostname string, port int) (err error) {
  cs := fmt.Sprintf("%s:%d", hostname, port)
  log.Info("Attempting to connect to hostname: ", cs)
  conn, err := self.cc.Dialer.Dial(m.explicitMethodName, cs)
  if err != nil {
    return err
  }

  switch m.implicitMethodName {
    case "tls":
      // Wrap the connection in TLS.
      var tls_config tls.Config
      if self.cc.TLSConfig != nil {
        tls_config = *self.cc.TLSConfig
      }

      if tls_config.ServerName == "" {
        tls_config.ServerName = self.urlHostname
      }

      tls_c := tls.Client(conn, &tls_config)
      err = tls_c.Handshake()
      if err != nil {
        return
      }
      log.Info("TLS handshake completed OK")

      cstate := tls_c.ConnectionState()
      log.Info(fmt.Sprintf("TLS State: %+v", cstate))

      conn = tls_c

    case "zmq":
      // ...

    case "":
      // Nothing to do.

    default:
      panic("unreachable")
  }

  self.cc.RetryConfig.Reset()
  self.asyncNotifyConnected(conn)
  return nil
}

func hostnameIsIP(hostname string) bool {
  // hostname should be in one of the following forms:
  //
  //   127.0.0.1      } returns true
  //   [::1]          }
  //
  //   foo.bar        } returns false
  if hostname == "" {
    return true
  }

  if hostname[0] == '[' {
    return true
  }

  return gnet.ParseIP(hostname) != nil
}

func (self *connector) asyncConnectMethodSRV(m cmdsMethod) error {
  if hostnameIsIP(self.urlHostname) {
    return errors.New("cannot do SRV lookup on IP address")
  }

  _, addrs, err := gnet.LookupSRV(m.name, m.explicitMethodName, self.urlHostname)
  if err != nil {
    return err
  }

  for i := range addrs {
    if addrs[i].Target == "." {
      continue
    }

    err := self.asyncConnectMethodPort(m, addrs[i].Target, int(addrs[i].Port))
    if err != nil {
      continue
    }

    return nil
  }

  if len(addrs) > 0 {
    self.inhibitFallback = true
  }

  return errors.New("all SRV endpoints failed")
}

var eFailMethod = errors.New("Fail directive reached.")

func (self *connector) asyncConnectMethod(m cmdsMethod) (err error) {
  if m.methodType == cmdsMT_FAIL {
    err = eFailMethod
    return
  }

  if m.name != "" {
    err = self.asyncConnectMethodSRV(m)
    return
  }

  if self.inhibitFallback {
    err = errors.New("Not using hostname fallback because a nonzero number of SRV records were received.")
    return
  }

  if m.port != -1 {
    port := m.port
    if self.urlPort != -1 {
      port = self.urlPort
    }
    err = self.asyncConnectMethodPort(m, self.urlHostname, port)
    return
  }

  err = errors.New("???")
  return
}

var eAborted = errors.New("Connection Aborted")

func (self *connector) updateProgress() error {
  select {
    case _ = <-self.abortCh:
      self.asyncNotifyInterim(CET_FinalFailure, "Connection Aborted")
      return eAborted
    default:
  }
  return nil
}

func (self *connector) asyncConnectAttempt() error {
  ms := self.cmdsApp.methods
  if self.urlPort != -1 {
    // if a port is explicitly specified, use only the last method
    ms = ms[len(ms)-1:]
  }

  self.inhibitFallback = false

  for i := range ms {
    if err := self.updateProgress(); err != nil {
      return err
    }

    m := self.cmdsApp.methods[i]
    log.Info(fmt.Sprintf("method: %+v", m))
    err := self.asyncConnectMethod(m)
    if err == nil {
      // done
      return nil
    }

    self.asyncNotifyInterim(CET_MethodFailure, "Method failed")
    if err == eFailMethod {
      self.asyncNotifyInterim(CET_AttemptFailure, "Fail method reached")
      return err
    }
  }

  self.asyncNotifyInterim(CET_AttemptFailure, "All methods exhausted")
  return errors.New("All methods exhausted")
}

func (self *connector) asyncConnect() {
  for {
    d := self.cc.RetryConfig.GetStepDelay()
    if d == 0 {
      break
    }

    err := self.asyncConnectAttempt()
    if err == nil {
      // done
      log.Info("async connect goroutine completed")
      return
    }
  }

  self.asyncNotifyInterim(CET_FinalFailure, "Retry limit exceeded")
}

// Like Connect, but provides a more advanced, asynchronous interface.
//
// Yields either an error (for immediate failures) or a Connector interface,
// which can be used to receive events indicating the progress of the
// connection effort.
//
// The Connector interface wraps a Channel yielding ConnectionEvents.
// The interface can also be used to abort the connection effort.
func ConnectEx(us string, cc ConnectConfig) (ctor Connector, err error) {
  u, err := url.Parse(us)
  if err != nil {
    return
  }

  cmds, err := parseCmds(cc.MethodDescriptor)
  if err != nil {
    return
  }

  var app cmdsApp
  var ok bool
  if app, ok = cmds[u.Scheme]; !ok {
    err = errors.New("unsupported scheme")
    return
  }

  c := connector {
    ch: make(chan ConnectionEvent, 5),
    abortCh: make(chan int, 5),
    url: u,
    urlHostname: u.Host,
    urlPort: -1,
    cc: cc,
    cmdsApp: app,
  }

  go c.asyncConnect()
  ctor = &c
  return
}

// This connects to an URL using the information specified in ConnectConfig.
// It's like Dial on steroids. It can automatically do SRV lookups and fallback
// to a conventional hostname lookup as appropriate. It can also automatically
// wrap the connection in TLS or ZeroMQ/CurveZMQ where appropriate.
//
// An open socket or error is returned.
//
// The connection process is primarily controlled via a Connection Method
// Description String, which describes how to connect to various URL schemes.
//
// This function will automatically keep trying to connect, based on the retry
// configuration specified in the ConnectConfig. If the retry configuration
// proscribes a limited number of attempts, the function returns after the
// maximum number of attempts has been made. An exponential backoff is used to
// delay each successive attempt.
//
// If you require more detailed information on the connection process (for
// example, because you would like to log a message whenever an attempt fails),
// use ConnectEx.
func Connect(us string, cc ConnectConfig) (ConnEx, error) {
  ctor, err := ConnectEx(us, cc)
  if err != nil {
    return nil, err
  }

  return ctor.Sock()
}
