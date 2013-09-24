package link

import (
	"crypto/tls"
	"fmt"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/transfer"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
)

type FriendStatus int

const (
	FriendStartup FriendStatus = iota // Sent after 'Run' to alter upper layer of existing links
	FriendAdded                       // Sent when a friend is added while running
	FriendRemoved                     // Sent when a friend is removed while running
)

const (
	ServiceNotify = 1
	ServiceData   = 2
)

const (
	DialTimeout = 3 * time.Second
)

type didWrite struct {
	inner io.Writer
	wrote bool
}

func (this *didWrite) Write(p []byte) (n int, err error) {
	this.wrote = true
	return this.inner.Write(p)
}

type friendInfo struct {
	id          int
	fingerprint *crypto.Digest
	host        string
	port        uint16
}

// The LinkMgr is the primary interface for the Link Layer
type LinkMgr struct {
	*base.Base
	listener  net.Listener
	server    *http.Server
	clientTls *tls.Config
	client    *http.Client
	friendsFp map[string]*friendInfo
	friendsId map[int]*friendInfo
	cmut      sync.RWMutex
	wait      sync.WaitGroup
	listeners []func(id int, fingerprint *crypto.Digest, what FriendStatus)
	handlers  map[int]func(int, *crypto.Digest, io.Reader, io.Writer) (err error)
}

func (this *LinkMgr) respondError(response http.ResponseWriter, status int, err string) {
	this.Log.Print(err)
	response.Header().Set("Content-Type", "text/plain")
	response.WriteHeader(status)
	response.Write([]byte(err))
}

// Used to forward ServeHTTP to LinkMgr without making it public
type hideServer struct {
	impl *LinkMgr
}

// Handle inbound request, implemented on hideServer to make non-public
func (hthis *hideServer) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	this := hthis.impl
	state := request.TLS
	if state == nil {
		this.respondError(response, http.StatusBadRequest, "Must use TLS")
		return
	}

	if len(state.PeerCertificates) < 1 {
		this.respondError(response, http.StatusForbidden, "Missing peer certificicate")
		return
	}

	id, err := crypto.PublicFromCert(state.PeerCertificates[0])
	if err != nil {
		this.respondError(response, http.StatusForbidden, "Invalid peer certificicate")
		return
	}

	if request.Header.Get("Content-Type") != "application/binary" {
		this.respondError(response, http.StatusBadRequest, "Invalid content type")
		return
	}

	var service int
	_, err = fmt.Sscanf(request.URL.Path, "/h0tb0x/%d", &service)
	if err != nil {
		this.respondError(response, http.StatusNotFound, fmt.Sprintf("Unknown URL: '%s'", request.URL.Path))
		return
	}

	if request.Method != "POST" {
		this.respondError(response, http.StatusMethodNotAllowed, fmt.Sprintf("Invalid method: '%s'", request.Method))
		return
	}

	this.cmut.RLock()
	handler, sok := this.handlers[service]
	if !sok {
		this.cmut.RUnlock()
		this.respondError(response, http.StatusForbidden, fmt.Sprintf("Unknown service: %d", service))
		return
	}

	fi, ok := this.friendsFp[id.Fingerprint().String()]
	if !ok {
		this.cmut.RUnlock()
		this.respondError(response, http.StatusForbidden, fmt.Sprintf("Unknown friend: %s", id.Fingerprint().String()))
		return
	}
	this.wait.Add(1)
	response.Header().Set("Content-Type", "application/binary")
	check := &didWrite{inner: response, wrote: false}
	err = handler(fi.id, id.Fingerprint(), request.Body, check)
	this.cmut.RUnlock()
	this.wait.Done()

	if err != nil && !check.wrote {
		this.respondError(response, http.StatusInternalServerError, err.Error())
		return
	}
}

// Generate an outbound TLS connection so we can hand verify the remote side
func (this *LinkMgr) safeDial(netStr string, host string) (net.Conn, error) {
	var id int
	_, err := fmt.Sscanf(host, "id_%d:80", &id)
	if err != nil {
		return nil, err
	}
	this.cmut.RLock()
	fi, ok := this.friendsId[id]
	if !ok {
		this.cmut.RUnlock()
		this.Log.Printf("No such friend")
		return nil, fmt.Errorf("Dial of removed friend: %s", host)
	}
	this.cmut.RUnlock()
	this.Log.Printf("Dialing(%s:%d)", fi.host, fi.port)
	var dialer net.Dialer
	dialer.Deadline = time.Now().Add(DialTimeout)
	tcpconn, err := dialer.Dial(netStr, fmt.Sprintf("%s:%d", fi.host, fi.port))

	if err != nil {
		return nil, err
	}

	conn := tls.Client(tcpconn, this.clientTls)
	err = conn.Handshake()
	if err != nil {
		return nil, err
	}

	state := conn.ConnectionState()

	if len(state.PeerCertificates) < 1 {
		err = fmt.Errorf("Missing peer certificate")
		return nil, err
	}

	ident, err := crypto.PublicFromCert(state.PeerCertificates[0])
	if err != nil {
		return nil, err
	}
	if ident.Fingerprint().String() != fi.fingerprint.String() {
		err = fmt.Errorf("Invalid peer certificate")
		return nil, err
	}

	return conn, nil
}

