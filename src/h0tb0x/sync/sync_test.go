package sync

import (
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/rendezvous"
	"log"
	"os"
	"sync"
	"testing"
	"time"
)

type TestNode struct {
	log   *log.Logger
	port  uint16
	ident *crypto.SecretIdentity
	link  *link.LinkMgr
	sync  *SyncMgr
	wg    sync.WaitGroup
}

func NewTestNode(name string, port uint16) *TestNode {
	base := base.NewBase(name, port)
	link := link.NewLinkMgr(base)
	sync := NewSyncMgr(link)
	rendezvous.Publish("http://localhost:3030", base.Ident, "localhost", port)

	tn := &TestNode{
		log:   base.Log,
		port:  base.Port,
		ident: base.Ident,
		link:  link,
		sync:  sync,
	}
	tn.sync.SetSink(RTData, tn.OnData)
	tn.sync.Run()
	return tn
}

func (this *TestNode) OnData(id int, src *crypto.Digest, record *Record) {
	this.log.Printf("Data: %v, %v, %s", record.Topic, record.Key, record.Value)
	this.wg.Done()
}

func (this *TestNode) Stop() {
	this.sync.Stop()
}

func CreateLink(lhs, rhs *TestNode) {
	lhs.link.AddUpdateFriend(rhs.ident.Fingerprint(), "localhost:3030")
	rhs.link.AddUpdateFriend(lhs.ident.Fingerprint(), "localhost:3030")
}

func TestSync(t *testing.T) {
	os.Remove("/tmp/rtest.db")
	rm := rendezvous.NewRendezvousMgr(3030, "/tmp/rtest.db")
	rm.Run()

	alice := NewTestNode("Alice", 10001)
	bob := NewTestNode("Bob", 10002)

	CreateLink(alice, bob)

	topic := "CuteKittens"
	key := "key"
	value := "value"

	alice.sync.Subscribe(bob.ident.Fingerprint(), topic, true)
	bob.sync.Subscribe(alice.ident.Fingerprint(), topic, true)

	bob.wg.Add(1)

	alice.sync.Put(&Record{
		RecordType: RTData,
		Topic:      topic,
		Key:        key,
		Value:      []byte(value),
		Author:     "unused",
	})

	// Hang out to make sure things aren't chatty
	time.Sleep(2 * time.Second)

	bob.wg.Wait()
	bob.wg.Add(2)

	alice.sync.Put(&Record{
		RecordType: RTData,
		Topic:      topic,
		Key:        "hello",
		Value:      []byte("world"),
		Author:     "unused",
	})
	alice.sync.Put(&Record{
		RecordType: RTData,
		Topic:      topic,
		Key:        "what",
		Value:      []byte("the"),
		Author:     "unused",
	})

	bob.wg.Wait()

	alice.log.Printf("Stopping Alice")
	alice.Stop()
	bob.log.Printf("Stopping Bob")
	bob.Stop()
}
