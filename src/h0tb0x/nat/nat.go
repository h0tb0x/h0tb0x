package nat

// NAT Developed by Jackpal - https://github.com/jackpal/Taipei-Torrent
// Commit dd88a8bfac6431c01d959ce3c745e74b8a911793
// Tue Jan 1 2013

import (
	"log"
	"net"
	"strconv"
)

// protocol is either "udp" or "tcp"
type NAT interface {
	GetExternalAddress() (addr net.IP, err error)
	AddPortMapping(protocol string, externalPort, internalPort int, description string, timeout int) (mappedExternalPort int, err error)
	DeletePortMapping(protocol string, externalPort, internalPort int) (err error)
}

func ChooseListenPort(natobj NAT, port uint16) (listenPort int, err error) {
	listenPort = int(port)

	// TODO: Unmap port when exiting. (Right now we never exit cleanly.)
	// TODO: Defend the port, remap when router reboots
	// NOTE:  My NetGear router returned a 500:Internal Server Error if the timeout was set higher than 0  (was originally 360000) - Mux
	listenPort, err = natobj.AddPortMapping("TCP", listenPort, listenPort,
		"H0TB0X_TCP_PORT_FORWARD_"+strconv.Itoa(listenPort), 0) // 360000
	if err != nil {
		log.Println("Could not choose listen port - error:  ", err)
		return
	}
	return
}
