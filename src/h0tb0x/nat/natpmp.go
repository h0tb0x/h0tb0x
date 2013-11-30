package nat

// Developed by Jackpal - https://github.com/jackpal/Taipei-Torrent
// Commit 86371b62420307c3c8e6b144327e3120d64b83e0
// Tue Jan 1 2013

import (
	natpmp "code.google.com/p/go-nat-pmp"
	"fmt"
	"net"
)

// Adapt the NAT-PMP protocol to the NAT interface

// TODO:
//  + Register for changes to the external address.
//  + Re-register port mapping when router reboots.
//  + A mechanism for keeping a port mapping registered.

type natPMPClient struct {
	client *natpmp.Client
}

func NewNatPMP(gateway net.IP) (nat NAT) {
	return &natPMPClient{natpmp.NewClient(gateway)}
}

func (this *natPMPClient) GetExternalAddress() (net.IP, error) {
	response, err := this.client.GetExternalAddress()
	if err != nil {
		return nil, err
	}
	ip := response.ExternalIPAddress
	addr := net.IPv4(ip[0], ip[1], ip[2], ip[3])
	return addr, nil
}

func (this *natPMPClient) AddPortMapping(
	proto string,
	internalPort, externalPort uint16,
	description string,
	timeout int) (uint16, error) {
	if timeout <= 0 {
		return 0, fmt.Errorf("timeout must not be <= 0")
	}
	response, err := this.client.AddPortMapping(
		proto, int(internalPort), int(externalPort), timeout)
	if err != nil {
		return 0, err
	}
	return response.MappedExternalPort, nil
}

func (this *natPMPClient) DeletePortMapping(proto string, externalPort, internalPort int) error {
	// To destroy a mapping, send an add-port with
	// an internalPort of the internal port to destroy, an external port of zero and a time of zero.
	_, err := this.client.AddPortMapping(proto, internalPort, 0, 0)
	return err
}
