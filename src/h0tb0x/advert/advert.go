package advert

import (
	"io"
	"bytes"
	"fmt"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/sync"
	"h0tb0x/transfer"
)

const costInf = 1000

type advert struct {
	Cost      int // The cost of this advert
	Timestamp int // The timestamp for this adver
}

type destInfo struct {
	RefCount  int            // Do I want to track this data personally?
	Cost      int            // My current 'best' cost
	Timestamp int            // My current timestamp
	Downhill  int            // Where I got my info from, -1 if no downhill
	Routing   bool           // Am I routing between two friends?
	NeedsReq  bool           // Do I need to do a request
	Friends   map[int]advert // What my friends think, empty if they don't care
}

type requestMsg struct {
	Dest      string  // The data/advert updata I seek
	Timestamp int     // The minimum timestamp I will accept
	Full      bool    // Do I want the actual data, or just new advert	
}

func (di *destInfo) acceptAdvert(src int, a advert) {
	di.Cost = a.Cost + 1
	if (di.Cost > costInf) { di.Cost = costInf }
	di.Timestamp = a.Timestamp
	di.Downhill = src
}

func (di *destInfo) check() {
	// See if I'm forced to update
	if di.Downhill >= 0 { // If I have a downhill
		dh, exists := di.Friends[di.Downhill] // Get it's info
		if !exists || dh.Cost == costInf {
			// Downhill is gone, go to inf
			di.Cost = costInf
			di.Downhill = -1
			di.Timestamp++
		} else if dh.Timestamp > di.Timestamp && dh.Cost+1 != di.Cost {
			// Down hill changed, propagate
			di.acceptAdvert(di.Downhill, dh)
		}
	}
	// See if anything is 'better'
	for src, a := range di.Friends {
		if (a.Cost+1 < di.Cost) && a.Timestamp >= di.Timestamp {
			di.acceptAdvert(src, a)
		}
	}
	// See if I should schedule a request, or act as a router
	for _, a := range di.Friends {
		if a.Cost+1 < di.Cost {
			di.NeedsReq = true
		}
		if a.Cost >= di.Cost+1 {
			di.Routing = true
		}
	}
	return
}

// Hold long distance advert subsystem
type AdvertMgr struct {
	*sync.SyncMgr
}

func NewAdvertMgr(sync *sync.SyncMgr) *AdvertMgr {
	return &AdvertMgr{SyncMgr: sync}
}

func (this *AdvertMgr) Start() {
	this.AddHandler(link.ServiceAdvert, this.onRequest)
	this.SetSink(sync.RTAdvert, this.onData)
	this.SyncMgr.Start()
}

func (this *AdvertMgr) Stop() {
	this.SyncMgr.Stop()
}

func (this *AdvertMgr) getDestInfo(key string) *destInfo {
	var di *destInfo
	row := this.Db.SingleQuery("SELECT data FROM Storage WHERE key=?", key)
	var data []byte
	if this.Db.MaybeScan(row, &data) {
		err := transfer.DecodeBytes(data, &di)
		if err != nil {
			panic(err)
		}
		return di
	}
	di = &destInfo{
		Cost:      costInf,
		Timestamp: -1,
		Downhill:  -1,
		Friends:   make(map[int]advert),
	}
	return di
}

func (this *AdvertMgr) putDestInfo(key string, di *destInfo) {
	this.Db.Exec("DELETE FROM Storage WHERE Key = ?", key) // Delete prior record
	if di.RefCount == 0 && len(di.Friends) == 0 {
		return // If record serves no purpose, leave deleted
	}
	data, err := transfer.EncodeBytes(di)
	if err != nil {
		panic(err)
	}
	this.Db.Exec("INSERT INTO Storage (key, needs_req, data) VALUES (?, ?, ?)",
		key, di.NeedsReq, data)
}

func (this *AdvertMgr) updateAdvert(key string, di *destInfo) {
	var a advert
	var data []byte
	var err error
	this.Log.Printf("Updating outgoing advert")
	a.Cost = di.Cost
	a.Timestamp = di.Timestamp
	if (di.RefCount > 0 || di.Routing) {
		this.Log.Printf("Cost: %d, timestamp: %d", a.Cost, a.Timestamp)
		data, err = transfer.EncodeBytes(a)
		if err != nil {
			panic(err)
		}
	} else {
		this.Log.Printf("Making nil advert")
		data = []byte{}
	}
	rout := &sync.Record{
		RecordType: sync.RTAdvert,
		Topic:      this.ProfileTopic(),
		Key:        key,
		Value:      data,
		Author:     "$",
	}
	rcur := this.Get(sync.RTAdvert, this.ProfileTopic(), key)
	if rcur == nil && len(rout.Value) == 0 {
		this.Log.Printf("Both empty, no change")
		return // Replacing empty with empty, no point
	}
	if rcur != nil && bytes.Equal(rcur.Value, rout.Value) {
		this.Log.Printf("Both equal, no change")
		return // No change, don't bother poking the sync layer
	}
	this.Put(rout)
}

