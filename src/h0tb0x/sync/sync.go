package sync

import (
	"bytes"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/transfer"
	"io"
	"sync"
	"time"
)

// Records at it's most basic is a key/value pair, however the key is divided into three parts: Topic, Key, Author, and RecordType.
// Topic determines who cares, RecordType allows multiple 'namespaces' within the topic, and Key is the rest of the key.
// Multiple authors may disagree about the value, thus we allow one value per author.
// The value also has a primary part (Value), along with Priority to help disambiguate.  The Signature allows
// cryptographic validation of the Author if set.
type Record struct {
	RecordType int    // A namespacing mechanism for keys.
	Topic      string // Basis for subscriptions, defines who is interested in this record.
	Key        string // The primary key for this record within the topic
	Value      []byte // The current value of this key
	Priority   int    // A mechanism to disambiguate multiple records with the same key
	Author     string // The Fingerprint of the Author that generated this value
	Signature  []byte // The signature from the author
}

const (
	RTSubscribe = 0 // Used by the sync layer itself to manage subscriptions
	RTBasis     = 1 // Used by the meta-data layer to manage collections
	RTWriter    = 2 // Used by the meta-data layer to manage writer
	RTData      = 3 // Used by the meta-data layer to manage meta-data
	RTAdvert    = 4 // Used by the data layer to manage storage
)

type dataMesg struct {
	Record
	Seqno int
}

const (
	FailureRetry = 5 * time.Second
)

type clientLooper struct {
	sync       *SyncMgr
	friendId   int
	lock       sync.Mutex
	shutdown   chan bool
	wakeNotify sync.Cond
	goRoutines sync.WaitGroup
}

func newClientLooper(sync *SyncMgr, friendId int) *clientLooper {
	cl := &clientLooper{
		sync:     sync,
		friendId: friendId,
		shutdown: make(chan bool),
	}
	cl.wakeNotify.L = &cl.lock
	return cl
}

func (this *clientLooper) isClosing() bool {
	select {
	case <-this.shutdown:
		return true
	default:
		return false
	}
}

func (this *clientLooper) safeSleep(until time.Time) {
	waitTime := until.Sub(time.Now())
	if waitTime < 0 {
		return
	}
	tchan := time.After(waitTime)
	select {
	case <-this.shutdown:
		return
	case <-tchan:
		return
	}
}

func (this *clientLooper) safeSend(service int, in io.Reader, out io.Writer) error {
	tryTime := time.Now()
	err := this.sync.Send(service, this.friendId, in, out)
	if err != nil {
		// If err was fast, wait a bit
		this.sync.Log.Printf("Error %s, retrying!\n", err)
		this.safeSleep(tryTime.Add(FailureRetry))
	}
	return err
}

func (this *clientLooper) notifyLoop() {
	this.lock.Lock() // Lock is held *except* when doing remote calls & sleeping
	// While I'm not closing
	for !this.isClosing() {
		this.sync.Log.Printf("Looking for things to notify\n")
		sql := `
			SELECT o.topic, o.seqno, o.key, o.value, o.type, o.author, o.priority, o.signature
			FROM Object o, TopicFriend tf
				WHERE o.topic = tf.topic AND
				tf.friend_id = ? AND
				tf.desired = 1 AND tf.requested = 1 AND
				o.seqno > tf.acked_seqno
			ORDER BY o.seqno
			LIMIT 100`

		rows := this.sync.Db.MultiQuery(sql, this.friendId)
		data := []dataMesg{}
		orm := make(map[string]int)
		for rows.Next() {
			var m dataMesg
			var tmp []byte
			this.sync.Db.Scan(rows, &m.Topic, &m.Seqno, &m.Key, &m.Value,
				&m.RecordType, &tmp, &m.Priority, &m.Signature)
			m.Author = string(tmp)
			orm[m.Topic] = m.Seqno
			data = append(data, m)
		}
		if len(data) > 0 {
			this.sync.Log.Printf("Doing an notify of %d rows\n", len(data))
			this.lock.Unlock()
			var send_buf, recv_buf bytes.Buffer
			transfer.Encode(&send_buf, data)
			err := this.safeSend(link.ServiceNotify, &send_buf, &recv_buf)
			this.lock.Lock()
			if err != nil {
				continue
			}
			for topic, seqno := range orm {
				// Mark that we did the notify
				this.sync.Db.Exec("UPDATE TopicFriend SET acked_seqno = ? WHERE friend_id = ? AND topic = ?",
					seqno, this.friendId, topic)
			}
		} else {
			this.sync.Log.Printf("No notifies, sleeping\n")
			this.wakeNotify.Wait()
		}
	}
	this.lock.Unlock()
	this.goRoutines.Done()
}

