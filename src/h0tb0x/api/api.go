package api

import (
	"encoding/json"
	"io"
	"bytes"
	"fmt"
	"github.com/gorilla/mux"
	"h0tb0x/crypto"
	"h0tb0x/data"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"h0tb0x/transfer"
	"net"
	"net/http"
	"strings"
	gosync "sync"
)

type ApiMgr struct {
	*data.DataMgr
	extHost    string
	extPort    uint16
	rendezvous string
	router     *mux.Router
	server     *http.Server
	wait       gosync.WaitGroup
	port       uint16
	conn       net.Listener
	mutex      gosync.Mutex
}

type SelfJson struct {
	Id         string `json:"id"`
	Rendezvous string `json:"rendezvous"`
	PublicKey  string `json:"publicKey"`
	Passport   string `json:"passport"`
	Host       string `json:"host"`
	Port       uint16 `json:"port"`
	SelfCid    string `json:"selfCid"`
}

type FriendJson struct {
	SelfJson
	SendCid string `json:"sendCid"`
	RecvCid string `json:"recvCid"`
}

type CollectionJson struct {
	Id    string `json:"id"`
	Owner string `json:"owner"`
}

type CollectionItemJson struct {
	Key string `json:"key"`
}

type InviteJson struct {
	Cid    string `json:"cid"`
	Friend string `json:"friend"`
	Remove bool   `json:",omitempty"`
}

type WriterJson struct {
	Id     string `json:"id"`
	PubKey string `json:"pubkey"`
}

func NewApiMgr(rendezvous string, apiPort uint16, data *data.DataMgr) *ApiMgr {
	router := mux.NewRouter()
	server := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%d", apiPort),
	}

	api := &ApiMgr{
		rendezvous: rendezvous,
		DataMgr:    data,
		router:     router,
		server:     server,
		port:       apiPort,
	}

	router.HandleFunc("/api/self", api.getSelf).Methods("GET")
	router.HandleFunc("/api/friends", api.getFriends).Methods("GET")
	router.HandleFunc("/api/friends", api.postFriends).Methods("POST")
	router.HandleFunc("/api/friends/{who}", api.getFriend).Methods("GET")
	router.HandleFunc("/api/friends/{who}", api.putFriend).Methods("PUT")
	router.HandleFunc("/api/friends/{who}", api.deleteFriend).Methods("DELETE")
	router.HandleFunc("/api/collections", api.getCollections).Methods("GET")
	router.HandleFunc("/api/collections", api.addCollection).Methods("POST")
	router.HandleFunc("/api/collections/{cid}", api.getCollection).Methods("GET")
	router.HandleFunc("/api/collections/{cid}/writers", api.getWriters).Methods("GET")
	router.HandleFunc("/api/collections/{cid}/writers/{who}", api.getWriter).Methods("GET")
	router.HandleFunc("/api/collections/{cid}/writers", api.addWriter).Methods("POST")
	router.HandleFunc("/api/collections/{cid}/writers/{who}", api.deleteWriter).Methods("DELETE")
	router.HandleFunc("/api/invites", api.postInvite).Methods("POST")

	router.HandleFunc("/api/collections/{cid}/data", api.listData).Methods("GET")
	router.HandleFunc("/api/collections/{cid}/data/{key:.+}", api.getData).Methods("GET")
	router.HandleFunc("/api/collections/{cid}/data/{key:.+}", api.putData).Methods("PUT")
	router.HandleFunc("/api/collections/{cid}/data/{key:.+}", api.postData).Methods("POST")
	router.HandleFunc("/api/collections/{cid}/data/{key:.+}", api.deleteData).Methods("DELETE")

	router.PathPrefix("/").Handler(http.FileServer(http.Dir("web/app")))
	return api
}

func (this *ApiMgr) runServer() {
	this.server.Serve(this.conn)
	this.wait.Done()
}

func (this *ApiMgr) Run() error {
	this.DataMgr.Run()
	this.CreateSpecialCollection(this.Ident, this.Ident.Fingerprint())
	conn, err := net.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.conn = conn
	this.wait.Add(1)
	go this.runServer()
	return nil
}

func (this *ApiMgr) Stop() {
	this.conn.Close()
	this.wait.Wait()
	this.DataMgr.Stop()
}

func (this *ApiMgr) sendJson(w http.ResponseWriter, obj interface{}) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.Encode(obj)
}

func (this *ApiMgr) sendError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	enc := json.NewEncoder(w)
	enc.Encode(message)
}

