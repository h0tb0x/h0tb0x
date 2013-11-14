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
	"log"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type upnpNAT struct {
	serviceURL string
	service    string
	ourIP      string
}

func Discover() (nat NAT, err error) {
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

func soapRequest(url, function, message string) (r *http.Response, err error) {
	//fullMessage := "<?xml version=\"1.0\" encoding=\"utf-8\"?>" +
	//	"<SOAP-ENV:Envelope xmlns:SOAP-ENV=\"http://schemas.xmlsoap.org/soap/envelope/\" SOAP-ENV:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\">" +
	//	"<SOAP-ENV:Body>" + message + "</SOAP-ENV:Body></SOAP-ENV:Envelope>"

	fullMessage := "<?xml version=\"1.0\"?>" +
		"<s:Envelope xmlns:s=\"http://schemas.xmlsoap.org/soap/envelope/\" s:encodingStyle=\"http://schemas.xmlsoap.org/soap/encoding/\">\r\n" +
		"<s:Body>" + message + "</s:Body></s:Envelope>"

	req, err := http.NewRequest("POST", url, strings.NewReader(fullMessage))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml ; charset=\"utf-8\"")
	req.Header.Set("User-Agent", "Darwin/10.0.0, UPnP/1.0, MiniUPnPc/1.3")
	// req.Header.Set("Transfer-Encoding", "chunked")
	req.Header.Set("SOAPAction", "\"urn:schemas-upnp-org:service:WANIPConnection:1#"+function+"\"")
	// req.Header.Set("SOAPAction", "\"urn:schemas-upnp-org:service:WANPPPConnection:1#"+function+"\"")
	req.Header.Set("Connection", "Close")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Pragma", "no-cache")

	// log.Stderr("soapRequest ", req)
	// log.Println("soapRequest URL:  ", url)
	// log.Println("soapRequest Message:  ", fullMessage)

	r, err = http.DefaultClient.Do(req)
	//	if r.Body != nil {
	//		defer r.Body.Close()
	//	}

	if r.StatusCode >= 400 {
		// log.Stderr(function, r.StatusCode)
		err = errors.New("soapRequest error - StatusCode " + strconv.Itoa(r.StatusCode) + " for " + function + "()")
		//		err = errors.New("soapRequest error - StatusString " + r.Status)
		r = nil
		return
	} // else {
	//		log.Println("soapRequest successful, StatusCode:  ", strconv.Itoa(r.StatusCode))
	//	}
	return
}

type externalIP struct {
	ExternalIpAddress string `xml:"Body>GetExternalIPAddressResponse>NewExternalIPAddress"`
}

func (n *upnpNAT) getExternalIPAddress() (extip externalIP, err error) {

	message := "<u:GetExternalIPAddress xmlns:u=\"urn:schemas-upnp-org:service:" +
		n.service + ":1\">\r\n" + "</u:GetExternalIPAddress>"

	var response *http.Response
	response, err = soapRequest(n.serviceURL, "GetExternalIPAddress", message)
	if err != nil {
		return
	}

	err = xml.NewDecoder(response.Body).Decode(&extip)
	if err != nil {
		return
	}

	response.Body.Close()
	return
}

func (n *upnpNAT) GetExternalAddress() (addr net.IP, err error) {
	extip, err := n.getExternalIPAddress()
	if err != nil {
		return
	}
	addr = net.ParseIP(extip.ExternalIpAddress)
	return
}

func (n *upnpNAT) AddPortMapping(protocol string, externalPort, internalPort int, description string, timeout int) (mappedExternalPort int, err error) {
	// A single concatenation would break ARM compilation.
	message := "<u:AddPortMapping xmlns:u=\"urn:schemas-upnp-org:service:" +
		n.service + ":1\">\r\n" + "<NewRemoteHost></NewRemoteHost><NewExternalPort>" +
		strconv.Itoa(externalPort)
	message += "</NewExternalPort><NewProtocol>" + protocol + "</NewProtocol>"
	message += "<NewInternalPort>" + strconv.Itoa(internalPort) + "</NewInternalPort>" +
		"<NewInternalClient>" + n.ourIP + "</NewInternalClient>" +
		"<NewEnabled>1</NewEnabled><NewPortMappingDescription>"
	message += description +
		"</NewPortMappingDescription><NewLeaseDuration>" + strconv.Itoa(timeout) +
		"</NewLeaseDuration></u:AddPortMapping>"

	var response *http.Response
	response, err = soapRequest(n.serviceURL, "AddPortMapping", message)
	if err != nil {
		log.Println("soapRequest returned error:  ", err)
		return
	}

	// TODO: check response to see if the port was forwarded
	// log.Println(message, response)
	mappedExternalPort = externalPort
	_ = response
	return
}

func (n *upnpNAT) DeletePortMapping(protocol string, externalPort, internalPort int) (err error) {

	message := "<u:DeletePortMapping xmlns:u=\"urn:schemas-upnp-org:service:" +
		n.service + ":1\">\r\n" +
		"<NewRemoteHost></NewRemoteHost><NewExternalPort>" + strconv.Itoa(externalPort) +
		"</NewExternalPort><NewProtocol>" + protocol + "</NewProtocol>" +
		"</u:DeletePortMapping>"

	var response *http.Response
	response, err = soapRequest(n.serviceURL, "DeletePortMapping", message)
	if err != nil {
		return
	}

	// TODO: check response to see if the port was deleted
	// log.Println(message, response)
	_ = response
	return
}
