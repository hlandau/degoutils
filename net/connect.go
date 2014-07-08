// A utility for automatically connecting to services using SRV records, TLS, ZeroMQ, etc.
package net

import zmq "github.com/pebbe/zmq4"
import "errors"
import "net/url"
import gnet "net"
import "crypto/tls"
import "github.com/hlandau/degoutils/log"
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
  RetryConfig         RetryConfig

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
    return nil, errors.New("Connector is stale.")
  }

  x := <-self.ch

  if x.Type == CET_FinalFailure {
    self.finalFailureReceived = true
  }

  return x, nil
}

func (self *connector) Sock() (c ConnEx, err error) {
  var e ConnectionEvent

  if self.finalFailureReceived {
    return nil, errors.New("Final failure already received; Connector is stale.")
  }

  for {
    e, err = self.Event()
    if err != nil {
      return
    }
    if e.Type == CET_Connected {
      c = e.Conn
      self.finalFailureReceived = true
      return
    }
    if e.Type == CET_FinalFailure {
      err = errors.New("final connect failure")
      self.finalFailureReceived = true
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
    ServiceAttemptNo: self.cc.RetryConfig.currentTry,
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
