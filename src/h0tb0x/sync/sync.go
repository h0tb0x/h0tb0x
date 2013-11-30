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
// Topic, Key, Author, and Type. Topic determines who cares, Type allows multiple
// 'namespaces' within the topic, and Key is the rest of the key. Multiple authors may disagree
// about the value, thus we allow one value per author. The value also has a primary part (Value),
// along with Priority to help disambiguate.  The Signature allows cryptographic validation of the
// Author if set.
type Record struct {
	Type      int    // A namespacing mechanism for keys.
	Topic     string // Basis for subscriptions, defines who is interested in this record.
	Key       string // The primary key for this record within the topic
	Value     []byte // The current value of this key
	Priority  int    // A mechanism to disambiguate multiple records with the same key
	Author    string // The Fingerprint of the Author that generated this value
	Signature []byte // The signature from the author
	Seqno     int
}

const (
	RTSubscribe = 0 // Used by the sync layer itself to manage subscriptions
	RTBasis     = 1 // Used by the meta-data layer to manage collections
	RTWriter    = 2 // Used by the meta-data layer to manage writer
	RTData      = 3 // Used by the meta-data layer to manage meta-data
	RTAdvert    = 4 // Used by the data layer to manage storage
)

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
			SELECT r.topic, r.seqno, r.key, r.value, r.type, r.author, r.priority, r.signature
			FROM Record r, TopicFriend tf
			WHERE
				r.topic = tf.topic AND
				tf.friend_id = ? AND
				tf.desired = 1 AND tf.requested = 1 AND
				r.seqno > tf.acked_seqno
			ORDER BY r.seqno
			LIMIT 100`
		var list []Record
		_, err := this.sync.Db.Select(&list, sql, this.friendId)
		if err != nil {
			panic(err)
		}

		orm := make(map[string]int)
		for _, msg := range list {
			orm[msg.Topic] = msg.Seqno
		}
		if len(list) > 0 {
			this.sync.Log.Printf("Doing an notify of %d rows\n", len(list))
			this.lock.Unlock()
			var send_buf, recv_buf bytes.Buffer
			transfer.Encode(&send_buf, list)
			err := this.safeSend(&send_buf, &recv_buf)
			this.lock.Lock()
			if err != nil {
				continue
			}
			for topic, seqno := range orm {
				// Mark that we did the notify
				_, err := this.sync.Db.Exec(
					"UPDATE TopicFriend SET acked_seqno = ? WHERE friend_id = ? AND topic = ?",
					seqno, this.friendId, topic)
				if err != nil {
					panic(err)
				}
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
	this := &SyncMgr{
		LinkMgr: thelink,
		sinks:   make(map[int]func(int, *crypto.Digest, *Record)),
		clients: make(map[string]*clientLooper),
		cmut:    base.NewNoisyLocker(thelink.Log.Prefix() + "sync "),
	}
	this.AddHandler(link.ServiceNotify, this.onNotify)
	this.SetSink(RTSubscribe, this.onSubscribe)
	this.AddListener(this.onFriendChange)
	return this
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
	var records []Record
	err := transfer.Decode(in, &records)
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

	for _, record := range records {
		this.Log.Printf("Received data, topic = %s, key = %s", record.Topic, record.Key)
		prev, err := this.Db.SelectNullInt(`
			SELECT heard_seqno 
			FROM TopicFriend
			WHERE topic = ? AND friend_id = ? AND desired = 1`,
			record.Topic, remote)
		if err != nil {
			panic(err)
		}
		if !prev.Valid {
			continue
		}
		sink, ok := this.sinks[record.Type]
		if !ok || prev.Int64 >= int64(record.Seqno) {
			continue
		}
		sink(cl.friendId, fp, &record)
		_, err = this.Db.Exec(
			"Update TopicFriend SET heard_seqno = ? WHERE topic = ? AND friend_id = ?",
			record.Seqno, record.Topic, remote)
		if err != nil {
			panic(err)
		}
	}
	return nil
}

// Get the special outbox topic for a friend
func (this *SyncMgr) OutboxTopic(friend *crypto.Digest) string {
	myFp := this.Ident.Public().Fingerprint()
	return crypto.HashOf(myFp, friend).String()
}

// Get the special inbox topic for a friend
func (this *SyncMgr) InboxTopic(friend *crypto.Digest) string {
	myFp := this.Ident.Public().Fingerprint()
	return crypto.HashOf(friend, myFp).String()
}

// Get my self topic
func (this *SyncMgr) SelfTopic() string {
	myFp := this.Ident.Public().Fingerprint()
	return crypto.HashOf(myFp, myFp).String()
}

// Get my profile topic
func (this *SyncMgr) ProfileTopic() string {
	myFp := this.Ident.Public().Fingerprint()
	return crypto.HashOf(myFp, crypto.HashOf("profile")).String()
}

// Get a friend's profile topic
func (this *SyncMgr) FriendProfileTopic(friend *crypto.Digest) string {
	return crypto.HashOf(friend, crypto.HashOf("profile")).String()
}

func (this *SyncMgr) addTopicFriend(topic string, id int, desired, requested bool) {
	_, err := this.Db.Exec(`
		INSERT OR IGNORE INTO TopicFriend (topic, friend_id, desired, requested) 
		VALUES (?, ?, ?, ?)`,
		topic, id, desired, requested)
	if err != nil {
		panic(err)
	}
}

func (this *SyncMgr) onFriendChange(id int, fp *crypto.Digest, what link.FriendStatus) {
	this.cmut.Lock()
	if what == link.FriendStartup || what == link.FriendAdded {
		this.Log.Printf("Adding friend: %s", fp.String())
		// create inbox
		this.addTopicFriend(this.InboxTopic(fp), id, true, true)
		// create outbox
		this.addTopicFriend(this.OutboxTopic(fp), id, true, true)
		// Export my profile
		this.addTopicFriend(this.ProfileTopic(), id, true, true)
		// Import their profile
		this.addTopicFriend(this.FriendProfileTopic(fp), id, true, true)
		cl := newClientLooper(this, id)
		this.clients[fp.String()] = cl
		cl.run()
	} else {
		cl := this.clients[fp.String()]
		cl.stop()
		delete(this.clients, fp.String())
		_, err := this.Db.Exec("DELETE FROM TopicFriend WHERE friend_id = ?", id)
		if err != nil {
			panic(err)
		}
	}
	this.cmut.Unlock()
}

// Put a record for synchronization to friends subscribed to the topic of the record.
// Overwrites any existing record with the same (Topic, Type, Author, Key).
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
	this.cmut.Lock()
	for _, client := range this.clients {
		client.lock.Lock()
	}
	_, err := this.Db.Exec(`
		REPLACE INTO Record
			(seqno, topic, key, value, type, author, priority, signature)
		VALUES
			(IFNULL((SELECT MAX(seqno) FROM Record), 0)+1, ?, ?, ?, ?, ?, ?, ?)`,
		record.Topic,
		record.Key,
		record.Value,
		record.Type,
		record.Author,
		record.Priority,
		record.Signature)
	if err != nil {
		panic(err)
	}

	for _, client := range this.clients {
		client.wakeNotify.Broadcast()
		client.lock.Unlock()
	}
	this.cmut.Unlock()
}

// Get the latest record for any author for a specific topic and key and type.
func (this *SyncMgr) Get(recordType int, topic, key string) *Record {
	sql := `
		SELECT topic, key, value, type, author, priority, signature
		FROM Record
		WHERE topic = ? AND key = ? AND type = ?
		ORDER BY priority DESC 
		LIMIT 1`
	var record Record
	err := this.Db.SelectOne(&record, sql, topic, key, recordType)
	if err != nil {
		panic(err)
	}
	if record.Topic == "" {
		return nil
	}
	return &record
}

// Get the latest record for a specific author, topic, key and type.
func (this *SyncMgr) GetAuthor(recordType int, topic, key, author string) *Record {
	sql := `
		SELECT topic, key, value, type, author, priority, signature
		FROM Record
		WHERE author = ? AND topic = ? AND key = ? AND type = ?
		ORDER BY priority DESC 
		LIMIT 1`
	var record Record
	err := this.Db.SelectOne(&record, sql, author, topic, key, recordType)
	if err != nil {
		panic(err)
	}
	if record.Topic == "" {
		return nil
	}
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
	_, err := this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (friend_id, topic) VALUES (?, ?)",
		client.friendId, topic)
	if err != nil {
		panic(err)
	}

	_, err = this.Db.Exec("UPDATE TopicFriend SET desired = ? WHERE friend_id = ? AND topic = ?",
		enable, client.friendId, topic)
	if err != nil {
		panic(err)
	}

	heard, err := this.Db.SelectNullInt(`
			SELECT heard_seqno 
			FROM TopicFriend
			WHERE topic = ? AND friend_id = ?`,
		topic, client.friendId)
	if err != nil {
		panic(err)
	}

	enbyte := byte(0)
	if enable {
		enbyte = 1
	}
	client.lock.Unlock()
	this.Put(&Record{
		Type:     RTSubscribe,
		Topic:    this.OutboxTopic(id),
		Key:      topic,
		Value:    []byte{enbyte},
		Priority: int(heard.Int64),
		Author:   "$",
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
	_, err := this.Db.Exec("INSERT OR IGNORE INTO TopicFriend (friend_id, topic) VALUES (?, ?)",
		client.friendId, rec.Key)
	if err != nil {
		panic(err)
	}

	enable := (rec.Value[0] != 0)
	_, err = this.Db.Exec("UPDATE TopicFriend SET requested = ?, acked_seqno = ? WHERE friend_id = ? AND topic = ?",
		enable, rec.Priority, id, rec.Key)
	if err != nil {
		panic(err)
	}
	client.wakeNotify.Broadcast()
	client.lock.Unlock()
}
