package conn

import (
	"net"
	"net/http"
	"time"
)

const (
	DialTimeout = 5 * time.Second
)

type ConnMgr interface {
	Dial(proto, address string, timeout time.Duration) (net.Conn, error)
	Listen(proto, address string) (net.Listener, error)
}

type netConnMgr struct {
}

func NewNetConnMgr() ConnMgr {
	return new(netConnMgr)
}

func (this *netConnMgr) Dial(proto, address string, timeout time.Duration) (net.Conn, error) {
	return net.DialTimeout(proto, address, timeout)
}

func (this *netConnMgr) Listen(proto, address string) (net.Listener, error) {
	return net.Listen(proto, address)
}

type HttpClient struct {
	*http.Client
	connMgr ConnMgr
}

func NewHttpClient(connMgr ConnMgr, timeout time.Duration) *http.Client {
	this := &HttpClient{
		Client:  new(http.Client),
		connMgr: connMgr,
	}
	this.Transport = &http.Transport{
		Dial: this.dial,
		ResponseHeaderTimeout: timeout,
	}
	return this.Client
}

func (this *HttpClient) dial(proto, host string) (net.Conn, error) {
	return this.connMgr.Dial(proto, host, DialTimeout)
}
