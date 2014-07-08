package net
import zmq "github.com/pebbe/zmq4"
import "errors"
import "net/url"
import gnet "net"
import "crypto/tls"
import "github.com/hlandau/degoutils/log"
import "fmt"

const (
  CET_ProgressInfo    = 1
  CET_Connected       = 2
  CET_MethodFailure   = 3
  CET_AttemptFailure  = 4
  CET_FinalFailure    = 5
  )

type ConnEx interface {
  gnet.Conn
}

type ConnectionEvent struct {
  Type int
  TransportLayerConnectionAttemptNo int
  ServiceAttemptNo int
  ProgressInfo string
  Conn ConnEx
}

type Connector interface {
  Chan() chan ConnectionEvent
  Event() (ConnectionEvent, error)
  Sock() (ConnEx, error)
  Abort()
}

type ZMQConfigurator func(sock *zmq.Socket) error

type ConnectConfig struct {
  RetryConfig
  MethodDescriptor    string

  Dialer              gnet.Dialer
  TLSConfig          *tls.Config

  ZMQConfigurator     ZMQConfigurator

  CurveZMQPrivateKey  string // z85
  ZMQNoNullAuth       bool
  ZMQNoPlainAuth      bool
  zmqIdentity         string
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
}

func (self *connector) Chan() chan ConnectionEvent {
  return self.ch
}

func (self *connector) Event() (ConnectionEvent, error) {
  x := <-self.ch
  return x, nil
}

func (self *connector) Sock() (c ConnEx, err error) {
  var e ConnectionEvent
  for {
    e, err = self.Event()
    if err != nil {
      return
    }
    if e.Type == CET_Connected {
      c = e.Conn
      return
    }
    if e.Type == CET_FinalFailure {
      err = errors.New("final connect failure")
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

func (self *connector) asyncConnectMethodPort(transport string, hostname string, port int) (err error) {
  cs := fmt.Sprintf("%s:%d", hostname, port)
  log.Info("Attempting to connect to hostname: ", cs)
  conn, err := self.cc.Dialer.Dial(transport, cs)
  if err != nil {
    return err
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

    err := self.asyncConnectMethodPort(m.explicitMethodName, addrs[i].Target, int(addrs[i].Port))
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
    err = self.asyncConnectMethodPort(m.explicitMethodName, self.urlHostname, port)
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

func Connect(us string, cc ConnectConfig) (ConnEx, error) {
  ctor, err := ConnectEx(us, cc)
  if err != nil {
    return nil, err
  }

  return ctor.Sock()
}
