package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"h0tb0x/base"
	"h0tb0x/conn"
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
	extHost  string
	extPort  uint16
	rshost   string
	rclient  *rendezvous.Client
	router   *mux.Router
	server   *http.Server
	wait     gosync.WaitGroup
	port     uint16
	listener net.Listener
	mutex    gosync.Locker
	connMgr  conn.ConnMgr
}

type PassportJson struct {
	Passport string `json:"passport"`
}

type IdentityJson struct {
	Id         string `json:"id"`
	Rendezvous string `json:"rendezvous"`
	PublicKey  string `json:"publicKey"`
	Passport   string `json:"passport"`
	Host       string `json:"host"`
	Port       uint16 `json:"port"`
}

type SelfJson struct {
	IdentityJson
	SelfCid string `json:"selfCid"`
}

type FriendJson struct {
	IdentityJson
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
	Cid        string            `json:"cid"`
	From       string            `json:"from"`
	Remove     bool              `json:",omitempty"`
	Attributes map[string]string `json:"attributes"`
}

type WriterJson struct {
	Id     string `json:"id"`
	PubKey string `json:"pubkey"`
}

func NewApiMgr(rshost string, apiPort uint16, data *data.DataMgr, connMgr conn.ConnMgr) *ApiMgr {
	router := mux.NewRouter()
	server := &http.Server{
		Handler: router,
		Addr:    fmt.Sprintf(":%d", apiPort),
	}

	api := &ApiMgr{
		rshost:  rshost,
		rclient: rendezvous.NewClient(connMgr),
		DataMgr: data,
		router:  router,
		server:  server,
		port:    apiPort,
		connMgr: connMgr,
		mutex:   base.NewNoisyLocker(data.Log.Prefix() + "api "),
	}

	sr := router.PathPrefix("/api").Subrouter()

	// get self details
	sr.HandleFunc("/self", api.getSelf).Methods("GET")

	// Friends
	// list friends
	sr.HandleFunc("/friends", api.getFriends).Methods("GET")
	// add friend
	sr.HandleFunc("/friends", api.postFriends).Methods("POST")
	// get friend details
	sr.HandleFunc("/friends/{who}", api.getFriend).Methods("GET")
	// remove friend
	sr.HandleFunc("/friends/{who}", api.deleteFriend).Methods("DELETE")

	// handle invitations
	// list incoming invitation
	sr.HandleFunc("/invites", api.getInvites).Methods("GET")
	// accept/reject invitation
	sr.HandleFunc("/invites", api.postInvite).Methods("POST")
	// send invitation
	sr.HandleFunc("/friends/{who}/invites", api.postFriendInvite).Methods("POST")

	// Collections
	// list collections
	sr.HandleFunc("/collections", api.getCollections).Methods("GET")
	// create collection
	sr.HandleFunc("/collections", api.addCollection).Methods("POST")
	// get collection details
	sr.HandleFunc("/collections/{cid}", api.getCollection).Methods("GET")

	// Collections Writers
	// list collection writers
	sr.HandleFunc("/collections/{cid}/writers", api.getWriters).Methods("GET")
	// get collection writer details
	sr.HandleFunc("/collections/{cid}/writers/{who}", api.getWriter).Methods("GET")
	// add collection writer
	sr.HandleFunc("/collections/{cid}/writers", api.addWriter).Methods("POST")
	// remove collection writer
	sr.HandleFunc("/collections/{cid}/writers/{who}", api.deleteWriter).Methods("DELETE")

	// Collection Objects
	// list collection objects
	sr.HandleFunc("/collections/{cid}/data", api.listData).Methods("GET")
	// get collection object
	sr.HandleFunc("/collections/{cid}/data/{key:.+}", api.getData).Methods("GET")
	// update collection object
	sr.HandleFunc("/collections/{cid}/data/{key:.+}", api.putData).Methods("PUT")
	// add new collection object
	sr.HandleFunc("/collections/{cid}/data/{key:.+}", api.postData).Methods("POST")
	// remove collection objects
	sr.HandleFunc("/collections/{cid}/data/{key:.+}", api.deleteData).Methods("DELETE")

	// serve web app
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("web/app")))
	return api
}

func (this *ApiMgr) runServer() {
	this.server.Serve(this.listener)
	this.wait.Done()
}

