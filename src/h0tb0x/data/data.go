package data

import (
	"bytes"
	"fmt"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/sync"
	"h0tb0x/transfer"
	"io"
	"math/rand"
	"os"
	"path"
	gosync "sync"
	"time"
)

const (
	DSNotReady = 0
	DSReady    = 1
	DSLocal    = 2
)

// All the state for a data object
type DataObj struct {
	mgr         *DataMgr       // The data manager, not serialized
	Key         string         // The key of this DataObj
	Holds       int            // How many 'holds' do a have (uploads/downloads/etc)
	State       int            // What is my current state
	Downloading bool           // Am I downloading?
	Tracking    map[string]int // For each topic, how many object in that topic use this
}

// The DataMgr
type DataMgr struct {
	*meta.MetaMgr
	rand       *rand.Rand
	dir        string
	incoming   string
	lock       gosync.Mutex
	download   gosync.Cond
	goRoutines gosync.WaitGroup
	isClosing  bool
}

// Add 'incoming advert'
func (this *DataMgr) addAdvert(who int, topic string, key string) {
	this.Db.Exec("INSERT OR IGNORE INTO Advert (key, topic, friend_id) VALUES (?, ?, ?)",
		key, topic, who)
}

// Del 'incoming advert'
func (this *DataMgr) delAdvert(who int, topic string, key string) {
	this.Db.Exec("DELETE FROM Advert WHERE key=? AND topic=? AND friend_id=?",
		key, topic, who)
}

// Check if have any incoming adverts
func (this *DataMgr) anyAdverts(key string) bool {
	row := this.Db.SingleQuery("SELECT COUNT(*) FROM Advert WHERE key=?", key)
	var count int
	this.Db.Scan(row, &count)
	return count > 0
}

// Get everyone advertizing a blob
func (this *DataMgr) allAdverts(key string) []int {
	rows := this.Db.MultiQuery("SELECT friend_id FROM Advert WHERE key=? GROUP BY friend_id", key)
	out := []int{}
	for rows.Next() {
		var friend int
		this.Db.Scan(rows, &friend)
		out = append(out, friend)
	}
	return out
}

// Change state of outgoing advert
func (this *DataMgr) advertize(topic string, key string, up bool) {
	this.Log.Printf("Doing advertize, topic = %s, key = %s, up = %d", topic, key, up)
	rec := &sync.Record{
		RecordType: sync.RTAdvert,
		Topic:      topic,
		Key:        key,
		Author:     "$",
	}
	if up {
		rec.Value = []byte{1}
	} else {
		rec.Value = []byte{}
	}
	this.SyncMgr.Put(rec)
}

// Return a deserialized object, or nil if none
func (this *DataMgr) maybeGetObj(key string) *DataObj {
	//this.Log.Printf("Getting Object: %s", key)
	row := this.Db.SingleQuery("SELECT data FROM Blob WHERE key=?", key)
	var data []byte
	if this.Db.MaybeScan(row, &data) {
		var obj *DataObj
		//this.Log.Printf("Found Object: %s, data = %v", key, data)
		err := transfer.DecodeBytes(data, &obj)
		if err != nil {
			panic(err)
		}
		obj.mgr = this
		return obj
	}
	return nil
}

// Return or create an object
func (this *DataMgr) getObj(key string) *DataObj {
	obj := this.maybeGetObj(key)
	if obj == nil {
		obj = &DataObj{
			mgr:      this,
			State:    DSNotReady,
			Key:      key,
			Tracking: make(map[string]int),
		}
	}
	return obj
}

// 'Writes' an object, this means deleting it (and associated data) if it's dead
func (this *DataMgr) writeObj(obj *DataObj) {
	//this.Log.Printf("Writing object: %v", obj)
	if len(obj.Tracking) == 0 && obj.Holds == 0 {
		if obj.State == DSLocal {
			name := path.Join(this.dir, obj.Key)
			os.Remove(name)
		}
		//this.Log.Printf("Deleting")
		this.Db.Exec("DELETE FROM Blob WHERE Key = ?", obj.Key)
	} else {
		//this.Log.Printf("Deleting and Storing")
		this.Db.Exec("DELETE FROM Blob WHERE Key = ?", obj.Key)
		data, err := transfer.EncodeBytes(obj)
		if err != nil {
			panic(err)
		}
		//this.Log.Printf("Putting data as: %v", data)
		this.Db.Exec("INSERT INTO Blob (key, needs_download, data) VALUES (?, ?, ?)",
			obj.Key, obj.State == DSReady && !obj.Downloading, data)
	}
}

