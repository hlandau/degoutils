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

type Protocol int

const (
	ProtocolTCP Protocol = natpmpTCP // Map a TCP port
	ProtocolUDP Protocol = natpmpUDP // Map a UDP port
)

type Config struct {
	// The protocol for which the port should be mapped.
	//
	// Must be ProtocolTCP or ProtocolUDP.
	Protocol Protocol

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

func (m *mapping) NotifyChan() <-chan struct{} {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	return m.notifyChan
}

func (m *mapping) Delete() {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.aborted {
		return
	}

	close(m.abortChan)
	m.aborted = true
}

func (m *mapping) DeleteWait() {
	m.Delete()
	for {
		<-m.NotifyChan()
		if m.GetExternalAddr() == "" {
			break
		}
	}
}

func (m *mapping) GetExternalAddr() string {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if !m.isActive() || m.config.ExternalPort == 0 {
		return ""
	}

	return gnet.JoinHostPort(m.externalAddr, strconv.FormatUint(uint64(m.config.ExternalPort), 10))
}

func (m *mapping) isActive() bool {
	return !m.expireTime.IsZero() && m.expireTime.After(time.Now())
}

func (m *mapping) hasFailed() bool {
	return m.failed
}

const upnpWANIPConnectionURN = "urn:schemas-upnp-org:service:WANIPConnection:1"
const modeNATPMP = 0
const modeUPnP = 1

func (m *mapping) notify() {
	ea := m.GetExternalAddr()

	m.mutex.Lock()
	defer m.mutex.Unlock()

	if m.prevValue == ea {
		// no change
		return
	}

	m.prevValue = ea

	nc := m.notifyChan
	m.notifyChan = make(chan struct{})
	close(nc)
}

func (m *mapping) tryNATPMPGW(gw gnet.IP, destroy bool) bool {
	var externalPort uint16
	var actualLifetime uint32
	var err error

	m.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			m.mutex.Unlock()
		}
	}()

	preferredLifetime := uint32(m.config.Lifetime.Seconds())
	if destroy {
		if !m.isActive() {
			return true
		}
		preferredLifetime = 0
	}

	log.Info("Attempting to map port")

	m.mutex.Unlock()
	locked = false

	externalPort, actualLifetime, err = natpmpMap(gw,
		int(m.config.Protocol), m.config.InternalPort, m.config.ExternalPort, preferredLifetime)

	if err != nil {
		log.Info(fmt.Sprintf("Port mapping failed: %+v", err))
		return false
	}

	m.mutex.Lock()
	locked = true

	log.Info("Mapping successful")
	m.config.ExternalPort = externalPort
	m.config.Lifetime = time.Duration(actualLifetime) * time.Second
	if preferredLifetime == 0 {
		// we have finished tearing down the mapping by mapping it with a
		// lifetime of zero, so return
		//m.failed = true
		return true
	}

	//m.failed = false
	m.expireTime = time.Now().Add(time.Duration(actualLifetime) * time.Second)

	m.mutex.Unlock()
	locked = false

	// Now attempt to get the external IP.
	extIP, err := natpmpGetExternalAddr(gw)
	if err != nil {
		// mapping still succeeded
		return true
	}

	m.mutex.Lock()
	locked = true

	// update external address
	m.externalAddr = extIP.String()

	return true
}

func (m *mapping) tryNATPMP(gwa []gnet.IP, destroy bool) bool {
	for _, gw := range gwa {
		if m.tryNATPMPGW(gw, destroy) {
			return true
		}
	}
	return false
}

func (m *mapping) tryUPnPSvc(svc ssdpreg.Service, destroy bool) bool {
	log.Info("trying to map port via UPnP")

	m.mutex.Lock()
	locked := true
	defer func() {
		if locked {
			m.mutex.Unlock()
		}
	}()

	preferredLifetime := m.config.Lifetime
	if destroy {
		if !m.isActive() {
			return true
		}

		m.mutex.Unlock()
		locked = false

		err := upnp.UnmapPort(svc, int(m.config.Protocol), m.config.ExternalPort)

		if err != nil {
			return false
		}
		return true
	}

	m.mutex.Unlock()
	locked = false

	actualExternalPort, err := upnp.MapPort(svc, int(m.config.Protocol),
		m.config.InternalPort,
		m.config.ExternalPort, m.config.Name, preferredLifetime)

	if err != nil {
		return false
	}

	m.mutex.Lock()
	locked = true

	preLifetime := preferredLifetime / 2
	m.expireTime = time.Now().Add(time.Duration(preLifetime) * time.Second)
	m.config.ExternalPort = actualExternalPort

	// Now attempt to get the external IP.
	if destroy {
		return true
	}

	m.mutex.Unlock()
	locked = false

	extIP, err := upnp.GetExternalAddr(svc)
	if err != nil {
		// mapping till succeeded
		return true
	}

	m.mutex.Lock()
	locked = true

	// update external address
	m.externalAddr = extIP.String()
	log.Info("External address determined via UPnP: ", extIP)

	return true
}

func (m *mapping) tryUPnP(svcs []ssdpreg.Service, destroy bool) bool {
	for _, svc := range svcs {
		if m.tryUPnPSvc(svc, destroy) {
			return true
		}
	}
	return false
}

func (m *mapping) portMappingLoop(gwa []gnet.IP) {
	aborting := false
	mode := modeNATPMP
	var ok bool
	var d time.Duration
	for {
		m.mutex.Lock()
		isActive := m.isActive()
		m.mutex.Unlock()

		if aborting && !isActive {
			return
		}

		switch mode {
		case modeNATPMP:
			ok = m.tryNATPMP(gwa, aborting)
			if ok {
				d = m.config.Lifetime / 2
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

			ok = m.tryUPnP(svcs, aborting)
			d = time.Duration(1) * time.Hour
		}

		if aborting {
			m.mutex.Lock()
			isActive = m.isActive()
			m.mutex.Unlock()
			if isActive {
				if ok {
					m.mutex.Lock()
					m.expireTime = time.Time{}
					m.mutex.Unlock()
				} else {
					//m.mutex.Lock()
					//et := m.expireTime
					//m.mutex.Unlock()
					//time.Sleep(et.Sub(time.Now()))
				}
			}
			if !isActive || ok {
				m.notify()
				return
			}
		}

		m.mutex.Lock()
		m.failed = !ok
		m.mutex.Unlock()

		if ok {
			log.Info("fwneg succeeded")
			m.notify()
			m.config.Backoff.Reset()
			select {
			case <-m.abortChan:
				aborting = true

			case <-time.After(d):
			}
		} else {
			// failed, do retry delay
			d := m.config.Backoff.NextDelay()
			if d == 0 {
				// max tries occurred
				if aborting {
					// if aborting, force !active and notify when we give up
					m.mutex.Lock()
					m.expireTime = time.Time{}
					m.mutex.Unlock()
					m.notify()
				}
				return
			}

			ta := time.After(d)
			m.notify()
			select {
			case <-m.abortChan:
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

	if config.Lifetime == 0 {
		config.Lifetime = 2 * time.Hour
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
