package test

import (
	"fmt"
	"h0tb0x/base"
	"h0tb0x/conn"
	"h0tb0x/crypto"
	"h0tb0x/db"
	"io"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"log"
	"net"
	"os"
	"sync"
	"time"
)

type TestMgr struct {
	C         *C
	ConnMgr   conn.ConnMgr
	tempFiles []string
	tempDirs  []string
}

func (this *TestMgr) SetUpTest(c *C) {
	this.C = c
	this.ConnMgr = newConnMgr()
	this.tempFiles = []string{}
	this.tempDirs = []string{}
}

func (this *TestMgr) TearDownTest(c *C) {
	for _, path := range this.tempFiles {
		os.Remove(path)
	}
	for _, path := range this.tempDirs {
		os.RemoveAll(path)
	}
}

func (this *TestMgr) GetTempFile() string {
	ftmp, err := ioutil.TempFile("", "")
	this.C.Assert(err, IsNil)
	defer ftmp.Close()
	path := ftmp.Name()
	this.tempFiles = append(this.tempFiles, path)
	return path
}

func (this *TestMgr) GetTempDir() string {
	path, err := ioutil.TempDir("", "")
	this.C.Assert(err, IsNil)
	this.tempDirs = append(this.tempDirs, path)
	return path
}

func (this *TestMgr) NewBase(name string, port uint16) *base.Base {
	return &base.Base{
		Log:   log.New(os.Stderr, fmt.Sprintf("[%v] ", name), log.LstdFlags),
		Ident: crypto.NewSecretIdentity(""),
		Port:  port,
		Db:    db.NewDatabase(this.GetTempFile(), "h0tb0x"),
	}
}

type connMgr struct {
	listeners map[string]*localListener
	mutex     sync.RWMutex
}

func newConnMgr() *connMgr {
	return &connMgr{
		listeners: make(map[string]*localListener),
	}
}

func (this *connMgr) getListener(port string) *localListener {
	this.mutex.RLock()
	defer this.mutex.RUnlock()
	listener, ok := this.listeners[port]
	if !ok {
		return nil
	}
	return listener
}

func (this *connMgr) Dial(proto, address string, timeout time.Duration) (net.Conn, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	listener := this.getListener(port)
	if listener == nil {
		return nil, fmt.Errorf("Could not find: %s", address)
	}
	p1, p2 := net.Pipe()
	listener.ch <- p1
	return p2, nil
}

func (this *connMgr) Listen(proto, address string) (net.Listener, error) {
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	this.mutex.Lock()
	defer this.mutex.Unlock()
	listener, ok := this.listeners[port]
	if ok {
		return nil, fmt.Errorf("Address already in use: %s", address)
	}
	listener = newLocalListener()
	this.listeners[port] = listener
	return listener, nil
}

type localListener struct {
	ch    chan net.Conn
	alive bool
}

func newLocalListener() *localListener {
	return &localListener{
		ch:    make(chan net.Conn),
		alive: true,
	}
}

func (this *localListener) Accept() (net.Conn, error) {
	conn := <-this.ch
	if conn == nil {
		return nil, io.EOF
	}
	return conn, nil
}

func (this *localListener) Close() error {
	if this.alive {
		close(this.ch)
		this.alive = false
	}
	return nil
}

func (this *localListener) Addr() net.Addr {
	return localAddr(0)
}

type localAddr int

func (localAddr) Network() string {
	return "local"
}

func (localAddr) String() string {
	return "local"
}
