// You should probably use package portmap instead of this package,
// which is intended for internal use only.
package upnp

import "github.com/hlandau/degoutils/net/ssdpreg"
import "net/http"
import "net/url"
import gnet "net"
import "encoding/xml"
import "errors"
import "github.com/hlandau/degoutils/log"
import "fmt"
import "math/rand"
import "strings"

const upnpDeviceNS = "urn:schemas-upnp-org:device-1-0"

type xRootDevice struct {
  XMLName xml.Name `xml:"root"`
  Device  xDevice  `xml:"device"`
}

type xDevice struct {
  Services []xService  `xml:"serviceList>service,omitempty"`
  Devices  []xDevice   `xml:"deviceList>device,omitempty"`
}

func (self *xDevice) InitURLFields(base *url.URL) {
  for i := range self.Services {
    self.Services[i].InitURLFields(base)
  }
  for i := range self.Devices {
    self.Devices[i].InitURLFields(base)
  }
}

type servicesVisitorFunc func(s *xService)

func (self *xDevice) VisitServices(f servicesVisitorFunc) {
  for i := range self.Services {
    f(&self.Services[i])
  }
  for i := range self.Devices {
    self.Devices[i].VisitServices(f)
  }
}

type xService struct {
  ServiceType   string    `xml:"serviceType"`
  ServiceID     string    `xml:"serviceId"`
  ControlURL    xURLField `xml:"controlURL"`
}

func (self *xService) InitURLFields(base *url.URL) {
  self.ControlURL.InitURLFields(base)
}

type xURLField struct {
  URL url.URL `xml:"-"`
  OK  bool    `xml:"-"`
  Str string  `xml:",chardata"`
}

func (self *xURLField) InitURLFields(base *url.URL) {
  u, err := url.Parse(self.Str)
  if err != nil {
    log.Info("error parsing URL: ", err)
    self.URL = url.URL{}
    self.OK = false
    return
  }

  self.URL = *base.ResolveReference(u)
  self.OK  = true
}


func getWANIPControlURL(svc ssdpreg.SSDPService) (wurl *url.URL, err error) {
  res, err := http.Get(svc.Location.String())
  if err != nil {
    return
  }

  if res.StatusCode != 200 {
    err = errors.New("non-200 status code when retrieving UPnP device description")
    return
  }

  defer res.Body.Close()
  d := xml.NewDecoder(res.Body)
  d.DefaultSpace = upnpDeviceNS

  var root xRootDevice
  err = d.Decode(&root)
  if err != nil {
    return
  }

  root.Device.InitURLFields(svc.Location)
  log.Info(fmt.Sprintf("xml: %+v", root))

  root.Device.VisitServices(func(s *xService) {
    if s.ServiceType != "urn:schemas-upnp-org:service:WANIPConnection:1" || wurl != nil || !s.ControlURL.OK {
      return
    }

    wurl = &s.ControlURL.URL
  })

  return
}

/*
POST /ctl/IPConn HTTP/1.1
Host: 192.168.1.1:5000
User-Agent: Linux/3.13.6-1-ARCH, UPnP/1.0, MiniUPnPc/1.9
Content-Length: 285
Content-Type: text/xml
SOAPAction: "urn:schemas-upnp-org:service:WANIPConnection:1#GetExternalIPAddress"
Connection: Close
Cache-Control: no-cache
Pragma: no-cache

<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:GetExternalIPAddress xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"></u:GetExternalIPAddress></s:Body></s:Envelope>
HTTP/1.1 200 OK
Content-Type: text/xml
Connection: close
Content-Length: 357
Server: OpenWRT/kamikaze UPnP/1.0 MiniUPnPd/1.4

<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:GetExternalIPAddressResponse xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"><NewExternalIPAddress>192.168.0.2</NewExternalIPAddress></u:GetExternalIPAddressResponse></s:Body></s:Envelope>
*/

