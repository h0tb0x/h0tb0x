package meta

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"h0tb0x/crypto"
	"h0tb0x/sync"
	"h0tb0x/transfer"
	"io"
)

type MetaMgrCallback func(string, string, []byte, string, bool)

type MetaMgr struct {
	*sync.SyncMgr
	callbacks []MetaMgrCallback
}

type collectionBasis struct {
	Uniq  *crypto.Digest
	Owner *crypto.PublicIdentity
}

func signRecord(rec *sync.Record, writer *crypto.SecretIdentity) {
	hash := crypto.HashOf(rec.RecordType, rec.Topic, rec.Key, rec.Value, rec.Priority)
	sig := writer.Sign(hash)
	rec.Author = writer.Fingerprint().String()
	rec.Signature = transfer.AsBytes(sig)
}

func verifyRecord(rec *sync.Record, writer *crypto.PublicIdentity) bool {
	hash := crypto.HashOf(rec.RecordType, rec.Topic, rec.Key, rec.Value, rec.Priority)
	var sig *crypto.Signature
	err := transfer.DecodeBytes(rec.Signature, &sig)
	if err != nil {
		panic(err)
	}
	return writer.Verify(hash, sig)
}

// Construct a new MetaMgr
func NewMetaMgr(sync *sync.SyncMgr) *MetaMgr {
	return &MetaMgr{SyncMgr: sync}
}

func (this *MetaMgr) AddCallback(callback MetaMgrCallback) {
	this.callbacks = append(this.callbacks, callback)
}

// Run the MetaMgr, starts sync as well
func (this *MetaMgr) Start() {
	this.SyncMgr.SetSink(sync.RTBasis, this.onBasis)
	this.SyncMgr.SetSink(sync.RTWriter, this.onWriter)
	this.SyncMgr.SetSink(sync.RTData, this.onData)
	this.SyncMgr.Start()
	this.CreateSpecialCollection(this.Ident, this.Ident.Fingerprint())
	this.CreateSpecialCollection(this.Ident, crypto.HashOf("profile"))
}

// Stop the MetaMgr, stops sync as well
func (this *MetaMgr) Stop() {
	this.SyncMgr.Stop()
}

// Creates a new collection with a specific owner, no writers (except owner), and no data.
func (this *MetaMgr) CreateNewCollection(owner *crypto.SecretIdentity) (cid string) {
	uniq := make([]byte, 16)
	io.ReadFull(rand.Reader, uniq)
	return this.CreateSpecialCollection(owner, crypto.HashOf(uniq))
}

// Creates a collection with a specific uniq element, used for friend collections
func (this *MetaMgr) CreateSpecialCollection(owner *crypto.SecretIdentity, uniq *crypto.Digest) (cid string) {
	pubkey := owner.Public()

	cb := &collectionBasis{
		Uniq:  uniq,
		Owner: pubkey,
	}
	cid = crypto.HashOf(cb.Owner.Fingerprint(), cb.Uniq).String()

	existing := this.SyncMgr.Get(sync.RTBasis, cid, "$")
	if existing != nil {
		return cid
	}

	basis := &sync.Record{
		RecordType: sync.RTBasis,
		Topic:      cid,
		Key:        "$",
		Author:     pubkey.Fingerprint().String(),
		Value:      transfer.AsBytes(cb),
	}
	signRecord(basis, owner)
	this.SyncMgr.Put(basis)

	owr := &sync.Record{
		RecordType: sync.RTWriter,
		Topic:      cid,
		Key:        pubkey.Fingerprint().String(),
		Value:      transfer.AsBytes(pubkey),
	}
	signRecord(owr, owner)
	this.SyncMgr.Put(owr)

	return
}

// Removes a writer by key
func (this *MetaMgr) RemoveWriter(cid string, owner *crypto.SecretIdentity, key string) {
	rec := this.SyncMgr.Get(sync.RTWriter, cid, key)
	if rec == nil {
		return
	}
	if len(rec.Value) == 0 {
		return
	}
	rec.Value = []byte{}
	rec.Priority = rec.Priority + 1
	signRecord(rec, owner)
	this.SyncMgr.Put(rec)
}

