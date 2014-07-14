
// Package portmap provides a utility for the automatic mapping of TCP and UDP
// ports via NAT-PMP or UPnP IGDv1.
//
// In order to map a TCP or UDP port, just call CreatePortMapping. Negotiation
// via NAT-PMP and, if that fails, via UPnP will be attempted in the
// background.
//
// You can interrogate the returned Mapping object to determine when the
// mapping has been successfully created, and to cancel the mapping.
package portmap

import gnet "net"
//import "errors"
import "time"
import "fmt"
import "github.com/hlandau/degoutils/net"
import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/net/ssdpreg"
import "github.com/hlandau/degoutils/net/portmap/upnp"

const Protocol_TCP = natpmpTCP
const Protocol_UDP = natpmpUDP

type MappingConfig struct {
  // The protocol for which the port should be mapped.
  //
  // Must be Protocol_TCP or Protocol_UDP.
  Protocol     int

  // A short description for the mapping. This may not be used in all cases.
  //
  // If it is left blank, a name will be generated automatically.
  Name         string

  // The internal port on this host to map to.
  //
  // Mapping to ports on other hosts is not supported.
  InternalPort uint16

  // The external port to be used. When passing MappingConfig to
  // CreatePortMapping, this is purely advisory. If you want portmap to choose
  // a port itself, set this to zero.
  //
  // In order to determine the external port actually allocated, call
  // GetConfig() on a Mapping object after it becomes active.
  // The allocated port number may be different to the originally specified
  // ExternalPort value, even if it was nonzero.
  ExternalPort uint16

  // The lifetime of the mapping in seconds. The mapping will automatically be
  // renewed halfway through a lifetime period, so this value determines how
  // long a mapping will stick around when the program exits, if the mapping is
  // not deleted beforehand.
  Lifetime     uint32 // seconds

  // Determines the backoff delays used between NAT-PMP or UPnP mapping
  // attempts. Note that if you set MaxTries to a nonzero value, the mapping
  // process will give up after that many tries.
  //
  // It is recommended that you use the nil value for this struct, which will
  // cause sensible defaults to be used with no limit on retries.
  RetryConfig  net.RetryConfig
}

type Mapping interface {
  // Gets the current MappingConfig for this mapping. Note that the
  // ExternalPort and Lifetime may both have changed from the values passed to
  // CreatePortMapping. If Name was blank, it will have been changed to a
  // generated value.
  //
  // It is possible for the ExternalPort to change several times, not just
  // initially, though ideally this will not happen, and even when it does
  // happen it should be very rare.
  GetConfig() MappingConfig

  // Returns true iff the mapping has been successfully created and is active.
  IsActive() bool

  // Returns true iff the last attempt to create a mapping failed. This may
  // only be a temporary condition, as some mapping attempts may fail before a
  // successful mapping attempt occurs.
  HasFailed() bool

  // Wait until the mapping may have become active. IsActive() is not
  // guaranteed to be true after calling this; generally you will want to use
  // this in a loop which keeps calling it until IsActive() is true.
  WaitActive()

  // Wait until the mapping may have been successfully torn down. IsActive() is
  // not guaranteed to be false after calling this; generally you will want to
  // use this in a loop which keeps calling it until IsActive() is false.
  //
  // There is no point calling this until you have called Delete().
  WaitInactive()

  // Deletes the mapping. Doesn't block until the mapping is destroyed.
  // If you do not call this and WaitInactive() in a loop until IsActive() is
  // false, then the mapping may still exist when the program exits.
  Delete()
}

type mapping struct {
  config MappingConfig
  failed bool
  expireTime time.Time
  abortChan chan struct{}
  waitChan chan struct{}
}

func (self *mapping) GetConfig() MappingConfig {
  return self.config
}

func (self *mapping) IsActive() bool {
  return !self.expireTime.IsZero() && self.expireTime.After(time.Now())
}

func (self *mapping) HasFailed() bool {
  return self.failed
}

type empty struct {}

func (self *mapping) Delete() {
  select {
    case self.abortChan <- empty{}:
    default:
  }
}

func (self *mapping) WaitActive() {
  if self.IsActive() {
    return
  }
  <-self.waitChan
}

func (self *mapping) WaitInactive() {
  if !self.IsActive() {
    return
  }
  <-self.waitChan
}

const upnpWANIPConnectionURN = "urn:schemas-upnp-org:service:WANIPConnection:1"
const modeNATPMP = 0
const modeUPnP   = 1

func (self *mapping) trySignalWait() {
  select {
    case self.waitChan<-empty{}:
    default:
  }
}