func (this *ApiMgr) decodeWho(req *http.Request) *crypto.Digest {
	vars := mux.Vars(req)
	friendStr := vars["who"]
	var fp *crypto.Digest
	err := transfer.DecodeString(friendStr, &fp)
	if err != nil {
		return nil
	}
	return fp
}

func (this *ApiMgr) decodeJsonBody(w http.ResponseWriter, req *http.Request, out interface{}) bool {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		this.sendError(w, http.StatusBadRequest, "Invalid content type")
		return false
	}
	dec := json.NewDecoder(req.Body)
	err := dec.Decode(out)
	if err != nil {
		this.sendError(w, http.StatusBadRequest, "Unable to decode JSON")
		return false
	}
	return true
}

func (this *ApiMgr) SetExt(host net.IP, port uint16) {
	this.mutex.Lock()
	this.extHost = host.String()
	this.extPort = port
	this.mutex.Unlock()
	this.Log.Printf("Publishing Rendezvous %s:%d to %s", this.extHost, this.extPort, this.rendezvous)
	rendezvous.Publish(this.rendezvous, this.Ident, this.extHost, this.extPort)
}

func (this *ApiMgr) getSelf(w http.ResponseWriter, req *http.Request) {
	myFp := this.Ident.Public().Fingerprint()
	myCid := crypto.HashOf(myFp, myFp).String()

	this.mutex.Lock()
	passport := transfer.AsString(myFp, this.rendezvous)
	json := SelfJson{
		Id:         myFp.String(),
		Rendezvous: this.rendezvous,
		PublicKey:  transfer.AsString(this.Ident.Public()),
		Passport:   passport,
		Host:       this.extHost,
		Port:       this.extPort,
		SelfCid:    myCid,
	}
	this.mutex.Unlock()
	this.sendJson(w, json)
}

func (this *ApiMgr) populateFriend(json *FriendJson, myFp, fp *crypto.Digest, keyBin []byte) {
	json.Id = fp.String()
	if keyBin != nil {
		var key *crypto.PublicIdentity
		if transfer.DecodeBytes(keyBin, &key) == nil {
			json.PublicKey = transfer.AsString(key)
		}
	}
	if json.Host == "$" {
		json.Host = ""
	}
	json.Passport = transfer.AsString(fp, json.Rendezvous)
	json.SelfCid = crypto.HashOf(fp, fp).String()
	json.SendCid = crypto.HashOf(myFp, fp).String()
	json.RecvCid = crypto.HashOf(fp, myFp).String()
}

func (this *ApiMgr) getFriends(w http.ResponseWriter, req *http.Request) {
	myFp := this.Ident.Public().Fingerprint()
	rows := this.Db.MultiQuery("SELECT fingerprint, rendezvous, public_key, host, port FROM Friend")
	out := []FriendJson{}
	for rows.Next() {
		var json FriendJson
		var fpb []byte
		var pubkey []byte
		this.Db.Scan(rows, &fpb, &json.Rendezvous, &pubkey, &json.Host, &json.Port)
		var fp *crypto.Digest
		transfer.DecodeBytes(fpb, &fp)
		this.populateFriend(&json, myFp, fp, pubkey)
		out = append(out, json)
	}
	this.sendJson(w, out)
}

func (this *ApiMgr) getFriend(w http.ResponseWriter, req *http.Request) {
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(w, http.StatusBadRequest, "Invalid friend id")
		return
	}
	row := this.Db.SingleQuery("SELECT id, rendezvous, public_key, host, port FROM Friend WHERE fingerprint = ?", fp.Bytes())
	var json FriendJson
	var pubkey []byte
	if !this.Db.MaybeScan(row, &json.Rendezvous, &pubkey, &json.Id, &json.Host, &json.Port) {
		this.sendError(w, http.StatusNotFound, "Unknown friend")
		return
	}
	myFp := this.Ident.Public().Fingerprint()
	this.populateFriend(&json, myFp, fp, pubkey)
	this.sendJson(w, json)
}

func (this *ApiMgr) putFriend(w http.ResponseWriter, req *http.Request) {
	// Get URL version of friend
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(w, http.StatusBadRequest, "Invalid friend id")
		return
	}
	// Get Json
	var json *FriendJson
	if !this.decodeJsonBody(w, req, &json) {
		return
	}
	// If Json has Id, check that it matches
	if json.Id != "" && json.Id != fp.String() {
		this.sendError(w, http.StatusBadRequest, "Friend ID's are inconsistent")
		return
	}
	// Set Id in case it's absent
	json.Id = fp.String()

	// Do the real work
	this.doPutFriend(w, req, &json.SelfJson)
}