func (this *DataMgr) onAdvert(who int, fp *crypto.Digest, rec *sync.Record) {
	this.lock.Lock()
	defer this.lock.Unlock()
	obj := this.maybeGetObj(rec.Key)
	if len(rec.Value) > 0 {
		this.addAdvert(who, rec.Topic, rec.Key)
		if obj != nil && obj.State == DSNotReady {
			obj.State = DSReady
			this.download.Broadcast()
		}
	} else {
		this.delAdvert(who, rec.Topic, rec.Key)
		if obj != nil && obj.State == DSReady && !this.anyAdverts(rec.Key) {
			obj.State = DSNotReady
		}
	}
	if obj != nil {
		this.writeObj(obj)
	}
}

func (this *DataObj) metaUp(topic string) {
	this.mgr.Log.Printf("onMetaUp: Obj %s, topic = %s, state = %d", this.Key, topic, this.State)
	this.Tracking[topic]++
	if this.State == DSLocal && this.Tracking[topic] == 1 {
		this.mgr.advertize(topic, this.Key, true)
	}
	if this.State == DSNotReady && this.mgr.anyAdverts(this.Key) {
		this.State = DSReady
		this.mgr.download.Broadcast()
	}
}

func (this *DataObj) metaDown(topic string) {
	this.mgr.Log.Printf("Obj (%s).onMetaDown(%s)", this.Key, topic)
	this.Tracking[topic]--
	if this.Tracking[topic] == 0 {
		delete(this.Tracking, topic)
		if this.State == DSLocal {
			this.mgr.advertize(topic, this.Key, false)
		}
	}
}

func (this *DataMgr) onMeta(topic string, key string, data []byte, fp string, isUp bool) {
	var objHash *crypto.Digest
	err := transfer.DecodeBytes(data, &objHash)
	if err != nil {
		this.Log.Printf("Unable to decode meta-data value")
		return
	}
	objKey := objHash.String()
	this.lock.Lock()
	defer this.lock.Unlock()
	obj := this.getObj(objKey)
	if isUp {
		obj.metaUp(topic)
	} else {
		obj.metaDown(topic)
	}
	this.writeObj(obj)
}

func (this *DataObj) newFile(file string) {
	if this.State == DSLocal {
		return
	}
	newname := path.Join(this.mgr.dir, this.Key)
	os.Rename(file, newname)
	this.mgr.Log.Printf("Setting state of %s to Local", this.Key)
	this.State = DSLocal
	for topic, _ := range this.Tracking {
		this.mgr.advertize(topic, this.Key, true)
	}
}

func (this *DataObj) startDownload() {
	this.Holds++
	this.Downloading = true
}

func (this *DataObj) finishDownload(file string, worked bool) {
	this.Downloading = false
	this.Holds--
	if worked {
		this.newFile(file)
	} else {
		os.Remove(file)
	}
}

// Generates a new DataMgr with a storage area
func NewDataMgr(dir string, themeta *meta.MetaMgr) *DataMgr {
	incoming := path.Join(dir, "incoming")
	err := os.MkdirAll(incoming, 0700)
	if err != nil {
		panic(err)
	}
	dm := &DataMgr{
		MetaMgr:  themeta,
		rand:     rand.New(rand.NewSource(time.Now().UnixNano())),
		dir:      dir,
		incoming: incoming,
	}
	dm.download.L = &dm.lock
	dm.SetSink(sync.RTAdvert, dm.onAdvert)
	dm.AddHandler(link.ServiceData, dm.onDataGet)
	dm.AddCallback(dm.onMeta)
	return dm
}

func (this *DataMgr) Start() {
	this.MetaMgr.Start()
	this.goRoutines.Add(1)
	go this.downloadLoop()
}

func (this *DataMgr) Stop() {
	this.lock.Lock()
	this.isClosing = true
	this.download.Broadcast()
	this.lock.Unlock()
	this.goRoutines.Wait()
	this.MetaMgr.Stop()
}

func (this *DataMgr) onDataGet(who int, ident *crypto.Digest, in io.Reader, out io.Writer) error {
	// TODO: Fix security hole where people can determine which blobs I have
	var key string
	err := transfer.Decode(in, &key)
	if err != nil {
		return err
	}

	this.lock.Lock()
	obj := this.maybeGetObj(key)
	if obj == nil || obj.State != DSLocal {
		this.lock.Unlock()
		return fmt.Errorf("Unknown blob: %s", key)
	}
	obj.Holds++
	this.writeObj(obj)
	this.lock.Unlock()

	name := path.Join(this.dir, key)
	file, err := os.Open(name)
	if err == nil {
		_, err = io.Copy(out, file)
	}

	this.lock.Lock()
	obj = this.getObj(key)
	obj.Holds--
	this.writeObj(obj)
	this.lock.Unlock()

	return err
}