func (this *clientLooper) run() {
	this.goRoutines.Add(1)
	go this.notifyLoop()
}

func (this *clientLooper) stop() {
	this.sync.Log.Printf("Gathering lock")
	this.lock.Lock()
	close(this.shutdown)
	this.sync.Log.Printf("Broadcasting wakeups")
	this.wakeNotify.Broadcast()
	this.lock.Unlock()
	this.sync.Log.Printf("Waiting")
	this.goRoutines.Wait()
}

// The SyncMgr is the primary interface for the Sync Layer
type SyncMgr struct {
	*link.LinkMgr
	sinks   map[int]func(int, *crypto.Digest, *Record)
	clients map[string]*clientLooper
	cmut    sync.RWMutex
}

// Constructs a new SyncMgr, does not start it.
func NewSyncMgr(thelink *link.LinkMgr) *SyncMgr {
	mgr := &SyncMgr{
		LinkMgr: thelink,
		sinks:   make(map[int]func(int, *crypto.Digest, *Record)),
		clients: make(map[string]*clientLooper),
	}
	mgr.AddHandler(link.ServiceNotify, mgr.onNotify)
	mgr.SetSink(RTSubscribe, mgr.onSubscribe)
	mgr.AddListener(mgr.onFriendChange)
	return mgr
}

// Sets the destination to send incoming sync data of a particular type
func (this *SyncMgr) SetSink(recordType int, sink func(int, *crypto.Digest, *Record)) {
	this.sinks[recordType] = sink
}

// Starts the sync manager.
func (this *SyncMgr) Run() error {
	return this.LinkMgr.Run()
}

// Stops the sync manager, block until complete
func (this *SyncMgr) Stop() {
	this.Log.Printf("Stopping server")
	this.cmut.Lock()
	for _, client := range this.clients {
		client.stop()
	}
	this.cmut.Unlock()
	this.LinkMgr.Stop()
}

func (this *SyncMgr) onNotify(remote int, fp *crypto.Digest, in io.Reader, out io.Writer) error {
	var mesgs []dataMesg
	err := transfer.Decode(in, &mesgs)
	if err != nil {
		return err
	}

	// Find client
	this.cmut.Lock()
	cl, ok := this.clients[fp.String()]
	this.cmut.Unlock()

	if !ok {
		this.Log.Printf("Receiving notify from non-friend, ignoring")
	}

	for _, mesg := range mesgs {
		this.Log.Printf("Received data, topic = %s, key = %s", mesg.Topic, mesg.Key)
		row := this.Db.SingleQuery(`SELECT heard_seqno FROM TopicFriend 
					WHERE topic = ? AND friend_id = ? AND desired = 1`,
			mesg.Topic, remote)
		var prev_seq int
		if !this.Db.MaybeScan(row, &prev_seq) {
			continue
		}
		sink, ok := this.sinks[mesg.RecordType]
		if !ok || prev_seq >= mesg.Seqno {
			continue
		}
		sink(cl.friendId, fp, &mesg.Record)
		this.Db.Exec("Update TopicFriend SET heard_seqno = ? WHERE topic = ? AND friend_id = ?",
			mesg.Seqno, mesg.Topic, remote)
	}
	return nil
}

func (this *SyncMgr) onFriendChange(id int, fp *crypto.Digest, what link.FriendStatus) {
	this.cmut.Lock()
	if what == link.FriendStartup || what == link.FriendAdded {
		this.Log.Printf("Adding friend: %s", fp.String())
		myFp := this.Ident.Public().Fingerprint()
		this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (topic, friend_id, desired, requested) VALUES (?, ?, ?, ?)",
			crypto.HashOf(fp, myFp).String(), id, 1, 1)
		this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (topic, friend_id, desired, requested) VALUES (?, ?, ?, ?)",
			crypto.HashOf(myFp, fp).String(), id, 1, 1)
		cl := newClientLooper(this, id)
		this.clients[fp.String()] = cl
		cl.run()
	} else {
		cl := this.clients[fp.String()]
		cl.stop()
		delete(this.clients, fp.String())
		this.Db.Exec("DELETE INTO TopicFriend WHERE friend_id = ?", id)
	}
	this.cmut.Unlock()
}