func (self *mapping) tryNATPMPGW(gw gnet.IP, destroy bool) bool {
  var externalPort uint16
  var actualLifetime uint32
  var err error

  preferredLifetime := self.config.Lifetime
  if destroy {
    if !self.IsActive() {
      return true
    }
    preferredLifetime = 0
  }

  log.Info("Attempting to map port")
  externalPort, actualLifetime, err = natpmpMap(gw,
    self.config.Protocol, self.config.InternalPort, self.config.ExternalPort, preferredLifetime)

  if err != nil {
    log.Info(fmt.Sprintf("Port mapping failed: %+v", err))
    return false
  }

  log.Info("Mapping successful")
  self.config.ExternalPort = externalPort
  self.config.Lifetime = actualLifetime
  if preferredLifetime == 0 {
    // we have finished tearing down the mapping by mapping it with a
    // lifetime of zero, so return
    //self.failed = true
    return true
  }

  //self.failed = false
  self.expireTime = time.Now().Add(time.Duration(actualLifetime)*time.Second)

  return true
}

func (self *mapping) tryNATPMP(gwa []gnet.IP, destroy bool) bool {
  for _, gw := range gwa {
    if self.tryNATPMPGW(gw, destroy) {
      return true
    }
  }
  return false
}

func (self *mapping) tryUPnPSvc(svc ssdpreg.SSDPService, destroy bool) bool {
  log.Info("trying to map port via UPnP")

  preferredLifetime := self.config.Lifetime
  if destroy {
    if !self.IsActive() {
      return true
    }
    // ...
    err := upnp.UnmapPort(svc, self.config.Protocol, self.config.ExternalPort)
    if err != nil {
      return false
    }
    return true
  }

  actualExternalPort, err := upnp.MapPort(svc, self.config.Protocol,
    self.config.InternalPort,
    self.config.ExternalPort, self.config.Name, preferredLifetime)

  if err != nil {
    return false
  }

  preLifetime := preferredLifetime/2
  self.expireTime = time.Now().Add(time.Duration(preLifetime)*time.Second)
  self.config.ExternalPort = actualExternalPort
  return true
}

func (self *mapping) tryUPnP(svcs []ssdpreg.SSDPService, destroy bool) bool {
  for _, svc := range svcs {
    if self.tryUPnPSvc(svc, destroy) {
      return true
    }
  }
  return false
}

func (self *mapping) portMappingLoop(gwa []gnet.IP) {
  aborting := false
  mode := modeNATPMP
  var ok bool
  var d time.Duration
  for {
    if aborting && !self.IsActive() {
      return
    }

    switch mode {
      case modeNATPMP:
        ok = self.tryNATPMP(gwa, aborting)
        if ok {
          preLifetime := self.config.Lifetime/2
          d = time.Duration(preLifetime)*time.Second
        } else {
          svc := ssdpreg.GetServicesByType(upnpWANIPConnectionURN)
          if len(svc) > 0 {
            // NAT-PMP failed and UPnP is available, so switch to it
            log.Info("switching to UPnP")
            mode = modeUPnP
            continue
          }
        }

      case modeUPnP:
        svcs := ssdpreg.GetServicesByType(upnpWANIPConnectionURN)
        if len(svcs) == 0 {
          log.Info("switching to NAT-PMP")
          mode = modeNATPMP
          continue
        }

        ok = self.tryUPnP(svcs, aborting)
        d  = time.Duration(1)*time.Hour
    }

    if aborting {
      if self.IsActive() {
        if ok {
          self.expireTime = time.Time{}
        } else {
          time.Sleep(self.expireTime.Sub(time.Now()))
        }
      }
      self.trySignalWait()
      return
    }

    self.failed = !ok
    if ok {
      log.Info("fwneg succeeded")
      self.trySignalWait()
      self.config.RetryConfig.Reset()
      select {
        case <-self.abortChan:
          aborting = true

        case <-time.After(d):
      }
    } else {
      // failed, do retry delay
      d := self.config.RetryConfig.GetStepDelay()
      if d == 0 {
        // max tries occurred
        return
      }

      ta := time.After(time.Duration(d)*time.Millisecond)
      self.trySignalWait()
      select {
        case <-self.abortChan:
          aborting = true

        case <-ta:
      }
    }
  }
}

// Creates a port mapping. The mapping process is continually attempted and
// maintained in the background, but the Mapping interface is returned
// immediately without blocking.
//
// The mapping will not be active when this call returns; instead, interrogate
// the returned Mapping interface.
//
// A successful mapping is by no means guaranteed.
//
// See the MappingConfig struct and the Mapping interface for more information.
func CreatePortMapping(config MappingConfig) (m Mapping, err error) {
  gwa, err := net.GetGatewayAddrs()
  if err != nil {
    return
  }

  if config.Lifetime == 0 {
    config.Lifetime = 60*60*2
  }

  mm := &mapping {
    config: config,
    abortChan: make(chan struct{}, 1),
    waitChan: make(chan struct{}, 1),
  }

  ssdpreg.Start()

  go mm.portMappingLoop(gwa)
  m = mm
  return
}