// Constructs a new LinkMgr, does not start it.
func NewLinkMgr(base *base.Base) *LinkMgr {
	cert := base.Ident.TlsCertificate()

	serverTlsCfg := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientAuth:   tls.RequireAnyClientCert,
	}

	clientTlsCfg := &tls.Config{
		Certificates:       []tls.Certificate{*cert},
		InsecureSkipVerify: true, // We validate by cert hash manually
	}

	client := &http.Client{}

	server := &http.Server{
		Addr:      fmt.Sprintf(":%d", base.Port),
		TLSConfig: serverTlsCfg,
	}

	linkMgr := &LinkMgr{
		Base:      base,
		friendsFp: make(map[string]*friendInfo),
		friendsId: make(map[int]*friendInfo),
		server:    server,
		clientTls: clientTlsCfg,
		client:    client,
		listeners: make([]func(id int, fingerprint *crypto.Digest, what FriendStatus), 0),
		handlers:  make(map[int]func(int, *crypto.Digest, io.Reader, io.Writer) (err error)),
	}

	server.Handler = &hideServer{impl: linkMgr}

	client.Transport = &http.Transport{
		Dial: linkMgr.safeDial,
	}

	return linkMgr
}

// Add a handler for a certain 'service id'
func (this *LinkMgr) AddHandler(service int, f func(int, *crypto.Digest, io.Reader, io.Writer) (err error)) {
	this.handlers[service] = f
}

// Add a listener to get link status notificiations
func (this *LinkMgr) AddListener(f func(id int, fingerprint *crypto.Digest, what FriendStatus)) {
	this.listeners = append(this.listeners, f)
}

// Kicks off the link manager, presumes Callbacks has been set
func (this *LinkMgr) Run() error {
	conn, err := net.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.listener = tls.NewListener(conn, this.server.TLSConfig)

	rows := this.Db.MultiQuery("SELECT id, fingerprint, host, port FROM Friend")
	for rows.Next() {
		var id int
		var fp []byte
		var host string
		var port uint16
		this.Db.Scan(rows, &id, &fp, &host, &port)
		var fingerprint *crypto.Digest
		err := transfer.DecodeBytes(fp, &fingerprint)
		if err != nil {
			panic(err)
		}
		fi := &friendInfo{id: id, fingerprint: fingerprint, host: host, port: port}
		this.friendsFp[fingerprint.String()] = fi
		this.friendsId[id] = fi
	}

	this.cmut.RLock()
	for id, fi := range this.friendsId {
		for _, f := range this.listeners {
			f(id, fi.fingerprint, FriendStartup)
		}
	}
	this.cmut.RUnlock()

	this.wait.Add(1)
	go func() {
		this.server.Serve(this.listener)
		this.wait.Done()
	}()

	return nil
}

func (this *LinkMgr) Stop() {
	this.listener.Close()
	this.wait.Wait()
	this.Db.Close()
}

// Add a new friend, or if the friend exists, update the host and port data.
func (this *LinkMgr) AddUpdateFriend(fp *crypto.Digest, host string, port uint16) {
	this.cmut.Lock()
	defer this.cmut.Unlock()
	// Make or insert friend
	this.Db.Exec(
		"INSERT OR IGNORE INTO Friend (id, fingerprint, host, port) VALUES (NULL, ?, ?, ?)",
		fp.Bytes(), host, port)

	row := this.Db.SingleQuery("SELECT id FROM Friend WHERE fingerprint = ?", fp.Bytes())
	var id int
	this.Db.Scan(row, &id)

	this.Db.Exec("UPDATE Friend SET host = ?, port = ? WHERE id = ?",
		host, port, id)

	_, ok := this.friendsFp[fp.String()]
	fi := &friendInfo{id: id, fingerprint: fp, host: host, port: port}
	this.friendsFp[fp.String()] = fi
	this.friendsId[id] = fi
	if !ok {
		// If it was added, signal upper layer
		for _, f := range this.listeners {
			f(id, fp, FriendAdded)
		}
	}
}

// Add a new friend, or if the friend exists, update the host and port data.
func (this *LinkMgr) RemoveFriend(fp *crypto.Digest) {
	this.cmut.Lock()
	defer this.cmut.Unlock()
	fi, ok := this.friendsFp[fp.String()]
	if !ok {
		return
	}
	for _, f := range this.listeners {
		f(fi.id, fp, FriendRemoved)
	}
	this.Db.Exec("DELETE FROM FRIEND WHERE id = ?", fi.id)
	delete(this.friendsFp, fp.String())
	delete(this.friendsId, fi.id)
}

// Send a request to a friend and get a response
func (this *LinkMgr) Send(service int, id int, req io.Reader, resp io.Writer) (err error) {
	url := fmt.Sprintf("http://id_%d:80/h0tb0x/%d", id, service)
	httpResp, err := this.client.Post(url, "application/binary", req)
	if err != nil {
		if httpResp != nil {
			httpResp.Body.Close()
		}
		return
	}
	if httpResp.StatusCode != http.StatusOK {
		err = fmt.Errorf("RPC had non 200 http return code: %d", httpResp.StatusCode)
		httpResp.Body.Close()
		return
	}
	if httpResp.Header.Get("Content-Type") != "application/binary" {
		err = fmt.Errorf("Content type mismatch")
		httpResp.Body.Close()
		return
	}
	_, err = io.Copy(resp, httpResp.Body)
}