// Put a record for synchronization to friends subscribed to the topic of the record.
// Overwrites any existing record with the same (Topic, RecordType, Author, Key).
func (this *SyncMgr) Put(record *Record) {
	if record.Key == "" {
		panic("Key must be set")
	}
	if record.Topic == "" {
		panic("Topic must be set")
	}
	if record.Author == "" {
		panic("Author must be set")
	}
	this.Log.Printf("Doing a PUT: topic = %s, key = %s", record.Topic, record.Key)
	this.cmut.RLock()
	for _, client := range this.clients {
		client.lock.Lock()
	}
	this.Db.Exec(`
		REPLACE INTO Object
			(seqno, topic, key, value, type, author, priority, signature)
		VALUES
			(IFNULL((SELECT MAX(seqno) FROM Object), 0)+1, ?, ?, ?, ?, ?, ?, ?)`,
		record.Topic,
		record.Key,
		record.Value,
		record.RecordType,
		record.Author,
		record.Priority,
		record.Signature)

	for _, client := range this.clients {
		client.wakeNotify.Broadcast()
		client.lock.Unlock()
	}
	this.cmut.RUnlock()
}

// Get the latest record for any author for a specific topic and key and type.
func (this *SyncMgr) Get(recordType int, topic, key string) *Record {
	query := `
		SELECT topic, key, value, type, author, priority, signature
		FROM Object
		WHERE topic = ? AND key = ? AND type = ?
		ORDER BY priority DESC 
		LIMIT 1`
	row := this.Db.SingleQuery(query, topic, key, recordType)
	var record Record
	var tmp []byte
	if !this.Db.MaybeScan(row,
		&record.Topic, &record.Key, &record.Value,
		&record.RecordType, &tmp, &record.Priority, &record.Signature) {
		return nil
	}
	record.Author = string(tmp)
	return &record
}

// Get the latest record for a specific author, topic, key and type.
func (this *SyncMgr) GetAuthor(recordType int, topic, key, author string) *Record {
	query := `
		SELECT topic, key, value, type, author, priority, signature
		FROM Object
		WHERE author = ? AND topic = ? AND key = ? AND type = ?
		ORDER BY priority DESC 
		LIMIT 1`
	row := this.Db.SingleQuery(query, author, topic, key, recordType)
	var tmp []byte
	var record Record
	if !this.Db.MaybeScan(row,
		&record.Topic, &record.Key, &record.Value,
		&record.RecordType, &tmp, &record.Priority, &record.Signature) {
		return nil
	}
	record.Author = string(tmp)
	return &record
}

// Update the subscription state for a topic on a particular friend.
func (this *SyncMgr) Subscribe(id *crypto.Digest, topic string, enable bool) bool {
	this.cmut.RLock()
	client, ok := this.clients[id.String()]
	this.cmut.RUnlock()
	if !ok {
		return false
	}

	client.lock.Lock()
	this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (friend_id, topic) VALUES (?, ?)",
		client.friendId, topic)

	this.Db.Exec("UPDATE TopicFriend SET desired = ? WHERE friend_id = ? AND topic = ?",
		enable, client.friendId, topic)

	row := this.Db.SingleQuery("SELECT heard_seqno FROM TopicFriend WHERE friend_id = ? AND topic = ?",
		client.friendId, topic)
	var heard int
	this.Db.Scan(row, &heard)

	enbyte := byte(0)
	if enable {
		enbyte = 1
	}
	myFp := this.Ident.Public().Fingerprint()
	client.lock.Unlock()
	this.Put(&Record{
		RecordType: RTSubscribe,
		Topic:      crypto.HashOf(myFp, id).String(),
		Key:        topic,
		Value:      []byte{enbyte},
		Priority:   heard,
		Author:     "$",
	})

	return true
}

func (this *SyncMgr) onSubscribe(id int, fp *crypto.Digest, rec *Record) {
	this.cmut.RLock()
	client, ok := this.clients[fp.String()]
	this.cmut.RUnlock()
	if !ok {
		return
	}
	client.lock.Lock()
	this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (friend_id, topic) VALUES (?, ?)",
		client.friendId, rec.Key)

	enable := (rec.Value[0] != 0)
	this.Db.Exec("UPDATE TopicFriend SET requested = ?, acked_seqno = ? WHERE friend_id = ? AND topic = ?",
		enable, rec.Priority, id, rec.Key)
	client.wakeNotify.Broadcast()
	client.lock.Unlock()
}