// Adds/Removes a writer.  If a writer is removed, records from that writer may be out of sync
func (this *MetaMgr) AddWriter(cid string, owner *crypto.SecretIdentity, writer *crypto.PublicIdentity) {
	// Setup some variables
	key := writer.Fingerprint().String()
	priority := 0 // Default priority is 0, otherwise, current + 1
	newValue := transfer.AsBytes(writer)

	// Check current writer state
	rec := this.SyncMgr.Get(sync.RTWriter, cid, key)
	if rec != nil {
		// If no change, leave alone
		if bytes.Equal(rec.Value, newValue) {
			return
		}
		// Otherwise, override old priority
		priority = rec.Priority + 1
	}

	// Add the record
	wrr := &sync.Record{
		RecordType: sync.RTWriter,
		Topic:      cid,
		Key:        key,
		Priority:   priority,
		Value:      newValue,
	}
	signRecord(wrr, owner)
	this.SyncMgr.Put(wrr)
}

func (this *MetaMgr) doCallbacks(cid string, key string, data []byte, author string, isUp bool) {
	for _, cb := range this.callbacks {
		cb(cid, key, data, author, isUp)
	}
}

// Writes meta-data, may fail if cid does not exist, or writer is not allowed
func (this *MetaMgr) Put(cid string, writer *crypto.SecretIdentity, key string, data []byte) error {
	basisRec := this.SyncMgr.Get(sync.RTBasis, cid, "$")
	if basisRec == nil {
		return fmt.Errorf("Unable to write to cid '%s', doesn't exist!", cid)
	}
	myFingerprint := writer.Public().Fingerprint().String()
	myPerm := this.SyncMgr.Get(sync.RTWriter, cid, myFingerprint)
	if myPerm == nil {
		return fmt.Errorf("Unable to write to cid '%s', '%s' doesn't have permission", cid, myFingerprint)
	}
	old := this.SyncMgr.Get(sync.RTData, cid, key)
	priority := 0
	if old != nil {
		priority = old.Priority + 1
	}
	rec := &sync.Record{
		RecordType: sync.RTData,
		Topic:      cid,
		Key:        key,
		Value:      data,
		Priority:   priority,
		Author:     myFingerprint,
	}
	signRecord(rec, writer)
	if old != nil {
		this.doCallbacks(cid, key, old.Value, myFingerprint, false)
	}
	this.SyncMgr.Put(rec)
	this.doCallbacks(cid, key, data, myFingerprint, true)
	return nil
}

// Gets the entry (if any) with the largest priority, nil if no entry
func (this *MetaMgr) Get(cid string, key string) []byte {
	// Grab Lock
	rec := this.SyncMgr.Get(sync.RTData, cid, key)
	// Track
	// Drop Lock
	if rec == nil {
		return nil
	}
	return rec.Value
}

// Checks if basis record is valid, if so, returns owner ID, otherwise nil
func (this *MetaMgr) decodeBasis(rec *sync.Record, check bool) *crypto.PublicIdentity {
	var cb *collectionBasis
	if rec == nil {
		return nil
	}
	err := transfer.DecodeBytes(rec.Value, &cb)
	if err != nil {
		this.Log.Printf("Unable to decode basis: %s", err)
		return nil
	}
	if check {
		cid := crypto.HashOf(cb.Owner.Fingerprint(), cb.Uniq).String()
		if cid != rec.Topic {
			this.Log.Printf("Basis hash mismatch: %s vs %s", cid, rec.Topic)
			return nil
		}
	}
	return cb.Owner
}

