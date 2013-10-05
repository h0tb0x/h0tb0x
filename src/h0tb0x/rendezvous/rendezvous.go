package rendezvous

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/h0tb0x/mux"
	"h0tb0x/crypto"
	"h0tb0x/db"
	"h0tb0x/transfer"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"
)

const (
	DialTimeout = 5 * time.Second
)

var (
	client *http.Client
)

// Represents a record that can be published into the rendezvous server
type RecordJson struct {
	Fingerprint string
	PublicKey   string
	Version     int
	Host        string
	Port        uint16
	Signature   string
}

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			Dial: shortDial,
		},
	}
}

func shortDial(network, host string) (net.Conn, error) {
	dialer := &net.Dialer{
		Deadline: time.Now().Add(DialTimeout),
	}
	return dialer.Dial(network, host)
}

// Validates the signature on a record
func (this *RecordJson) CheckSignature() bool {
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
	digest := crypto.HashOf(this.Version, this.Host, this.Port)
	if !pub.Verify(digest, sig) {
		fmt.Printf("Error: CheckSignature failed to verify signature\n")
		return false
	}
	return true
}

// Given that Version, Host and Port are set, sets the rest of the fields and signs
func (this *RecordJson) Sign(private *crypto.SecretIdentity) {
	pub := private.Public()
	fp := private.Fingerprint()
	this.Fingerprint = fp.String()
	this.PublicKey = transfer.AsString(pub)
	digest := crypto.HashOf(this.Version, this.Host, this.Port)
	sig := private.Sign(digest)
	this.Signature = transfer.AsString(sig)
}

func (this *RecordJson) dump() {
	fmt.Printf("\tFingerprint: %q\n", this.Fingerprint)
	fmt.Printf("\tPublicKey: %q\n", this.PublicKey)
	fmt.Printf("\tVersion: %d\n", this.Version)
	fmt.Printf("\tHost: %q\n", this.Host)
	fmt.Printf("\tPort: %d\n", this.Port)
	fmt.Printf("\tSignature: %q\n", this.Signature)
}

// Talks to a rendezvous server at addr and gets a record.
// Also validated signature.
// Returns nil on error.
// TODO: timeout support
func GetRendezvous(url, fingerprint string) (*RecordJson, error) {
	resp, err := client.Get(url + "/" + fingerprint)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Invalid http return: %d - %s", resp.StatusCode, resp.Status)
	}
	var rec *RecordJson
	err = json.NewDecoder(resp.Body).Decode(&rec)
	if err != nil {
		return nil, err
	}
	fmt.Printf("GetRendezvous:\n")
	rec.dump()
	if !rec.CheckSignature() {
		return nil, fmt.Errorf("Bad signature for record")
	}
	return rec, nil
}

// Puts a rendezvous record to the address in the record.
// Presumes the record is signed.
// TODO: timeout support
func PutRendezvous(url string, record *RecordJson) error {
	fmt.Printf("PutRendezvous:\n")
	record.dump()
	var buf bytes.Buffer
	json.NewEncoder(&buf).Encode(&record)
	req, err := http.NewRequest("PUT", url+"/"+record.Fingerprint, &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Invalid status on Put: %d", resp.StatusCode)
	}
	return nil
}

func Publish(url string, ident *crypto.SecretIdentity, host string, port uint16) error {
	rec := &RecordJson{
		Version: int(time.Now().Unix()),
		Host:    host,
		Port:    port,
	}
	rec.Sign(ident)
	return PutRendezvous(url, rec)
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
	fmt.Printf("PUT request for key %s\n", key)
	var record *RecordJson
	if !decodeJsonBody(w, req, &record) {
		return
	}
	record.dump()
	if record.Fingerprint != key || !record.CheckSignature() {
		sendError(w, http.StatusUnauthorized, "Unable to validate record")
		return
	}
	recno := -1
	row := this.database.SingleQuery(`
		SELECT version 
		FROM Rendezvous 
		WHERE fingerprint = ?`, record.Fingerprint)
	exists := this.database.MaybeScan(row, &recno)
	if record.Version <= recno {
		sendError(w, http.StatusConflict, "Record too old")
		return
	}
	if exists {
		this.database.Exec(`
			UPDATE Rendezvous 
			SET 
				version = ?, 
				host = ?, 
				port = ?, 
				signature = ?`,
			record.Version, record.Host, record.Port, record.Signature)
	} else {
		this.database.Exec(`
			INSERT INTO Rendezvous (
				fingerprint, 
				public_key, 
				version, 
				host, 
				port, 
				signature
			) VALUES (?, ?, ?, ?, ?, ?)`,
			record.Fingerprint, record.PublicKey, record.Version,
			record.Host, record.Port, record.Signature)
	}
}

func (this *RendezvousMgr) onGet(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	key := vars["key"]
	fmt.Printf("GET request for key %s\n", key)
	row := this.database.SingleQuery(`
		SELECT 
			public_key, 
			version, 
			host, 
			port, 
			signature
		FROM Rendezvous 
		WHERE fingerprint = ?`, key)
	record := &RecordJson{Fingerprint: key}
	if !this.database.MaybeScan(row,
		&record.PublicKey,
		&record.Version,
		&record.Host,
		&record.Port,
		&record.Signature) {
		sendError(w, http.StatusNotFound, "Unknown Key")
		return
	}
	record.dump()
	sendJson(w, record)
}

// Represents the 'server' side of the Rendezvous protocol
type RendezvousMgr struct {
	database *db.Database
	listener net.Listener
	server   *http.Server
	wait     sync.WaitGroup
}

func NewRendezvousMgr(port int, file string) *RendezvousMgr {
	database := db.NewDatabase(file)
	database.Install()

	router := mux.NewRouter()
	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}

	rm := &RendezvousMgr{
		database: database,
		server:   server,
	}

	router.HandleFunc("/{key}", rm.onGet).Methods("GET")
	router.HandleFunc("/{key}", rm.onPut).Methods("PUT")

	return rm
}

// Start the server
func (this *RendezvousMgr) Run() error {
	listener, err := net.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.listener = listener
	this.wait.Add(1)
	go func() {
		this.server.Serve(this.listener)
		this.wait.Done()
	}()
	return nil
}

// Stops the server
func (this *RendezvousMgr) Stop() {
	this.listener.Close()
	this.wait.Wait()
}

func Serve(port int, file string) {
	rendezvous := NewRendezvousMgr(port, file)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, os.Kill)

	fmt.Println("Rendezvous server starting")
	rendezvous.Run()
	fmt.Println("Rendezvous server started")
	<-ch
	fmt.Println("Rendezvous server stopping")
	rendezvous.Stop()
	fmt.Println("Rendezvous server stopped")
}
