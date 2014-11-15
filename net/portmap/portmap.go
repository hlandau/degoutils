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
import "time"
import "fmt"
import "sync"
import "strconv"
import "github.com/hlandau/degoutils/net"
import "github.com/hlandau/degoutils/log"
import "github.com/hlandau/degoutils/net/ssdpreg"
import "github.com/hlandau/degoutils/net/portmap/upnp"

const ProtocolTCP = natpmpTCP // Map a TCP port.
const ProtocolUDP = natpmpUDP // Map a UDP port.

type Config struct {
	// The protocol for which the port should be mapped.
	//
	// Must be ProtocolTCP or ProtocolUDP.
	Protocol int

	// A short description for the mapping. This may not be used in all cases.
	//
	// If it is left blank, a name will be generated automatically.
	Name string

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
	Lifetime time.Duration // seconds

	// Determines the backoff delays used between NAT-PMP or UPnP mapping
	// attempts. Note that if you set MaxTries to a nonzero value, the mapping
	// process will give up after that many tries.
	//
	// It is recommended that you use the nil value for this struct, which will
	// cause sensible defaults to be used with no limit on retries.
	Backoff net.Backoff
}

// A mapping has a state:
//
//   |            |-- [Started] -->|          |
//   |  Inactive  |                |  Active  |-- [Changed] --\
//   |            |<-- [Stopped] --|          |<--------------/
//
// The external IP address and port number of a mapping may change whenever
// the Started or Changed transition occurs. When the Stopped transition occurs
// the mapping is no longer active.

type Mapping interface {
	// Returns a channel which will be closed when a state transition occurs.
	// Subsequent calls to WaitChan() will return a new channel; do not store
	// the channel.
	NotifyChan() <-chan struct{}

	// Deletes the mapping. Doesn't block until the mapping is destroyed.
	Delete()

	// Deletes the mapping. Blocks until the mapping is destroyed.
	DeleteWait()

	// Returns the external address in "IP:port" format.
	// If the mapping is not active, returns an empty string.
	// The IP address may not be globally routable, for example in double-NAT cases.
	// If the external port has been mapped but the external IP cannot be determined,
	// returns ":port".
	GetExternalAddr() string

	// Gets the current MappingConfig for this mapping. Note that the
	// ExternalPort and Lifetime may both have changed from the values passed to
	// CreatePortMapping. If Name was blank, it will have been changed to a
	// generated value.
	//
	// It is possible for the ExternalPort to change several times, not just
	// initially, though ideally this will not happen, and even when it does
	// happen it should be very rare.
	//GetConfig() MappingConfig

	// Returns true iff the mapping has been successfully created and is active.
	//IsActive() bool

	// Returns true iff the last attempt to create a mapping failed. This may
	// only be a temporary condition, as some mapping attempts may fail before a
	// successful mapping attempt occurs.
	//HasFailed() bool
}

type mapping struct {
	mutex sync.Mutex

	// m: Protected by mutex

	config Config // m(ExternalPort)

	failed     bool      // m
	expireTime time.Time // m

	aborted   bool          // m
	abortChan chan struct{} // m

	notifyChan chan struct{} // m

	externalAddr string // m
	prevValue    string
}

func (self *mapping) NotifyChan() <-chan struct{} {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	return self.notifyChan
}

func (self *mapping) Delete() {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	if self.aborted {
		return
	}

	close(self.abortChan)
	self.aborted = true
}

func (self *mapping) DeleteWait() {
	self.Delete()
	for {
		<-self.NotifyChan()
		if self.GetExternalAddr() == "" {
			break
		}
	}
}

func (self *mapping) GetExternalAddr() string {
	self.mutex.Lock()
	defer self.mutex.Unlock()

	if !self.isActive() || self.config.ExternalPort == 0 {
		return ""
	}

	return gnet.JoinHostPort(self.externalAddr, strconv.FormatUint(uint64(self.config.ExternalPort), 10))
}

//func (self *mapping) GetConfig() MappingConfig {
//  return self.config
//}

func (self *mapping) isActive() bool {
	return !self.expireTime.IsZero() && self.expireTime.After(time.Now())
}

func (self *mapping) hasFailed() bool {
	return self.failed
}

const upnpWANIPConnectionURN = "urn:schemas-upnp-org:service:WANIPConnection:1"
const modeNATPMP = 0
const modeUPnP = 1

func (self *mapping) notify() {
	ea := self.GetExternalAddr()

	self.mutex.Lock()
	defer self.mutex.Unlock()

	if self.prevValue == ea {
		// no change
		return
	}

	self.prevValue = ea

	nc := self.notifyChan
	self.notifyChan = make(chan struct{})
	close(nc)
}

