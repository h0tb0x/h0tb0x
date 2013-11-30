package nat

// upnp.go from filegive-0.7.1
//
// Just enough UPnP to be able to forward ports
//
// Copyright jackpal, BSD license, file developed based on Taipei-Torrent's.

// Modifications from jackpal version by Lluís Batlle i Rossell

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type upnpNAT struct {
	serviceURL string
	service    string
	ourIP      string
}

func UpnpDiscover() (nat NAT, err error) {
	ssdp, err := net.ResolveUDPAddr("udp4", "239.255.255.250:1900")
	if err != nil {
		return
	}
	conn, err := net.ListenPacket("udp4", ":0")
	if err != nil {
		return
	}
	socket := conn.(*net.UDPConn)
	defer socket.Close()

	err = socket.SetDeadline(time.Now().Add(3 * time.Second))
	if err != nil {
		return
	}

	stvalue := "urn:schemas-upnp-org:device:InternetGatewayDevice:1"
	st := "ST: " + stvalue + "\r\n"
	buf := bytes.NewBufferString(
		"M-SEARCH * HTTP/1.1\r\n" +
			"HOST: 239.255.255.250:1900\r\n" +
			st +
			"MAN: \"ssdp:discover\"\r\n" +
			"MX: 2\r\n\r\n")
	message := buf.Bytes()
	answerBytes := make([]byte, 1024)
	for i := 0; i < 3; i++ {
		_, err = socket.WriteToUDP(message, ssdp)
		if err != nil {
			return
		}
		var n int
		n, _, err = socket.ReadFromUDP(answerBytes)
		if err != nil {
			continue
			// socket.Close()
			// return
		}
		answer := string(answerBytes[0:n])
		if strings.Index(answer, stvalue+"\r\n") < 0 {
			continue
		}
		// HTTP header field names are case-insensitive.
		// http://www.w3.org/Protocols/rfc2616/rfc2616-sec4.html#sec4.2
		locString := "\r\nlocation: "
		answerlow := strings.ToLower(answer)
		locIndex := strings.Index(answerlow, locString)
		if locIndex < 0 {
			continue
		}
		loc := answer[locIndex+len(locString):]
		endIndex := strings.Index(loc, "\r\n")
		if endIndex < 0 {
			continue
		}
		locURLString := loc[0:endIndex]
		var serviceURL string
		var service string
		serviceURL, service, err = getServiceURL(locURLString)
		if err != nil {
			return
		}

		// We need the IP that matches that of the locURL IP
		var locURL *url.URL
		locURL, err = url.Parse(locURLString)
		if err != nil {
			return
		}

		// Find ':' for port in the locURL host
		var locHost string
		locHost, _, err = net.SplitHostPort(locURL.Host)
		if err != nil {
			locHost = locURL.Host
		}

		var ourIP string
		ourIP, err = GetOurIP(net.ParseIP(locHost))
		if err != nil {
			return
		}
		nat = &upnpNAT{serviceURL: serviceURL, service: service, ourIP: ourIP}
		return
	}
	err = errors.New("UPnP port discovery failed.")
	return
}

type Service struct {
	ServiceType string `xml:"serviceType"`
	ControlURL  string `xml:"controlURL"`
}

type DeviceList struct {
	Device []Device `xml:"device"`
}

type ServiceList struct {
	Service []Service `xml:"service"`
}

type Device struct {
	DeviceType  string      `xml:"deviceType"`
	DeviceList  DeviceList  `xml:"deviceList"`
	ServiceList ServiceList `xml:"serviceList"`
}

type Root struct {
	Device Device `xml:"device"`
}

func getChildDevice(d *Device, deviceType string) *Device {
	dl := d.DeviceList.Device
	for i := 0; i < len(dl); i++ {
		if dl[i].DeviceType == deviceType {
			return &dl[i]
		}
	}
	return nil
}

func getChildService(d *Device, serviceType string) *Service {
	sl := d.ServiceList.Service
	for i := 0; i < len(sl); i++ {
		if sl[i].ServiceType == serviceType {
			return &sl[i]
		}
	}
	return nil
}

func GetOurIP(destIP net.IP) (ip string, err error) {
	var ifs []net.Interface
	ifs, err = net.Interfaces()
	if err != nil {
		return
	}

	for i := range ifs {
		var addrs []net.Addr
		addrs, err = ifs[i].Addrs()
		if err == nil {
			for j := range addrs {
				var parsed net.IP
				var ipnet *net.IPNet

				// Linux adds "/8" or "/24" to interfaces, windows not.
				ipString := addrs[j].String()
				if strings.Contains(ipString, "/") {
					parsed, ipnet, err = net.ParseCIDR(ipString)
				} else {
					parsed = net.ParseIP(ipString)
				}

				// Handle 0.0.0.0 addresses windows gives in some interfaces
				if parsed == nil || parsed.IsUnspecified() {
					continue
				}
				ip4 := parsed.To4()
				if ip4 == nil {
					continue
				}

				if !ip4.IsLoopback() {
					if destIP != nil && ipnet != nil && !ipnet.Contains(destIP) {
						continue
					}
					ip = ip4.String()
					return
				}
			}
		}
	}
	err = errors.New("Cannot find the local IP address")
	return
}