func (this *MetaMgr) verifyUpdate(rec *sync.Record, signer *crypto.PublicIdentity) {
	// Get the current version of this record
	curRec := this.SyncMgr.GetAuthor(rec.RecordType, rec.Topic, rec.Key, rec.Author)
	if curRec != nil {
		// If my record is newer, ignore incoming
		if curRec.Priority > rec.Priority {
			return
		}
		// If my records has identical priority and bigger data or equal data ignore
		if curRec.Priority == rec.Priority &&
			bytes.Compare(curRec.Value, rec.Value) >= 0 {
			this.Log.Printf("Getting dup of priority with same value, ignoring")
			return
		}
	}
	// Validate signature
	if !verifyRecord(rec, signer) {
		return
	}

	// All checks passed, Forward to everyone and store!
	// Grab Lock
	if rec.RecordType == sync.RTData && curRec != nil {
		this.doCallbacks(rec.Topic, rec.Key, curRec.Value, rec.Author, false)
	}
	this.SyncMgr.Put(rec)
	if rec.RecordType == sync.RTData {
		this.doCallbacks(rec.Topic, rec.Key, rec.Value, rec.Author, true)
	}
	// Drop Lock
}

func (this *MetaMgr) onBasis(who int, remote *crypto.Digest, rec *sync.Record) {
	this.Log.Printf("Processing Basis")

	curRec := this.SyncMgr.Get(sync.RTBasis, rec.Topic, "$")
	if curRec != nil {
		this.Log.Printf("Getting a redundant basis, ignoring")
		return
	}

	owner := this.decodeBasis(rec, true)
	if owner == nil {
		this.Log.Printf("Basis failed to verify, ignoring")
		return
	}

	this.SyncMgr.Put(rec)
}

func (this *MetaMgr) onWriter(who int, remote *crypto.Digest, rec *sync.Record) {
	this.Log.Printf("Processing Writer")

	// Verify the basis
	basisRec := this.SyncMgr.Get(sync.RTBasis, rec.Topic, "$")
	owner := this.decodeBasis(basisRec, false)
	if owner == nil {
		this.Log.Printf("Getting record before basis, ignoring")
		return
	}

	// Validate the incoming writer record is valid
	var checkit *crypto.PublicIdentity
	err := transfer.DecodeBytes(rec.Value, &checkit)
	if err != nil {
		this.Log.Printf("Writer record is misformed: %s", err)
		return
	}
	if checkit.Fingerprint().String() != rec.Key {
		this.Log.Printf("Writer record is misformed, key != hash")
		return
	}
	this.verifyUpdate(rec, owner)
}

func (this *MetaMgr) onData(who int, remote *crypto.Digest, rec *sync.Record) {
	this.Log.Printf("Processing Data")

	// Verify the basis
	basisRec := this.SyncMgr.Get(sync.RTBasis, rec.Topic, "$")
	owner := this.decodeBasis(basisRec, false)
	if owner == nil {
		this.Log.Printf("Getting record before basis, ignoring")
		return
	}

	// Otherwise, get the writer data
	var signer *crypto.PublicIdentity
	writerRec := this.SyncMgr.Get(sync.RTWriter, rec.Topic, rec.Author)
	if writerRec == nil {
		this.Log.Printf("Attempt to write by unauthorized writer, ignoring")
		return
	}
	err := transfer.DecodeBytes(writerRec.Value, &signer)
	if err != nil {
		this.Log.Printf("Error decoding writer record: %s", err)
		return
	}

	// Process the data record
	this.verifyUpdate(rec, signer)
}

func (this *MetaMgr) GetOwner(cid string) *crypto.PublicIdentity {
	basisRec := this.SyncMgr.Get(sync.RTBasis, cid, "$")
	if basisRec == nil {
		return nil
	}
	owner := this.decodeBasis(basisRec, false)
	return owner
}

func (this *MetaMgr) GetWriter(cid string, writer string) *crypto.PublicIdentity {
	writerRec := this.SyncMgr.Get(sync.RTWriter, cid, writer)
	if writerRec == nil {
		return nil
	}
	var writerKey *crypto.PublicIdentity
	err := transfer.DecodeBytes(writerRec.Value, &writerKey)
	if err != nil {
		return nil
	}
	return writerKey
}

// TODO: Add lots more functions, such as 'ListWriters' and 'GetAll', etc.
// Also, I need to have a way to delete collections