func (this *ApiMgr) postFriends(w http.ResponseWriter, req *http.Request) {
	// Grab whole post as a string
	var buf bytes.Buffer
	io.Copy(&buf, req.Body)
	json := &SelfJson{ Passport: string(buf.Bytes()) }

	// Apply it
	this.doPutFriend(w, req, json)
}

func (this *ApiMgr) doPutFriend(w http.ResponseWriter, req *http.Request, json *SelfJson) {
	var fp *crypto.Digest
	var rendezvous string

	err := transfer.DecodeString(json.Passport, &fp, &rendezvous)
	if err == nil {
		// If Passport decodes, validate consistency
		if json.Id != "" && json.Id != fp.String() {
			this.sendError(w, http.StatusBadRequest, "Friend ID's are inconsistent")
			return
		}
		if json.Rendezvous != "" && json.Rendezvous != rendezvous {
			this.sendError(w, http.StatusBadRequest, "Rendezvous addresses are inconsistent")
			return
		}
	} else {
		// If not, try getting the fields from other places
		rendezvous = json.Rendezvous
		err = transfer.DecodeString(json.Id, &fp)
		if err != nil || rendezvous == "" {
			this.sendError(w, http.StatusBadRequest, "Missing required fields")
			return
		}
	}
	// Now validate the public key if it exist
	var pubkey *crypto.PublicIdentity
	if json.PublicKey != "" {
		err = transfer.DecodeString(json.PublicKey, &pubkey)
		if err != nil || pubkey.Fingerprint().String() != fp.String() {
			this.sendError(w, http.StatusBadRequest, "Public key is inconsitent")
			return
		}
	}

	// By this point, everything is consistent and fp & rendezvous are valid
	// add/update friend
	this.AddUpdateFriend(fp, rendezvous)
	this.CreateSpecialCollection(this.Ident, fp)

	// Now check to see if we are getting host and port, in which case update them
	if json.Host != "" && json.Port != 0 {
		// If so, update them
		this.UpdateHostData(fp, json.Host, json.Port)
	}

	// If public key exists, add it
	// TODO: Actually write AddPublicKey and enable code
	//if (pubkey != nil) {
	//	this.AddPublicKey(fp, pubkey)
	//}
}

func (this *ApiMgr) deleteFriend(w http.ResponseWriter, req *http.Request) {
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(w, http.StatusBadRequest, "Invalid friend id")
		return
	}
	// TODO: Remove friend collection?
	this.RemoveFriend(fp)
}

func (this *ApiMgr) getCollections(w http.ResponseWriter, req *http.Request) {
	// FIXME: we call GetOwner() on each record which results in another query
	rows := this.Db.MultiQuery("SELECT topic FROM Object WHERE type = ? AND key = ? GROUP BY topic",
		sync.RTBasis, "$")
	out := []CollectionJson{}
	for rows.Next() {
		var topic string
		this.Db.Scan(rows, &topic)
		json := CollectionJson{
			Id:    topic,
			Owner: this.GetOwner(topic).Fingerprint().String(),
		}
		out = append(out, json)
	}
	this.sendJson(w, out)
}

func (this *ApiMgr) getCollection(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "No such collection")
		return
	}
	json := &CollectionJson{
		Id:    cid,
		Owner: owner.Fingerprint().String(),
	}
	this.sendJson(w, json)
}

func (this *ApiMgr) addCollection(w http.ResponseWriter, req *http.Request) {
	cid := this.CreateNewCollection(this.Ident)
	json := &CollectionJson{
		Id:    cid,
		Owner: this.GetOwner(cid).Fingerprint().String(),
	}
	this.sendJson(w, json)
}

func (this *ApiMgr) getWriters(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	basisRec := this.SyncMgr.Get(sync.RTBasis, cid, "$")
	if basisRec == nil {
		this.sendError(w, http.StatusNotFound, "No such collection")
		return
	}
	rows := this.Db.MultiQuery("SELECT key, value FROM Object WHERE topic = ? AND type = ?",
		cid, sync.RTWriter)
	out := []WriterJson{}
	for rows.Next() {
		var json WriterJson
		var data []byte
		this.Db.Scan(rows, &json.Id, &data)
		if len(data) > 0 {
			json.PubKey = transfer.AsString(data)
			out = append(out, json)
		}
	}
	this.sendJson(w, out)
}

func (this *ApiMgr) getWriter(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	who := vars["who"]
	pubKey := this.GetWriter(cid, who)
	if pubKey == nil {
		this.sendError(w, http.StatusNotFound, "No such writer")
		return
	}
	json := WriterJson{
		Id:     who,
		PubKey: transfer.AsString(pubKey),
	}
	this.sendJson(w, json)
}