func getServiceURL(rootURL string) (url, service string, err error) {
	r, err := http.Get(rootURL)
	if err != nil {
		return
	}
	defer r.Body.Close()
	if r.StatusCode >= 400 {
		err = errors.New(string(r.StatusCode))
		return
	}

	var root Root
	err = xml.NewDecoder(r.Body).Decode(&root)
	if err != nil {
		return
	}
	a := &root.Device
	if a.DeviceType != "urn:schemas-upnp-org:device:InternetGatewayDevice:1" {
		err = errors.New("No InternetGatewayDevice")
		return
	}
	b := getChildDevice(a, "urn:schemas-upnp-org:device:WANDevice:1")
	if b == nil {
		err = errors.New("No WANDevice")
		return
	}
	c := getChildDevice(b, "urn:schemas-upnp-org:device:WANConnectionDevice:1")
	if c == nil {
		err = errors.New("No WANConnectionDevice")
		return
	}
	d := getChildService(c, "urn:schemas-upnp-org:service:WANIPConnection:1")
	if d != nil {
		url = combineURL(rootURL, d.ControlURL)
		service = "WANIPConnection"
		return
	}
	d = getChildService(c, "urn:schemas-upnp-org:service:WANPPPConnection:1")
	if d != nil {
		url = combineURL(rootURL, d.ControlURL)
		service = "WANPPPConnection"
		return
	}
	err = errors.New("No WANIPConnection or WANPPPConnection")
	return
}

func combineURL(rootURL, subURL string) string {
	protocolEnd := "://"
	protoEndIndex := strings.Index(rootURL, protocolEnd)
	a := rootURL[protoEndIndex+len(protocolEnd):]
	rootIndex := strings.Index(a, "/")
	return rootURL[0:protoEndIndex+len(protocolEnd)+rootIndex] + subURL
}

func (this *upnpNAT) soapRequest(function, message string) (*http.Response, error) {
	const format = `
<?xml version="1.0"?>
<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">
	<s:Body>%s</s:Body>
</s:Envelope>`

	soap := fmt.Sprintf(format, message)

	req, err := http.NewRequest("POST", this.serviceURL, strings.NewReader(soap))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", `text/xml ; charset="utf-8"`)
	req.Header.Set("User-Agent", "Darwin/10.0.0, UPnP/1.0, MiniUPnPc/1.3")
	// req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("SOAPAction", fmt.Sprintf(`"urn:schemas-upnp-org:service:%s:1#%s"`, this.service, function))
	req.Header.Set("Connection", "Close")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return resp, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		// log.Stderr(function, r.StatusCode)
		return nil, fmt.Errorf("soapRequest error - StatusCode %d for %s()", resp.StatusCode, function)
	}
	return resp, nil
}

type externalIP struct {
	ExternalIpAddress string `xml:"Body>GetExternalIPAddressResponse>NewExternalIPAddress"`
}

func (this *upnpNAT) getExternalIPAddress() (*externalIP, error) {
	const format = `
<u:GetExternalIPAddress xmlns:u="urn:schemas-upnp-org:service:%s:1"></u:GetExternalIPAddress>
`
	msg := fmt.Sprintf(format, this.service)
	resp, err := this.soapRequest("GetExternalIPAddress", msg)
	if err != nil {
		return nil, err
	}

	var addr externalIP
	err = xml.NewDecoder(resp.Body).Decode(&addr)
	if err != nil {
		return nil, err
	}

	return &addr, nil
}

func (this *upnpNAT) GetExternalAddress() (net.IP, error) {
	extip, err := this.getExternalIPAddress()
	if err != nil {
		return nil, err
	}
	addr := net.ParseIP(extip.ExternalIpAddress)
	return addr, nil
}

func (this *upnpNAT) AddPortMapping(
	proto string,
	internalPort, externalPort uint16,
	description string,
	timeout int) (uint16, error) {
	const format = `
<u:AddPortMapping xmlns:u="urn:schemas-upnp-org:service:%s:1">
	<NewRemoteHost></NewRemoteHost>
	<NewExternalPort>%d</NewExternalPort>
	<NewProtocol>%s</NewProtocol>
	<NewInternalPort>%d</NewInternalPort>
	<NewInternalClient>%s</NewInternalClient>
	<NewEnabled>1</NewEnabled>
	<NewPortMappingDescription>%s</NewPortMappingDescription>
	<NewLeaseDuration>%d</NewLeaseDuration>
</u:AddPortMapping>`

	msg := fmt.Sprintf(format,
		this.service, externalPort, proto, internalPort, this.ourIP, description, timeout)
	_, err := this.soapRequest("AddPortMapping", msg)
	if err != nil {
		log.Println("soapRequest returned error:", err)
		return 0, err
	}

	// TODO: check response to see if the port was forwarded
	// log.Println(message, response)
	return externalPort, nil
}

func (this *upnpNAT) DeletePortMapping(proto string, externalPort, internalPort int) error {
	const format = `
<u:DeletePortMapping xmlns:u="urn:schemas-upnp-org:service:%s:1\">
	<NewRemoteHost></NewRemoteHost>
	<NewExternalPort>%d</NewExternalPort>
	<NewProtocol>%s</NewProtocol>
</u:DeletePortMapping>`

	msg := fmt.Sprintf(format, this.service, externalPort, proto)

	_, err := this.soapRequest("DeletePortMapping", msg)
	// TODO: check response to see if the port was deleted
	// log.Println(message, response)
	return err
}