func (this *ApiMgr) Start() error {
	this.DataMgr.Start()
	this.CreateSpecialCollection(this.Ident, this.Ident.Fingerprint())
	var err error
	this.listener, err = this.connMgr.Listen("tcp", this.server.Addr)
	if err != nil {
		return err
	}
	this.wait.Add(1)
	go this.runServer()
	return nil
}

func (this *ApiMgr) Stop() {
	this.listener.Close()
	this.wait.Wait()
	this.DataMgr.Stop()
}

func (this *ApiMgr) sendJson(resp http.ResponseWriter, obj interface{}) {
	resp.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(resp)
	enc.Encode(obj)
}

func (this *ApiMgr) sendError(resp http.ResponseWriter, status int, message string) {
	resp.Header().Set("Content-Type", "application/json")
	resp.WriteHeader(status)
	enc := json.NewEncoder(resp)
	enc.Encode(message)
}

func (this *ApiMgr) decodeWho(req *http.Request) *crypto.Digest {
	friendStr := mux.Vars(req)["who"]
	var fp *crypto.Digest
	err := transfer.DecodeString(friendStr, &fp)
	if err != nil {
		return nil
	}
	return fp
}

func (this *ApiMgr) decodeJsonBody(resp http.ResponseWriter, req *http.Request, out interface{}) bool {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		this.sendError(resp, http.StatusBadRequest, "Invalid content type")
		return false
	}
	err := json.NewDecoder(req.Body).Decode(out)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, "Unable to decode JSON")
		return false
	}
	return true
}

func (this *ApiMgr) SetExt(host net.IP, port uint16) {
	this.mutex.Lock()
	this.extHost = host.String()
	this.extPort = port
	this.mutex.Unlock()
	this.Log.Printf("Publishing Rendezvous %s:%d to %s", this.extHost, this.extPort, this.rshost)
	this.rclient.Put("http://"+this.rshost, this.Ident, this.extHost, this.extPort)
}

func (this *ApiMgr) getSelf(resp http.ResponseWriter, req *http.Request) {
	myFp := this.Ident.Public().Fingerprint()
	myCid := crypto.HashOf(myFp, myFp).String()

	this.mutex.Lock()
	passport := transfer.AsString(myFp, this.rshost)
	json := SelfJson{
		IdentityJson: IdentityJson{
			Id:         myFp.String(),
			Rendezvous: this.rshost,
			PublicKey:  transfer.AsString(this.Ident.Public()),
			Passport:   passport,
			Host:       this.extHost,
			Port:       this.extPort,
		},
		SelfCid: myCid,
	}
	this.mutex.Unlock()
	this.sendJson(resp, json)
}

func (this *ApiMgr) populateFriend(json *FriendJson, fp *crypto.Digest, keyBin []byte) {
	myFp := this.Ident.Public().Fingerprint()
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
	// json.SelfCid = crypto.HashOf(fp, fp).String()
	json.SendCid = crypto.HashOf(myFp, fp).String()
	json.RecvCid = crypto.HashOf(fp, myFp).String()
}

func (this *ApiMgr) getFriends(resp http.ResponseWriter, req *http.Request) {
	rows := this.Db.MultiQuery(`
		SELECT fingerprint, rendezvous, public_key, host, port 
		FROM Friend`)
	out := []FriendJson{}
	for rows.Next() {
		var json FriendJson
		var fpb []byte
		var pubkey []byte
		this.Db.Scan(rows, &fpb, &json.Rendezvous, &pubkey, &json.Host, &json.Port)
		var fp *crypto.Digest
		transfer.DecodeBytes(fpb, &fp)
		this.populateFriend(&json, fp, pubkey)
		out = append(out, json)
	}
	this.sendJson(resp, out)
}

func (this *ApiMgr) getFriend(resp http.ResponseWriter, req *http.Request) {
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(resp, http.StatusBadRequest, "Invalid friend id")
		return
	}
	row := this.Db.SingleQuery(`
		SELECT friend_id, rendezvous, public_key, host, port 
		FROM Friend 
		WHERE fingerprint = ?`, fp.Bytes())
	var json FriendJson
	var pubkey []byte
	if !this.Db.MaybeScan(row, &json.Id, &json.Rendezvous, &pubkey, &json.Host, &json.Port) {
		this.sendError(resp, http.StatusNotFound, "Unknown friend")
		return
	}
	this.populateFriend(&json, fp, pubkey)
	this.sendJson(resp, json)
}

