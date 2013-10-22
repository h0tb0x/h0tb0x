package link

import (
	"crypto/tls"
	"fmt"
	"h0tb0x/base"
	"h0tb0x/conn"
	"h0tb0x/crypto"
	"h0tb0x/db"
	"h0tb0x/rendezvous"
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
	rendezvous  string
	publicKey   *crypto.PublicIdentity
	host        string
	port        uint16
	failed      bool
}

type ListenerFunc func(id int, fingerprint *crypto.Digest, what FriendStatus)
type HandlerFunc func(int, *crypto.Digest, io.Reader, io.Writer) (err error)

// The LinkMgr is the primary interface for the Link Layer
type LinkMgr struct {
	*base.Base
	clientTls     *tls.Config
	friendsByFp   map[string]*friendInfo
	friendsById   map[int]*friendInfo
	friendsByHost map[string]*friendInfo
	mutex         base.RWLocker
	wait          sync.WaitGroup
	listeners     []ListenerFunc
	handlers      map[int]HandlerFunc
	client        *http.Client
	server        *http.Server
	connMgr       conn.ConnMgr
	listener      net.Listener
	rclient       *rendezvous.Client
}

// Constructs a new LinkMgr, does not start it.
func NewLinkMgr(theBase *base.Base, connMgr conn.ConnMgr) *LinkMgr {
	cert := theBase.Ident.TlsCertificate()

	serverTls := &tls.Config{
		Certificates: []tls.Certificate{*cert},
		ClientAuth:   tls.RequireAnyClientCert,
	}

	this := &LinkMgr{
		Base:          theBase,
		friendsByFp:   make(map[string]*friendInfo),
		friendsById:   make(map[int]*friendInfo),
		friendsByHost: make(map[string]*friendInfo),
		client:        new(http.Client),
		clientTls: &tls.Config{
			Certificates:       []tls.Certificate{*cert},
			InsecureSkipVerify: true, // We validate by cert hash manually
		},
		server: &http.Server{
			Addr:      fmt.Sprintf(":%d", theBase.Port),
			TLSConfig: serverTls,
		},
		handlers: make(map[int]HandlerFunc),
		connMgr:  connMgr,
		rclient:  rendezvous.NewClient(connMgr),
		mutex:    base.NewNoisyLocker(theBase.Log.Prefix() + "link "),
	}

	transport := new(http.Transport)
	transport.RegisterProtocol("h0tb0x", this)
	transport.Dial = this.dial
	this.client.Transport = transport
	this.server.Handler = this

	return this
}

func (this *LinkMgr) respondError(response http.ResponseWriter, status int, err string) {
	this.Log.Print(err)
	response.Header().Set("Content-Type", "text/plain")
	response.WriteHeader(status)
	response.Write([]byte(err))
}

// Handle inbound request
func (this *LinkMgr) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	state := request.TLS
	if state == nil {
		this.respondError(response, http.StatusBadRequest, "Must use TLS")
		return
	}

	if len(state.PeerCertificates) < 1 {
		this.respondError(response, http.StatusForbidden, "Missing peer certificicate")
		return
	}

	ident, err := crypto.PublicFromCert(state.PeerCertificates[0])
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

	this.mutex.RLock()
	handler, sok := this.handlers[service]
	if !sok {
		this.mutex.RUnlock()
		this.respondError(response, http.StatusForbidden, fmt.Sprintf("Unknown service: %d", service))
		return
	}

	fp := ident.Fingerprint().String()
	fi, ok := this.friendsByFp[fp]
	if !ok {
		this.mutex.RUnlock()
		this.respondError(response, http.StatusForbidden, fmt.Sprintf("Unknown friend: %s", fp))
		return
	}
	this.wait.Add(1)
	response.Header().Set("Content-Type", "application/binary")
	check := &didWrite{inner: response, wrote: false}
	err = handler(fi.id, ident.Fingerprint(), request.Body, check)
	this.mutex.RUnlock()
	this.wait.Done()

	if err != nil && !check.wrote {
		this.respondError(response, http.StatusInternalServerError, err.Error())
		return
	}
}