func (this *ApiMgr) addWriter(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	var keystr string
	if !this.decodeJsonBody(w, req, &keystr) {
		return
	}
	var pubkey *crypto.PublicIdentity
	err := transfer.DecodeString(keystr, &pubkey)
	if err != nil {
		this.sendError(w, http.StatusBadRequest, "Invalid public key")
		return
	}
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "Collection invalid")
		return
	}
	if owner.Fingerprint().String() != this.Ident.Public().Fingerprint().String() {
		this.sendError(w, http.StatusUnauthorized, "You are not the owner of the collection")
		return
	}
	this.AddWriter(cid, this.Ident, pubkey)
	this.sendJson(w, "/collections/"+cid+"/writers/"+pubkey.Fingerprint().String())
}

func (this *ApiMgr) deleteWriter(w http.ResponseWriter, req *http.Request) {
	// Make sure I'm the owner
	vars := mux.Vars(req)
	cid := vars["cid"]
	who := vars["who"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "Collection invalid")
		return
	}
	if owner.Fingerprint().String() != this.Ident.Public().Fingerprint().String() {
		this.sendError(w, http.StatusUnauthorized, "You are not the owner of the collection")
		return
	}
	this.RemoveWriter(cid, this.Ident, who)
}

func (this *ApiMgr) postInvite(w http.ResponseWriter, req *http.Request) {
	var invite InviteJson
	if !this.decodeJsonBody(w, req, &invite) {
		return
	}
	var fp *crypto.Digest
	err := transfer.DecodeString(invite.Friend, &fp)
	if err != nil {
		this.sendError(w, http.StatusBadRequest, "Invalid friend Id")
		return
	}
	this.Log.Printf("Processing an invite %s:%s:%v", invite.Cid, invite.Friend, invite.Remove)
	if !this.Subscribe(fp, invite.Cid, !invite.Remove) {
		this.sendError(w, http.StatusBadRequest, "Invalid friend Id")
	}
}

func (this *ApiMgr) getData(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "Collection invalid")
		return
	}
	key := vars["key"]
	err := this.GetData(cid, key, w)
	// TODO: Handle each error independently
	if err != nil {
		this.sendError(w, http.StatusBadRequest, err.Error())
	}
}

func (this *ApiMgr) putData(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "Collection invalid")
		return
	}
	writer := this.GetWriter(cid, this.Ident.Public().Fingerprint().String())
	if writer == nil {
		this.sendError(w, http.StatusUnauthorized, "You are not a writer for this collection")
		return
	}
	key := vars["key"]
	err := this.PutData(cid, key, this.Ident, req.Body)
	if err != nil {
		this.sendError(w, http.StatusBadRequest, err.Error())
	}
}

func (this *ApiMgr) postData(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(w, http.StatusNotFound, "Collection invalid")
		return
	}
	writer := this.GetWriter(cid, this.Ident.Public().Fingerprint().String())
	if writer == nil {
		this.sendError(w, http.StatusUnauthorized, "You are not a writer for this collection")
		return
	}
	// key := vars["key"]

	// err := this.PutData(cid, key, this.Ident, req.Body)
	// if err != nil {
	// 	this.sendError(w, http.StatusBadRequest, err.Error())
	// }
	err := req.ParseMultipartForm(10 * 1024 * 1024)
	if err != nil {
		this.sendError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.MultipartForm.File) != 1 {
		this.sendError(w, http.StatusBadRequest, "Missing single file for upload")
		return
	}

	for _, fh := range req.MultipartForm.File {
		if len(fh) != 1 {
			this.sendError(w, http.StatusBadRequest, "Missing single file for upload")
			return
		}

		file, err := fh[0].Open()
		if err != nil {
			this.sendError(w, http.StatusBadRequest, err.Error())
			return
		}

		key := vars["key"]
		err = this.PutData(cid, key, this.Ident, file)
		if err != nil {
			this.sendError(w, http.StatusBadRequest, err.Error())
		}
		break
	}
}

func (this *ApiMgr) deleteData(w http.ResponseWriter, req *http.Request) {
	this.sendError(w, http.StatusNotImplemented, "Not Implemented")
}

// TODO: Lot of options, limt 1000, starting key, time order, long poll, etc
func (this *ApiMgr) listData(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	rows := this.Db.MultiQuery(`
		SELECT key FROM Object
		WHERE type = ? AND topic = ?
		GROUP BY key
		ORDER BY key`,
		sync.RTData, cid)
	out := []CollectionItemJson{}
	for rows.Next() {
		var item CollectionItemJson
		this.Db.Scan(rows, &item.Key)
		out = append(out, item)
	}
	this.sendJson(w, out)
}