func (this *ApiMgr) postFriends(resp http.ResponseWriter, req *http.Request) {
	// Get Json
	var json *PassportJson
	if !this.decodeJsonBody(resp, req, &json) {
		return
	}
	var fp *crypto.Digest
	var rendezvous string
	err := transfer.DecodeString(json.Passport, &fp, &rendezvous)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, "Invalid passport")
		return
	}

	if fp.Equal(this.Ident.Public().Fingerprint()) {
		this.sendError(resp, http.StatusBadRequest, "Self passport")
		return
	}

	// By this point, everything is consistent and fp & rendezvous are valid
	// add/update friend
	this.AddUpdateFriend(fp, rendezvous)
	this.CreateSpecialCollection(this.Ident, fp)

	result := &FriendJson{
		IdentityJson: IdentityJson{
			Rendezvous: rendezvous,
		},
	}
	this.populateFriend(result, fp, nil)
	this.sendJson(resp, result)
}

func (this *ApiMgr) deleteFriend(resp http.ResponseWriter, req *http.Request) {
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(resp, http.StatusBadRequest, "Invalid friend id")
		return
	}
	// TODO: Remove friend collection?
	this.RemoveFriend(fp)
}

func (this *ApiMgr) getCollections(resp http.ResponseWriter, req *http.Request) {
	// FIXME: we call GetOwner() on each record which results in another query
	rows := this.Db.MultiQuery(`
		SELECT t.name 
		FROM Object o
			JOIN Topic t USING (topic_id)
		WHERE o.type = ? AND o.key = ? 
		GROUP BY t.name`,
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
	this.sendJson(resp, out)
}

func (this *ApiMgr) getCollection(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "No such collection")
		return
	}
	json := &CollectionJson{
		Id:    cid,
		Owner: owner.Fingerprint().String(),
	}
	this.sendJson(resp, json)
}

func (this *ApiMgr) addCollection(resp http.ResponseWriter, req *http.Request) {
	cid := this.CreateNewCollection(this.Ident)
	json := &CollectionJson{
		Id:    cid,
		Owner: this.GetOwner(cid).Fingerprint().String(),
	}
	this.sendJson(resp, json)
}

func (this *ApiMgr) getWriters(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	basisRec := this.SyncMgr.Get(sync.RTBasis, cid, "$")
	if basisRec == nil {
		this.sendError(resp, http.StatusNotFound, "No such collection")
		return
	}
	rows := this.Db.MultiQuery(`
		SELECT o.key, o.value 
		FROM Object o
			JOIN Topic t USING (topic_id)
		WHERE t.name = ? AND type = ?`,
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
	this.sendJson(resp, out)
}

func (this *ApiMgr) getWriter(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	who := vars["who"]
	pubKey := this.GetWriter(cid, who)
	if pubKey == nil {
		this.sendError(resp, http.StatusNotFound, "No such writer")
		return
	}
	json := WriterJson{
		Id:     who,
		PubKey: transfer.AsString(pubKey),
	}
	this.sendJson(resp, json)
}

func (this *ApiMgr) addWriter(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	var keystr string
	if !this.decodeJsonBody(resp, req, &keystr) {
		return
	}
	var pubkey *crypto.PublicIdentity
	err := transfer.DecodeString(keystr, &pubkey)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, "Invalid public key")
		return
	}
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "Collection invalid")
		return
	}
	if !owner.Fingerprint().Equal(this.Ident.Public().Fingerprint()) {
		this.sendError(resp, http.StatusUnauthorized, "You are not the owner of the collection")
		return
	}
	this.AddWriter(cid, this.Ident, pubkey)
	this.sendJson(resp, "/collections/"+cid+"/writers/"+pubkey.Fingerprint().String())
}

func (this *ApiMgr) deleteWriter(resp http.ResponseWriter, req *http.Request) {
	// Make sure I'm the owner
	vars := mux.Vars(req)
	cid := vars["cid"]
	who := vars["who"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "Collection invalid")
		return
	}
	if owner.Fingerprint().String() != this.Ident.Public().Fingerprint().String() {
		this.sendError(resp, http.StatusUnauthorized, "You are not the owner of the collection")
		return
	}
	this.RemoveWriter(cid, this.Ident, who)
}

