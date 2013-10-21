package meta

import (
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"h0tb0x/test"
	. "launchpad.net/gocheck"
	"log"
	"testing"
	"time"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestMetaSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestMetaSuite{})
}

func (this *TestMetaSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestMetaSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

type TestNode struct {
	id   *crypto.SecretIdentity
	port uint16
	log  *log.Logger
	link *link.LinkMgr
	sync *sync.SyncMgr
	meta *MetaMgr
}

func (this *TestMetaSuite) NewTestNode(name string, port uint16) *TestNode {
	base := this.NewBase(name, port)
	link := link.NewLinkMgr(base, this.ConnMgr)
	sync := sync.NewSyncMgr(link)
	meta := NewMetaMgr(sync)
	rc := rendezvous.NewClient(this.ConnMgr)
	err := rc.Put("http://localhost:3030", base.Ident, "localhost", port)
	this.C.Assert(err, IsNil)

	tn := &TestNode{
		id:   base.Ident,
		port: base.Port,
		log:  base.Log,
		link: link,
		sync: sync,
		meta: meta,
	}
	tn.meta.Start()
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

func (this *TestMetaSuite) TestMeta(c *C) {
	this.C = c

	// Make some users
	alice := this.NewTestNode("A", 10001)
	bob := this.NewTestNode("B", 10002)
	carol := this.NewTestNode("C", 10003)
	dave := this.NewTestNode("D", 10004)

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
	c.Assert(w, NotNil)
	c.Assert(w, DeepEquals, []byte("World"))

	// Stop everyone
	alice.Stop()
	bob.Stop()
	carol.Stop()
	dave.Stop()
}
