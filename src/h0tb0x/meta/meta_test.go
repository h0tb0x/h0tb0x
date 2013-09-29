package meta

import (
	"bytes"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"log"
	"os"
	"testing"
	"time"
)

type TestNode struct {
	id   *crypto.SecretIdentity
	port uint16
	base *base.Base
	log  *log.Logger
	link *link.LinkMgr
	sync *sync.SyncMgr
	meta *MetaMgr
}

func NewTestNode(name string, port uint16) *TestNode {
	base := base.NewBase(name, port)
	link := link.NewLinkMgr(base)
	sync := sync.NewSyncMgr(link)
	meta := NewMetaMgr(sync)
	rendezvous.Publish("localhost:3030", base.Ident, "localhost", port)

	tn := &TestNode{
		id:   base.Ident,
		port: base.Port,
		log:  base.Log,
		link: link,
		sync: sync,
		meta: meta,
	}
	tn.meta.Run()
	return tn
}

func (this *TestNode) Stop() {
	this.meta.Stop()
}

func CreateLink(lhs, rhs *TestNode) {
	lhs.link.AddUpdateFriend(rhs.id.Fingerprint(), "localhost:3030")
	rhs.link.AddUpdateFriend(lhs.id.Fingerprint(), "localhost:3030")
}

func SubPub(lhs, rhs *TestNode, cid string) {
	lhs.sync.Subscribe(rhs.id.Fingerprint(), cid, true)
	rhs.sync.Subscribe(lhs.id.Fingerprint(), cid, true)
}

func TestMeta(t *testing.T) {
	os.Remove("/tmp/rtest.db")
	rm := rendezvous.NewRendezvousMgr(3030, "/tmp/rtest.db")
	rm.Run()

	// Make some users
	alice := NewTestNode("Alice", 10001)
	bob := NewTestNode("Bob", 10002)
	carol := NewTestNode("Carol", 10003)
	dave := NewTestNode("Dave", 10004)

	// Make a circle of links
	CreateLink(alice, bob)
	CreateLink(bob, carol)
	CreateLink(carol, dave)
	CreateLink(dave, alice)

	// Alice makes a collection
	cid := alice.meta.CreateNewCollection(alice.id)

	// Every publishes and subscribes
	SubPub(alice, bob, cid)
	SubPub(bob, carol, cid)
	SubPub(carol, dave, cid)
	SubPub(dave, alice, cid)

	// Alice allows dave to write
	alice.meta.AddWriter(cid, alice.id, dave.id.Public())

	// Wait for a bit to make sure dave knows he can write
	// TODO: Make this actually not race like
	time.Sleep(3 * time.Second)

	// Dave writes a record
	dave.meta.Put(cid, dave.id, "Hello", []byte("World"))

	// Wait until is propagates, TODO: No races
	time.Sleep(3 * time.Second)

	// Bob reads the records
	w := bob.meta.Get(cid, "Hello")
	if w == nil {
		t.Fatal("Unable to read record")
	}
	if !bytes.Equal(w, []byte("World")) {
		t.Fatal("Record had wrong value")
	}

	// Stop everyone
	alice.Stop()
	bob.Stop()
	carol.Stop()
	dave.Stop()

	rm.Stop()
}
