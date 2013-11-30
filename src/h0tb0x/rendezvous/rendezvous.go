package rendezvous

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"h0tb0x/conn"
	"h0tb0x/crypto"
	"h0tb0x/db"
	"h0tb0x/transfer"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

var (
	client *http.Client
)

// Represents a record that can be published into the rendezvous server
type Record struct {
	Fingerprint string `db:"fingerprint"`
	PublicKey   string `db:"public_key"`
	Timestamp   int64  `db:"version"`
	Host        string `db:"host"`
	Port        uint16 `db:"port"`
	Signature   string `db:"signature"`
}

type Client struct {
	*http.Client
}

func NewClient(connMgr conn.ConnMgr) *Client {
	return &Client{conn.NewHttpClient(connMgr)}
}

// Talks to a rendezvous server at addr and gets a record.
// Also validated signature.
// Returns nil on error.
// TODO: timeout support
func (this *Client) Get(url, fingerprint string) (*Record, error) {
	resp, err := this.Client.Get(url + "/" + fingerprint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid http return: %d - %s", resp.StatusCode, resp.Status)
	}
	var record *Record
	err = json.NewDecoder(resp.Body).Decode(&record)
	if err != nil {
		return nil, err
	}
	// fmt.Printf("GetRendezvous:\n")
	// record.dump()
	if !record.CheckSignature() {
		return nil, fmt.Errorf("Signature validation failed")
	}
	return record, nil
}

// Puts a rendezvous record to the address in the record.
// Presumes the record is signed.
// TODO: timeout support
func (this *Client) Put(url string, ident *crypto.SecretIdentity, host string, port uint16) error {
	// fmt.Printf("PutRendezvous:\n")
	record := &Record{
		Timestamp: time.Now().Unix(),
		Host:      host,
		Port:      port,
	}
	record.Sign(ident)
	// record.dump()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(&record)
	req, err := http.NewRequest("PUT", url+"/"+record.Fingerprint, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := this.Client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		return fmt.Errorf("PUT failed: [%d] %s", resp.StatusCode, body)
	}
	return nil
}

// Validates the signature on a record
func (this *Record) CheckSignature() bool {
	var pub *crypto.PublicIdentity
	var sig *crypto.Signature
	err := transfer.DecodeString(this.PublicKey, &pub)
	if err != nil {
		fmt.Printf("Error: CheckSignature failed to decode PublicKey: %s\n", this.PublicKey)
		return false
	}
	err = transfer.DecodeString(this.Signature, &sig)
	if err != nil {
		fmt.Printf("Error: CheckSignature failed to decode Signature: %s\n", this.Signature)
		return false
	}
	fp := pub.Fingerprint().String()
	if fp != this.Fingerprint {
		fmt.Printf("Error: CheckSignature fingerprint mismatch: %s\n", this.Fingerprint)
		return false
	}
	digest := crypto.HashOf(this.Timestamp, this.Host, this.Port)
	if !pub.Verify(digest, sig) {
		fmt.Printf("Error: CheckSignature failed to verify signature\n")
		return false
	}
	return true
}

// Given that Timestamp, Host and Port are set, sets the rest of the fields and signs
func (this *Record) Sign(private *crypto.SecretIdentity) {
	pub := private.Public()
	fp := private.Fingerprint()
	this.Fingerprint = fp.String()
	this.PublicKey = transfer.AsString(pub)
	digest := crypto.HashOf(this.Timestamp, this.Host, this.Port)
	sig := private.Sign(digest)
	this.Signature = transfer.AsString(sig)
}

func (this *Record) dump() {
	fmt.Printf("\tFingerprint: %q\n", this.Fingerprint)
	fmt.Printf("\tPublicKey: %q\n", this.PublicKey)
	fmt.Printf("\tTimestamp: %d\n", this.Timestamp)
	fmt.Printf("\tHost: %q\n", this.Host)
	fmt.Printf("\tPort: %d\n", this.Port)
	fmt.Printf("\tSignature: %q\n", this.Signature)
}

// TODO: Dedup this code (it also appears in API, but I didn't know if I should make a whole module

func sendJson(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(obj)
}

func sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(message)
}

func decodeJsonBody(w http.ResponseWriter, req *http.Request, out interface{}) bool {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		sendError(w, http.StatusBadRequest, "Invalid content type")
		return false
	}
	err := json.NewDecoder(req.Body).Decode(out)
	if err != nil {
		sendError(w, http.StatusBadRequest, "Unable to decode JSON")
		return false
	}
	return true
}

func (this *RendezvousMgr) onPut(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	key := vars["key"]
	// fmt.Printf("PUT request for key %s\n", key)
	var record *Record
	if !decodeJsonBody(w, req, &record) {
		return
	}
	// record.dump()
	if record.Fingerprint != key || !record.CheckSignature() {
		sendError(w, http.StatusUnauthorized, "Unable to validate record")
		return
	}
	tx, err := this.db.Begin()
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	recno, err := tx.SelectNullInt(`
		SELECT version 
		FROM Rendezvous 
		WHERE fingerprint = ?`, record.Fingerprint)
	if err != nil {
		tx.Rollback()
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if recno.Valid {
		if record.Timestamp <= recno.Int64 {
			tx.Rollback()
			sendError(w, http.StatusConflict, "Record too old")
			return
		}
		_, err = tx.Update(record)
		if err != nil {
			tx.Rollback()
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
	} else {
		err := tx.Insert(record)
		if err != nil {
			tx.Rollback()
			sendError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	tx.Commit()
}

func (this *RendezvousMgr) onGet(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	key := vars["key"]
	record, err := this.db.Get(&Record{}, key)
	if err != nil {
		sendError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if record == nil {
		sendError(w, http.StatusNotFound, "Unknown Key")
		return
	}
	// record.(*Record).dump()
	sendJson(w, record)
}

// Represents the 'server' side of the Rendezvous protocol
type RendezvousMgr struct {
	db       *db.Database
	connMgr  conn.ConnMgr
	listener net.Listener
	router   *mux.Router
	server   *http.Server
	wait     sync.WaitGroup
}

func NewRendezvousMgr(connMgr conn.ConnMgr, port uint16, file string) *RendezvousMgr {
	router := mux.NewRouter()
	this := &RendezvousMgr{
		db:      db.NewDatabase(file, "rendezvous"),
		connMgr: connMgr,
		server: &http.Server{
			Addr:    fmt.Sprintf(":%d", port),
			Handler: router,
		},
	}
	router.HandleFunc("/{key}", this.onGet).Methods("GET")
	router.HandleFunc("/{key}", this.onPut).Methods("PUT")
	this.db.AddTableWithName(Record{}, "Rendezvous").SetKeys(false, "fingerprint")
	return this
}

// Start the server
func (this *RendezvousMgr) Start() error {
	var err error
	this.listener, err = this.connMgr.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.wait.Add(1)
	go func() {
		this.server.Serve(this.listener)
		this.wait.Done()
	}()
	return nil
}

// Stops the server
func (this *RendezvousMgr) Stop() {
	if this.listener != nil {
		this.listener.Close()
	}
	this.wait.Wait()
}

func Serve(connMgr conn.ConnMgr, port uint16, file string) {
	rendezvous := NewRendezvousMgr(connMgr, port, file)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)

	fmt.Println("Rendezvous server starting")
	rendezvous.Start()
	fmt.Println("Rendezvous server started")
	<-ch
	fmt.Println("Rendezvous server stopping")
	rendezvous.Stop()
	fmt.Println("Rendezvous server stopped")
}