func (this *DataMgr) downloadLoop() {
	this.Log.Printf("Entering download loop")
	this.lock.Lock() // Lock is held *except* when doing remote calls & sleeping
	// While I'm not closing
	for !this.isClosing {
		this.SyncMgr.Log.Printf("Looking things to download\n")
		// Get a object to download
		var key string
		row := this.Db.SingleQuery("SELECT key FROM Blob WHERE needs_download = 1")
		if !this.Db.MaybeScan(row, &key) {
			this.Log.Printf("Nothing to download, sleeping\n")
			this.download.Wait()
			continue
		}
		friends := this.allAdverts(key)
		if len(friends) == 0 {
			this.Log.Printf("Strangely got a download where len(friends) = 0")
			this.Db.Exec("UPDATE Blob SET needs_download = 0 WHERE key = ?", key)
			continue
		}
		friend := friends[rand.Intn(len(friends))]
		obj := this.getObj(key)
		obj.startDownload()
		this.writeObj(obj)
		this.lock.Unlock()
		this.Log.Printf("Doing a download of %s from %d!\n", key, friend)

		var send_buf bytes.Buffer
		transfer.Encode(&send_buf, key)
		tmppath := path.Join(this.incoming, crypto.RandomString())
		file, _ := os.Create(tmppath)
		hasher := crypto.NewHasher()
		both := io.MultiWriter(file, hasher)

		tryTime := time.Now()
		err := this.Send(link.ServiceData, friend, &send_buf, both)
		if err != nil && time.Now().Sub(tryTime) < 5*time.Second {
			// TODO: Make this not suck
			time.Sleep(5 * time.Second)
		}
		if err != nil {
			this.Log.Printf("Download failed: %s", err)
		} else {
			this.Log.Printf("Download worked!")
		}
		this.lock.Lock()
		obj = this.getObj(key)
		obj.finishDownload(tmppath, err == nil)
		this.writeObj(obj)
	}
	this.lock.Unlock()
	this.goRoutines.Done()
}

// Reads from io.Reader and generates a new object, put it to the meta-data layer
func (this *DataMgr) PutData(topic string, key string, writer *crypto.SecretIdentity, stream io.Reader) error {
	this.Log.Printf("Putting data: %s, %s", topic, key)
	// Write the object to disk
	tmppath := path.Join(this.incoming, crypto.RandomString())
	file, err := os.Create(tmppath)
	if err != nil {
		return err
	}
	hasher := crypto.NewHasher()
	both := io.MultiWriter(file, hasher)
	_, err = io.Copy(both, stream)
	if err != nil {
		_ = os.Remove(tmppath) // Ignore errors
		return err
	}
	digest := hasher.Finalize()
	okey := digest.String()

	// Add info to DB
	this.lock.Lock()
	obj := this.getObj(okey)
	obj.newFile(tmppath)
	obj.Holds++
	this.writeObj(obj)
	this.lock.Unlock()

	// Put to meta-data layer
	data, _ := transfer.EncodeBytes(digest)
	err = this.MetaMgr.Put(topic, writer, key, data)

	// Remove hold
	this.lock.Lock()
	obj = this.getObj(okey)
	obj.Holds--
	this.writeObj(obj)
	this.lock.Unlock()

	return err
}

// Reads from io.Reader and generates a new object, put it to the meta-data layer
func (this *DataMgr) GetData(topic string, key string, stream io.Writer) error {
	data := this.Get(topic, key)
	if data == nil {
		return fmt.Errorf("Unknown key")
	}
	var objHash *crypto.Digest
	err := transfer.DecodeBytes(data, &objHash)
	if err != nil {
		return err
	}
	okey := objHash.String()

	this.lock.Lock()
	obj := this.maybeGetObj(okey)
	if obj == nil || obj.State != DSLocal {
		this.lock.Unlock()
		return fmt.Errorf("File not local yet")
	}
	obj.Holds++
	this.writeObj(obj)
	this.lock.Unlock()

	name := path.Join(this.dir, okey)
	file, err := os.Open(name)
	if err == nil {
		_, err = io.Copy(stream, file)
	}

	this.lock.Lock()
	obj = this.getObj(okey)
	obj.Holds--
	this.writeObj(obj)
	this.lock.Unlock()

	return err
}