func (this *LinkMgr) getFriendByFp(fp string) *friendInfo {
	this.mutex.RLock()
	defer this.mutex.RUnlock()
	fi, ok := this.friendsByFp[fp]
	if !ok {
		return nil
	}
	return fi
}

func (this *LinkMgr) getFriendByHost(host string) *friendInfo {
	this.mutex.RLock()
	defer this.mutex.RUnlock()
	fi, ok := this.friendsByHost[host]
	if !ok {
		return nil
	}
	return fi
}

func (this *LinkMgr) RoundTrip(req *http.Request) (*http.Response, error) {
	fp := req.URL.Host
	fi := this.getFriendByFp(fp)
	if fi == nil {
		this.Log.Printf("No such friend")
		return nil, fmt.Errorf("Dial of removed friend: %s", fp)
	}

	// resolve the request
	this.Log.Printf("Doing Rendezvous lookup")
	if fi.failed || fi.host == "$" {
		rec, err := this.rclient.Get("http://"+fi.rendezvous, fp)
		if err == nil {
			this.Log.Printf("Got new host: %s, port: %d", rec.Host, rec.Port)
			fi = this.UpdateHostData(fi.fingerprint, rec.Host, rec.Port)
		} else {
			this.Log.Printf("Rendezvous failed: %s", err)
		}
	}

	if fi.host == "$" {
		return nil, fmt.Errorf("Unable to connect, don't know address yet & rendezvous failed")
	}

	// re-write the url
	req.URL.Scheme = "http"
	req.URL.Host = fmt.Sprintf("%s:%d", fi.host, fi.port)
	return this.client.Do(req)
}

// Generate an outbound TLS connection so we can hand verify the remote side
func (this *LinkMgr) dial(proto, host string) (net.Conn, error) {
	fi := this.getFriendByHost(host)
	if fi == nil {
		this.Log.Printf("No such friend")
		return nil, fmt.Errorf("Dial of removed friend: %s", host)
	}
	this.Log.Printf("Dialing(%s)", host)
	tcp, err := this.connMgr.Dial(proto, host, DialTimeout)
	if err != nil {
		fi.failed = true
		return nil, err
	}
	conn := tls.Client(tcp, this.clientTls)
	err = conn.Handshake()
	if err != nil {
		fi.failed = true
		return nil, err
	}
	state := conn.ConnectionState()
	if len(state.PeerCertificates) < 1 {
		err = fmt.Errorf("Missing peer certificate")
		fi.failed = true
		return nil, err
	}
	ident, err := crypto.PublicFromCert(state.PeerCertificates[0])
	if err != nil {
		fi.failed = true
		return nil, err
	}
	if ident.Fingerprint().String() != fi.fingerprint.String() {
		err = fmt.Errorf("Invalid peer certificate")
		fi.failed = true
		return nil, err
	}
	return conn, nil
}

// Add a handler for a certain 'service id'
func (this *LinkMgr) AddHandler(service int, handler HandlerFunc) {
	this.handlers[service] = handler
}

// Add a listener to get link status notificiations
func (this *LinkMgr) AddListener(listener ListenerFunc) {
	this.listeners = append(this.listeners, listener)
}

func (this *LinkMgr) decodeFriend(row db.Row, failed bool) *friendInfo {
	var id int
	var fp []byte
	var rendezvous string
	var pubData []byte
	var host string
	var port uint16
	this.Db.Scan(row, &id, &fp, &rendezvous, &pubData, &host, &port)
	var fingerprint *crypto.Digest
	err := transfer.DecodeBytes(fp, &fingerprint)
	if err != nil {
		panic(err)
	}
	var publicKey *crypto.PublicIdentity
	if pubData != nil {
		err := transfer.DecodeBytes(pubData, &publicKey)
		if err != nil {
			panic(err)
		}
	}
	return &friendInfo{
		id:          id,
		fingerprint: fingerprint,
		rendezvous:  rendezvous,
		publicKey:   publicKey,
		host:        host,
		port:        port,
		failed:      failed,
	}
}

