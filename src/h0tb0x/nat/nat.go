package nat

// NAT Developed by Jackpal - https://github.com/jackpal/Taipei-Torrent
// Commit dd88a8bfac6431c01d959ce3c745e74b8a911793
// Tue Jan 1 2013

import (
	"flag"
	"fmt"
	"log"
	"net"
)

const (
	ProtoTCP = "TCP"
	ProtoUDP = "UDP"
)

const (
	// TODO:  Currently UPnP overrides this value, sets to 0 - determine why Netgear routers using UPnP return status 500 if this value is > 0
	PortMapLifetime = 7200 // 2 hours
)

// Flag variables
var (
	onlyUpnp   bool
	onlyNatPmp bool
)

type NAT interface {
	GetExternalAddress() (net.IP, error)
	AddPortMapping(
		protocol string,
		internalPort,
		externalPort uint16,
		description string,
		timeout int) (uint16, error)
	DeletePortMapping(protocol string, externalPort, internalPort int) error
}

func init() {
	flag.BoolVar(&onlyUpnp, "only-upnp", false, "Only use UPnP to set up router port forwarding (skip NAT-PMP detection)")
	flag.BoolVar(&onlyNatPmp, "only-nat-pmp", false, "Only use NAT-PMP to set up router port forwarding (skip UPnP detection)")
}

func tryNatPmp(gateway net.IP) (NAT, net.IP) {
	log.Printf("Attempting to use NAT-PMP to set up port forwarding.")
	nat := NewNatPMP(gateway)
	external, err := nat.GetExternalAddress()
	if err != nil {
		log.Printf("Error: Unable to get external IP address from router using NAT-PMP: %v", err)
		return nil, nil
	}
	return nat, external
}

func tryUpnp(gateway net.IP) (NAT, net.IP) {
	log.Printf("Attempting to use UPnP to set up port forwarding.")
	nat, err := UpnpDiscover()
	if err != nil {
		log.Printf("Error: Unable to discover router capabilities using UPnP: %v", err)
		return nil, nil
	}
	external, err := nat.GetExternalAddress()
	if err != nil {
		log.Printf("Error: Unable to get external IP address from router using UPnP: %v", err)
		return nil, nil
	}
	return nat, external
}

// Top function - will call UPnP or NAT-PMP GetExternalAddr function dependent on parameters
func GetExternalAddr(port uint16) (net.IP, uint16, error) {
	var nat NAT
	var external net.IP
	loopback := net.ParseIP("127.0.0.1")

	str, err := getGateway()
	log.Printf("Gateway Address: %q", str)

	gatewayIP := net.ParseIP(str)
	if gatewayIP == nil {
		return loopback, 0, fmt.Errorf("Invalid gateway: %v", gatewayIP)
	}

	// Try NAT-PMP First
	if !onlyUpnp {
		nat, external = tryNatPmp(gatewayIP)
	}

	// If NAT-PMP failed, or -only-upnp is set, try UPnP
	if !onlyNatPmp && nat == nil {
		nat, external = tryUpnp(gatewayIP)
	}

	// If IP discovery failed
	if nat == nil {
		log.Printf("WARNING: Router port forwarding setup failed - peers will be unable to connect through firewall.")
		return loopback, port, nil
	}

	log.Printf("Router's external IP address discovered: %v", external)

	// Request that the router forward the port
	desc := fmt.Sprintf("h0tb0x:%d", port)
	port, err = nat.AddPortMapping(ProtoTCP, port, port, desc, PortMapLifetime)
	if err != nil {
		log.Printf("Error during router port forwarding request: %v", err)
		log.Printf("Peers will be unable to connect through firewall.")
		return loopback, port, nil
	}

	log.Printf("Router port forwarding request successful.")
	log.Printf("Listening on external port %v", port)
	return external, port, nil
}