func (this *ApiMgr) getInvites(resp http.ResponseWriter, req *http.Request) {
	// rows := this.Db.MultiQuery(`
	// 	SELECT
	// 		t.name,
	// 		o.key,
	// 		o.value
	// 	FROM
	// 		Object o
	// 		JOIN Attribute a USING(oid)
	// 		JOIN TopicGroup g USING(tid)
	// 		JOIN Topic t USING(tid)
	// 	WHERE
	// 		g.name = ? AND
	// 		a.name = ? AND
	// 		a.value = ?
	// `, cid, sync.RTWriter)
}

func (this *ApiMgr) postInvite(resp http.ResponseWriter, req *http.Request) {
}

func (this *ApiMgr) postFriendInvite(resp http.ResponseWriter, req *http.Request) {
	fp := this.decodeWho(req)
	if fp == nil {
		this.sendError(resp, http.StatusBadRequest, "Invalid friend id")
		return
	}
	var invite InviteJson
	if !this.decodeJsonBody(resp, req, &invite) {
		this.sendError(resp, http.StatusBadRequest, "Invalid invitation request")
		return
	}
	myFp := this.Ident.Public().Fingerprint()
	invite.From = myFp.String()

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(invite)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, err.Error())
		return
	}

	this.Log.Printf("Publish collection: %s to: %s", invite.Cid, fp)

	cid := crypto.HashOf(myFp, fp).String()
	key := crypto.HashOf(invite).String()
	err = this.PutData(cid, key, this.Ident, &buf)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, err.Error())
		return
	}

	this.sendJson(resp, "OK")
}

func (this *ApiMgr) getData(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "Collection invalid")
		return
	}
	key := vars["key"]
	err := this.GetData(cid, key, resp)
	// TODO: Handle each error independently
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, err.Error())
	}
}

func (this *ApiMgr) putData(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "Collection invalid")
		return
	}
	writer := this.GetWriter(cid, this.Ident.Public().Fingerprint().String())
	if writer == nil {
		this.sendError(resp, http.StatusUnauthorized, "You are not a writer for this collection")
		return
	}
	key := vars["key"]
	err := this.PutData(cid, key, this.Ident, req.Body)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, err.Error())
	}
}

func (this *ApiMgr) postData(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	owner := this.GetOwner(cid)
	if owner == nil {
		this.sendError(resp, http.StatusNotFound, "Collection invalid")
		return
	}
	writer := this.GetWriter(cid, this.Ident.Public().Fingerprint().String())
	if writer == nil {
		this.sendError(resp, http.StatusUnauthorized, "You are not a writer for this collection")
		return
	}
	// key := vars["key"]

	// err := this.PutData(cid, key, this.Ident, req.Body)
	// if err != nil {
	// 	this.sendError(resp, http.StatusBadRequest, err.Error())
	// }
	err := req.ParseMultipartForm(10 * 1024 * 1024)
	if err != nil {
		this.sendError(resp, http.StatusBadRequest, err.Error())
		return
	}

	if len(req.MultipartForm.File) != 1 {
		this.sendError(resp, http.StatusBadRequest, "Missing single file for upload")
		return
	}

	for _, fh := range req.MultipartForm.File {
		if len(fh) != 1 {
			this.sendError(resp, http.StatusBadRequest, "Missing single file for upload")
			return
		}

		file, err := fh[0].Open()
		if err != nil {
			this.sendError(resp, http.StatusBadRequest, err.Error())
			return
		}

		key := vars["key"]
		err = this.PutData(cid, key, this.Ident, file)
		if err != nil {
			this.sendError(resp, http.StatusBadRequest, err.Error())
		}
		break
	}
}

func (this *ApiMgr) deleteData(resp http.ResponseWriter, req *http.Request) {
	this.sendError(resp, http.StatusNotImplemented, "Not Implemented")
}

// TODO: Lot of options, limt 1000, starting key, time order, long poll, etc
func (this *ApiMgr) listData(resp http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	cid := vars["cid"]
	rows := this.Db.MultiQuery(`
		SELECT o.key 
		FROM Object o
			JOIN Topic t USING (topic_id)
		WHERE o.type = ? AND t.name = ?
		GROUP BY o.key
		ORDER BY o.key`,
		sync.RTData, cid)
	out := []CollectionItemJson{}
	for rows.Next() {
		var item CollectionItemJson
		this.Db.Scan(rows, &item.Key)
		out = append(out, item)
	}
	this.sendJson(resp, out)
}