// Handles learning data from our friend
func (this *AdvertMgr) onData(who int, remote *crypto.Digest, rec *sync.Record) {
	this.Log.Printf("Getting an incoming advert from %d:", who);
	di := this.getDestInfo(rec.Key)
	var a advert
	if len(rec.Value) != 0 { // If it's a real value
		err := transfer.DecodeBytes(rec.Value, &a)
		if err != nil {
			this.Log.Printf("Got junk advert from friend %d:%s", who, remote.String())
			return
		}
		this.Log.Printf("  timestamp = %d, cost = %d", a.Timestamp, a.Cost)
		di.Friends[who] = a // Add the advert
	} else {
		this.Log.Printf("  empty advert");
		delete(di.Friends, who) // Otherwise, remove advert
	}
	di.check()
	this.updateAdvert(rec.Key, di)
	this.putDestInfo(rec.Key, di)
}

// Handles ref counting of concern
func (this *AdvertMgr) IncRef(key string) {
	di := this.getDestInfo(key)
	di.RefCount++
	this.updateAdvert(key, di)
	this.putDestInfo(key, di)
}

// Handles ref counting of concern
func (this *AdvertMgr) DecRef(key string) {
	di := this.getDestInfo(key)
	di.RefCount--
	this.updateAdvert(key, di)
	this.putDestInfo(key, di)
}

// Handles info about local copy
func (this *AdvertMgr) HasCopy(key string) {
	this.Log.Printf("Have a copy of %s", key)
	di := this.getDestInfo(key)
	di.RefCount++
	di.Cost = 0
	di.Downhill = -1
	di.Timestamp = 0
	for _, a := range di.Friends {
		if a.Timestamp > di.Timestamp {
			di.Timestamp = a.Timestamp
		}
	}
	this.updateAdvert(key, di)
	this.putDestInfo(key, di)
}

// Handles info about local copy
func (this *AdvertMgr) NoCopy(key string) {
	di := this.getDestInfo(key)
	di.RefCount--
	di.Cost = costInf
	di.Downhill = -1
	di.check()
	this.updateAdvert(key, di)
	this.putDestInfo(key, di)
}

// Requests coming in from link layer
func (this *AdvertMgr) onRequest(remote int, fp *crypto.Digest, in io.Reader, out io.Writer) error {
	this.Log.Printf("Getting a request")
	var rm *requestMsg 
	err := transfer.Decode(in, &rm) // Get the request
	if (err != nil) { 
		this.Log.Printf("Invalid request")
		return err 
	}  
	a, resp := this.reqInner(rm.Dest, rm.Timestamp, rm.Full)
	err = transfer.Encode(out, &a)
	if (err == nil && resp != nil) {
		io.Copy(out, resp)
	}
	return err
}

func (this *AdvertMgr) reqInner(key string, timestamp int, getData bool) (a *advert, resp io.Reader) {
	di := this.getDestInfo(key)     // Get the record
	if (di.Cost == costInf) {
		this.Log.Printf("No known source for %s, return failue", key)
		// If I can't find the data, let the user know
		a = &advert{ Cost: costInf, Timestamp: timestamp}
		return
	}
	if (di.Cost == 0) {
		// If I am a source
		this.Log.Printf("I am a source for %s", key)
		if (di.Timestamp < timestamp) {
			this.Log.Printf("Moving timestamp forward")
			di.Timestamp = timestamp// Update timestamp as needed
			this.putDestInfo(key, di) // Write new state
		}
		a = &advert{ Cost: 0, Timestamp: di.Timestamp }
		// if (getData) TODO: Get actual data, put it in resp
		return
	}
	// Looks like I need to forward
	// Find the downhill guy
	who := di.Downhill
	var ibuf bytes.Buffer
	err := transfer.Encode(&ibuf, &requestMsg{ Dest: key, Timestamp: timestamp, Full: getData})
	if (err != nil) { panic(err) }
	this.Log.Printf("Not a source, forwarding request to %d", who)
	resp, err = this.SendHalf(link.ServiceAdvert, who, &ibuf) 
	if (err == nil) {
		this.Log.Printf("Got response, reading advert");
		err = transfer.Decode(resp, &a)
		this.Log.Printf("Decoded")
	}
	if (err != nil) {
		this.Log.Printf("Got error doing forward")
		// The downhill failed me, I need to update my state of the world
		di.Timestamp = di.Timestamp + 1
		di.Downhill = -1
		di.Cost = costInf
		di.check()
		this.putDestInfo(key, di)
		a = &advert{ Cost: di.Cost, Timestamp: di.Timestamp }
		// TODO: Should a rerun the request if I'm non-inf?
	}
	return
}	

// Request data from some (possibly) remote node, write to writer
// Return error if not found (or didn't complete writing correctly)
func (this *AdvertMgr) Request(key string, out io.Writer) error {
	a, resp := this.reqInner(key, 0, true)
	if (a.Cost == costInf || resp == nil) {
		this.Log.Printf("Unable to get data, unknown route")
		return fmt.Errorf("No route")
	}
	_, err := io.Copy(out, resp)
	return err
}