func (self *mapping) tryNATPMPGW(gw gnet.IP, destroy bool) bool {
	var externalPort uint16
	var actualLifetime uint32
	var err error

	self.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			self.mutex.Unlock()
		}
	}()

	preferredLifetime := uint32(self.config.Lifetime.Seconds())
	if destroy {
		if !self.isActive() {
			return true
		}
		preferredLifetime = 0
	}

	log.Info("Attempting to map port")

	self.mutex.Unlock()
	locked = false

	externalPort, actualLifetime, err = natpmpMap(gw,
		self.config.Protocol, self.config.InternalPort, self.config.ExternalPort, preferredLifetime)

	if err != nil {
		log.Info(fmt.Sprintf("Port mapping failed: %+v", err))
		return false
	}

	self.mutex.Lock()
	locked = true

	log.Info("Mapping successful")
	self.config.ExternalPort = externalPort
	self.config.Lifetime = time.Duration(actualLifetime) * time.Second
	if preferredLifetime == 0 {
		// we have finished tearing down the mapping by mapping it with a
		// lifetime of zero, so return
		//self.failed = true
		return true
	}

	//self.failed = false
	self.expireTime = time.Now().Add(time.Duration(actualLifetime) * time.Second)

	self.mutex.Unlock()
	locked = false

	// Now attempt to get the external IP.
	extIP, err := natpmpGetExternalAddr(gw)
	if err != nil {
		// mapping still succeeded
		return true
	}

	self.mutex.Lock()
	locked = true

	// update external address
	self.externalAddr = extIP.String()

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

	self.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			self.mutex.Unlock()
		}
	}()

	preferredLifetime := self.config.Lifetime
	if destroy {
		if !self.isActive() {
			return true
		}

		self.mutex.Unlock()
		locked = false

		err := upnp.UnmapPort(svc, self.config.Protocol, self.config.ExternalPort)

		if err != nil {
			return false
		}
		return true
	}

	self.mutex.Unlock()
	locked = false

	actualExternalPort, err := upnp.MapPort(svc, self.config.Protocol,
		self.config.InternalPort,
		self.config.ExternalPort, self.config.Name, preferredLifetime)

	if err != nil {
		return false
	}

	self.mutex.Lock()
	locked = true

	preLifetime := preferredLifetime / 2
	self.expireTime = time.Now().Add(time.Duration(preLifetime) * time.Second)
	self.config.ExternalPort = actualExternalPort

	// Now attempt to get the external IP.
	if destroy {
		return true
	}

	self.mutex.Unlock()
	locked = false

	extIP, err := upnp.GetExternalAddr(svc)
	if err != nil {
		// mapping till succeeded
		return true
	}

	self.mutex.Lock()
	locked = true

	// update external address
	self.externalAddr = extIP.String()
	log.Info("External address determined via UPnP: ", extIP)

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
		self.mutex.Lock()
		isActive := self.isActive()
		self.mutex.Unlock()

		if aborting && !isActive {
			return
		}

		switch mode {
		case modeNATPMP:
			ok = self.tryNATPMP(gwa, aborting)
			if ok {
				d = self.config.Lifetime / 2
			} else {
				svc := ssdpreg.GetServicesByType(upnpWANIPConnectionURN)
				if len(svc) > 0 {
					// NAT-PMP failed and UPnP is available, so switch to it
					log.Info("switching to UPnP")
					mode = modeUPnP
					continue
				} else {
					log.Info("no UPnP services")
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
			d = time.Duration(1) * time.Hour
		}

		if aborting {
			self.mutex.Lock()
			isActive = self.isActive()
			self.mutex.Unlock()
			if isActive {
				if ok {
					self.mutex.Lock()
					self.expireTime = time.Time{}
					self.mutex.Unlock()
				} else {
					//self.mutex.Lock()
					//et := self.expireTime
					//self.mutex.Unlock()
					//time.Sleep(et.Sub(time.Now()))
				}
			}
			if !isActive || ok {
				self.notify()
				return
			}
		}

		self.mutex.Lock()
		self.failed = !ok
		self.mutex.Unlock()

		if ok {
			log.Info("fwneg succeeded")
			self.notify()
			self.config.Backoff.Reset()
			select {
			case <-self.abortChan:
				aborting = true

			case <-time.After(d):
			}
		} else {
			// failed, do retry delay
			d := self.config.Backoff.NextDelay()
			if d == 0 {
				// max tries occurred
				if aborting {
					// if aborting, force !active and notify when we give up
					self.mutex.Lock()
					self.expireTime = time.Time{}
					self.mutex.Unlock()
					self.notify()
				}
				return
			}

			ta := time.After(d)
			self.notify()
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
func New(config Config) (m Mapping, err error) {
	gwa, err := net.GetGatewayAddrs()
	if err != nil {
		return
	}

	if config.Lifetime == time.Duration(0) {
		config.Lifetime = time.Duration(2) * time.Hour
	}

	mm := &mapping{
		config:     config,
		abortChan:  make(chan struct{}),
		notifyChan: make(chan struct{}),
	}

	ssdpreg.Start()

	go mm.portMappingLoop(gwa)
	m = mm
	return
}
