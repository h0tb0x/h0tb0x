package sync

import (
	"bytes"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/transfer"
	"io"
	"sync"
	"time"
)

// Records at it's most basic is a key/value pair, however the key is divided into three parts:
// Topic, Key, Author, and RecordType. Topic determines who cares, RecordType allows multiple
// 'namespaces' within the topic, and Key is the rest of the key. Multiple authors may disagree
// about the value, thus we allow one value per author. The value also has a primary part (Value),
// along with Priority to help disambiguate.  The Signature allows cryptographic validation of the
// Author if set.
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
	RTInvite    = 5 // Used by the api layer to manage invitations
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
	lock       sync.Locker
	shutdown   chan bool
	wakeNotify sync.Cond
	goRoutines sync.WaitGroup
}

func newClientLooper(sync *SyncMgr, friendId int) *clientLooper {
	cl := &clientLooper{
		sync:     sync,
		friendId: friendId,
		shutdown: make(chan bool),
		lock:     base.NewNoisyLocker(sync.Log.Prefix() + "clientLooper "),
	}
	cl.wakeNotify.L = cl.lock
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

func (this *clientLooper) safeSend(in io.Reader, out io.Writer) error {
	tryTime := time.Now()
	err := this.sync.Send(link.ServiceNotify, this.friendId, in, out)
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
			SELECT 
				t.topic_id, t.name, o.seqno, o.key, o.value, o.type, o.author, o.priority, o.signature
			FROM Object o
				JOIN TopicFriend tf USING (topic_id)
				JOIN Topic t USING (topic_id)
			WHERE 
				tf.friend_id = ? AND
				tf.desired = 1 AND tf.requested = 1 AND
				o.seqno > tf.acked_seqno
			ORDER BY o.seqno
			LIMIT 100`

		rows := this.sync.Db.MultiQuery(sql, this.friendId)
		data := []dataMesg{}
		orm := make(map[int]int)
		for rows.Next() {
			var m dataMesg
			var tmp []byte
			var topicId int
			this.sync.Db.Scan(rows,
				&topicId, &m.Topic, &m.Seqno, &m.Key, &m.Value,
				&m.RecordType, &tmp, &m.Priority, &m.Signature)
			m.Author = string(tmp)
			orm[topicId] = m.Seqno
			data = append(data, m)
		}
		if len(data) > 0 {
			this.sync.Log.Printf("Notify %d rows\n", len(data))
			this.lock.Unlock()
			var send_buf, recv_buf bytes.Buffer
			transfer.Encode(&send_buf, data)
			err := this.safeSend(&send_buf, &recv_buf)
			this.lock.Lock()
			if err != nil {
				continue
			}
			for topicId, seqno := range orm {
				// Mark that we did the notify
				this.sync.Db.Exec(`
					UPDATE TopicFriend 
					SET acked_seqno = ? 
					WHERE friend_id = ? AND topic_id = ?`,
					seqno, this.friendId, topicId)
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
	cmut    base.RWLocker
}

// Constructs a new SyncMgr, does not start it.
func NewSyncMgr(thelink *link.LinkMgr) *SyncMgr {
	mgr := &SyncMgr{
		LinkMgr: thelink,
		sinks:   make(map[int]func(int, *crypto.Digest, *Record)),
		clients: make(map[string]*clientLooper),
		cmut:    base.NewNoisyLocker(thelink.Log.Prefix() + "sync "),
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
	client := this.getClient(fp)
	if client == nil {
		this.Log.Printf("Receiving notify from non-friend, ignoring")
		return nil
	}

	for _, mesg := range mesgs {
		this.Log.Printf("onNotify: %s, %q", mesg.Topic, mesg.Key)
		row := this.Db.SingleQuery(`
			SELECT tf.heard_seqno, t.topic_id
			FROM TopicFriend tf
				JOIN Topic t USING (topic_id)
			WHERE t.name = ? AND tf.friend_id = ? AND tf.desired = 1`,
			mesg.Topic, remote)
		var prev_seq, topicId int
		if !this.Db.MaybeScan(row, &prev_seq, &topicId) {
			continue
		}
		sink, ok := this.sinks[mesg.RecordType]
		if !ok || prev_seq >= mesg.Seqno {
			continue
		}
		sink(client.friendId, fp, &mesg.Record)
		this.Db.Exec(`
			UPDATE TopicFriend 
			SET heard_seqno = ? 
			WHERE topic_id = ? AND friend_id = ?`,
			mesg.Seqno, topicId, remote)
	}
	return nil
}

func (this *SyncMgr) addTopic(topic string) int {
	this.Db.Exec("INSERT OR IGNORE INTO Topic (name) VALUES (?)", topic)
	row := this.Db.SingleQuery("SELECT topic_id FROM Topic WHERE name = ?", topic)
	var topicId int
	this.Db.Scan(row, &topicId)
	return topicId
}

func (this *SyncMgr) addTopicFriend(topic string, friendId int, desired, requested bool) int {
	topicId := this.addTopic(topic)
	this.Db.Exec(`
		INSERT OR IGNORE INTO TopicFriend 
			(topic_id, friend_id, desired, requested) 
		VALUES 
			(?, ?, ?, ?)`,
		topicId, friendId, desired, requested)
	return topicId
}

func (this *SyncMgr) onFriendChange(friendId int, fp *crypto.Digest, what link.FriendStatus) {
	this.cmut.Lock()
	defer this.cmut.Unlock()
	if what == link.FriendStartup || what == link.FriendAdded {
		this.Log.Printf("Adding friend: %s", fp.String())
		myFp := this.Ident.Public().Fingerprint()
		// create inbox
		this.addTopicFriend(crypto.HashOf(fp, myFp).String(), friendId, true, true)
		// create outbox
		this.addTopicFriend(crypto.HashOf(myFp, fp).String(), friendId, true, true)
		client := newClientLooper(this, friendId)
		this.clients[fp.String()] = client
		client.run()
	} else {
		client := this.clients[fp.String()]
		client.stop()
		delete(this.clients, fp.String())
		this.Db.Exec("DELETE FROM TopicFriend WHERE friend_id = ?", friendId)
	}
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
	this.Log.Printf("PUT: %s, %q", record.Topic, record.Key)
	this.cmut.Lock()
	for _, client := range this.clients {
		client.lock.Lock()
	}
	topicId := this.addTopic(record.Topic)
	this.Db.Exec(`
		REPLACE INTO Object
			(seqno, topic_id, key, value, type, author, priority, signature)
		VALUES
			(IFNULL((SELECT MAX(seqno) FROM Object), 0)+1, ?, ?, ?, ?, ?, ?, ?)`,
		topicId,
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
	this.cmut.Unlock()
}

// Get the latest record for any author for a specific topic and key and type.
func (this *SyncMgr) Get(recordType int, topic, key string) *Record {
	query := `
		SELECT t.name, o.key, o.value, o.type, o.author, o.priority, o.signature
		FROM Object o
			JOIN Topic t USING (topic_id)
		WHERE t.name = ? AND o.key = ? AND o.type = ?
		ORDER BY o.priority DESC 
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
		SELECT t.name, o.key, o.value, o.type, o.author, o.priority, o.signature
		FROM Object o
			JOIN Topic t USING (topic_id)
		WHERE o.author = ? AND t.name = ? AND o.key = ? AND o.type = ?
		ORDER BY o.priority DESC 
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

func (this *SyncMgr) getClient(fp *crypto.Digest) *clientLooper {
	this.cmut.RLock()
	defer this.cmut.RUnlock()
	return this.clients[fp.String()]
}

// Update the subscription state for a topic on a particular friend.
func (this *SyncMgr) Subscribe(id *crypto.Digest, topic string, enable bool) bool {
	client := this.getClient(id)
	if client == nil {
		return false
	}

	client.lock.Lock()
	topicId := this.addTopicFriend(topic, client.friendId, false, false)
	this.Db.Exec(`
		UPDATE TopicFriend 
		SET desired = ? 
		WHERE friend_id = ? AND topic_id = ?`,
		enable, client.friendId, topicId)
	row := this.Db.SingleQuery(`
		SELECT heard_seqno 
		FROM TopicFriend 
		WHERE friend_id = ? AND topic_id = ?`,
		client.friendId, topicId)
	client.lock.Unlock()

	var heard int
	this.Db.Scan(row, &heard)

	enbyte := byte(0)
	if enable {
		enbyte = 1
	}
	myFp := this.Ident.Public().Fingerprint()

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
	client := this.getClient(fp)
	if client == nil {
		return
	}
	client.lock.Lock()
	topicId := this.addTopicFriend(rec.Key, client.friendId, false, false)
	enable := (rec.Value[0] != 0)
	this.Db.Exec(`
		UPDATE TopicFriend 
		SET requested = ?, acked_seqno = ? 
		WHERE friend_id = ? AND topic_id = ?`,
		enable, rec.Priority, client.friendId, topicId)
	client.wakeNotify.Broadcast()
	client.lock.Unlock()
}