/*
POST /ctl/IPConn HTTP/1.1
Host: 192.168.1.1:5000
User-Agent: Linux/3.13.6-1-ARCH, UPnP/1.0, MiniUPnPc/1.9
Content-Length: 598
Content-Type: text/xml
SOAPAction: "urn:schemas-upnp-org:service:WANIPConnection:1#AddPortMapping"
Connection: Close
Cache-Control: no-cache
Pragma: no-cache

<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><u:AddPortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"><NewRemoteHost></NewRemoteHost><NewExternalPort>1234</NewExternalPort><NewProtocol>TCP</NewProtocol><NewInternalPort>4321</NewInternalPort><NewInternalClient>192.168.1.123</NewInternalClient><NewEnabled>1</NewEnabled><NewPortMappingDescription>libminiupnpc</NewPortMappingDescription><NewLeaseDuration>8765</NewLeaseDuration></u:AddPortMapping></s:Body></s:Envelope>
HTTP/1.1 500 Internal Server Error
Content-Type: text/xml
Connection: close
Content-Length: 406
Server: OpenWRT/kamikaze UPnP/1.0 MiniUPnPd/1.4

<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body><s:Fault><faultcode>s:Client</faultcode><faultstring>UPnPError</faultstring><detail><UPnPError xmlns="urn:schemas-upnp-org:control-1-0"><errorCode>718</errorCode><errorDescription>ConflictInMappingEntry</errorDescription></UPnPError></detail></s:Fault></s:Body></s:Envelope>
*/

func soapRequest(url, method, msg string) (res *http.Response, err error) {
  fm := `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/"><s:Body>` + msg + `</s:Body></s:Envelope>`

  req, err := http.NewRequest("POST", url, strings.NewReader(fm))
  if err != nil {
    return
  }

  req.Header.Set("Content-Type", "text/xml; charset=\"utf-8\"")
  req.Header.Set("SOAPAction", "\"urn:schemas-upnp-org:service:WANIPConnection:1#" + method + "\"")

  res, err = http.DefaultClient.Do(req)
  if err != nil {
    return
  }

  if res.StatusCode != 200 {
    err = errors.New("Non-successful HTTP error code")
    res.Body.Close()
    res = nil
    return
  }

  return
}

func determineSelfIP(u *url.URL) (ip gnet.IP, err error) {
  c, err := gnet.Dial("udp", u.Host)
  if err != nil {
    return
  }

  defer c.Close()

  uaddr := c.LocalAddr().(*gnet.UDPAddr)
  ip = uaddr.IP

  return
}

const (
  protocolTCP =  6
  protocolUDP = 17
)

func protocolString(protocol int) string {
  if protocol == protocolTCP {
    return "TCP"
  } else if protocol == protocolUDP {
    return "UDP"
  } else {
    panic("???")
  }
}

func MapPort(svc ssdpreg.SSDPService, protocol int, internalPort uint16,
             externalPort uint16, name string, duration uint32) (actualExternalPort uint16, err error) {
  wurl, err := getWANIPControlURL(svc)

  if externalPort == 0 {
    externalPort = uint16(1025+rand.Int31n(64000))
  }

  selfIP, err := determineSelfIP(wurl)
  if err != nil {
    return
  }

  log.Info("WANIP Control URL: ", wurl.String())
  log.Info("Requesting External Port: ", externalPort)
  log.Info("Self IP: ", selfIP.String())

  protocolStr := protocolString(protocol)

  s := fmt.Sprintf(`<u:AddPortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"><NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>%s</NewProtocol><NewInternalPort>%d</NewInternalPort><NewInternalClient>%s</NewInternalClient><NewEnabled>1</NewEnabled><NewPortMappingDescription>%s</NewPortMappingDescription><NewLeaseDuration>%d</NewLeaseDuration></u:AddPortMapping>`, externalPort, protocolStr, internalPort, selfIP.String(), name, duration)

  res, err := soapRequest(wurl.String(), "AddPortMapping", s)
  if err != nil {
    return
  }
  defer res.Body.Close()
  // HTTP Status Code is non-200 if there was an error, so do we even need to check the body?

  log.Info("UPnP OK")

  actualExternalPort = externalPort
  return
}

func UnmapPort(svc ssdpreg.SSDPService, protocol int, externalPort uint16) (err error) {
  wurl, err := getWANIPControlURL(svc)

  protocolStr := protocolString(protocol)

  s := fmt.Sprintf(`<u:DeletePortMapping xmlns:u="urn:schemas-upnp-org:service:WANIPConnection:1"><NewRemoteHost></NewRemoteHost><NewExternalPort>%d</NewExternalPort><NewProtocol>%s</NewProtocol></u:DeletePortMapping></s:Body></s:Envelope>`,
    externalPort, protocolStr)

  res, err := soapRequest(wurl.String(), "DeletePortMapping", s)
  if err != nil {
    return
  }
  defer res.Body.Close()

  return
}