// Kicks off the link manager, presumes Callbacks has been set
func (this *LinkMgr) Start() error {
	rows := this.Db.MultiQuery(
		"SELECT id, fingerprint, rendezvous, public_key, host, port FROM Friend")
	for rows.Next() {
		fi := this.decodeFriend(rows, false)
		this.friendsByFp[fi.fingerprint.String()] = fi
		this.friendsById[fi.id] = fi
		hostKey := fmt.Sprintf("%s:%d", fi.host, fi.port)
		this.friendsByHost[hostKey] = fi
	}

	this.mutex.RLock()
	for id, fi := range this.friendsById {
		for _, onListener := range this.listeners {
			onListener(id, fi.fingerprint, FriendStartup)
		}
	}
	this.mutex.RUnlock()

	listener, err := this.connMgr.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.listener = tls.NewListener(listener, this.server.TLSConfig)

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

// Add a new friend, or if the friend exists, update rendezvous address
func (this *LinkMgr) AddUpdateFriend(fp *crypto.Digest, rendezvous string) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	// Make or insert friend
	result := this.Db.Exec(`
		INSERT OR IGNORE INTO Friend (
			id, fingerprint, rendezvous, host
		) VALUES (
			NULL, ?, ?, '$'
		)`,
		fp.Bytes(), rendezvous)
	id64, _ := result.LastInsertId()
	id := int(id64)

	this.Db.Exec("UPDATE Friend SET rendezvous = ? WHERE id = ?", rendezvous, id)

	_, ok := this.friendsByFp[fp.String()]
	row := this.Db.SingleQuery(`
		SELECT 
			id, fingerprint, rendezvous, public_key, host, port 
		FROM Friend 
		WHERE id = ?`, id)
	fi := this.decodeFriend(row, false)
	this.friendsByFp[fi.fingerprint.String()] = fi
	this.friendsById[id] = fi
	if !ok {
		// If it was added, signal upper layer
		for _, onListener := range this.listeners {
			onListener(id, fp, FriendAdded)
		}
	}
}

// Update a friend's host and port data, allows bypass of rendezvous mechanism
func (this *LinkMgr) UpdateHostData(fp *crypto.Digest, host string, port uint16) *friendInfo {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	row := this.Db.SingleQuery("SELECT id FROM Friend WHERE fingerprint = ?", fp.Bytes())
	var id int
	this.Db.Scan(row, &id)
	this.Db.Exec("UPDATE Friend SET host = ?, port = ? WHERE id = ?", host, port, id)
	row = this.Db.SingleQuery(`
		SELECT 
			id, fingerprint, rendezvous, public_key, host, port 
		FROM Friend 
		WHERE id = ?`, id)
	fi := this.decodeFriend(row, false)
	this.friendsByFp[fi.fingerprint.String()] = fi
	this.friendsById[id] = fi
	hostKey := fmt.Sprintf("%s:%d", fi.host, fi.port)
	this.friendsByHost[hostKey] = fi
	return fi
}

// Add a new friend, or if the friend exists, update the host and port data.
func (this *LinkMgr) RemoveFriend(fp *crypto.Digest) {
	this.mutex.Lock()
	defer this.mutex.Unlock()
	fi, ok := this.friendsByFp[fp.String()]
	if !ok {
		return
	}
	for _, onListener := range this.listeners {
		onListener(fi.id, fp, FriendRemoved)
	}
	this.Db.Exec("DELETE FROM FRIEND WHERE id = ?", fi.id)
	delete(this.friendsByFp, fp.String())
	delete(this.friendsById, fi.id)
}

// Send a request to a friend and get a response
func (this *LinkMgr) Send(service int, id int, req io.Reader, wr io.Writer) error {
	fi := this.friendsById[id]
	url := fmt.Sprintf("h0tb0x://%s/h0tb0x/%d", fi.fingerprint, service)
	resp, err := this.client.Post(url, "application/binary", req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("RPC had non 200 http return code: %d", resp.StatusCode)
	}
	if resp.Header.Get("Content-Type") != "application/binary" {
		return fmt.Errorf("Content type mismatch")
	}
	_, err = io.Copy(wr, resp.Body)
	return err
}
