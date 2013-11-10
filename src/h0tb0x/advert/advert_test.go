package advert

import (
	"bytes"
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

type TestAdvertSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestAdvertSuite{})
}

func (this *TestAdvertSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestAdvertSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

type TestNode struct {
	id     *crypto.SecretIdentity
	port   uint16
	log    *log.Logger
	link   *link.LinkMgr
	sync   *sync.SyncMgr
	advert *AdvertMgr
}

func (this *TestAdvertSuite) NewTestNode(name string, port uint16) *TestNode {
	base := this.NewBase(name, port)
	link := link.NewLinkMgr(base, this.ConnMgr)
	sync := sync.NewSyncMgr(link)
	advert := NewAdvertMgr(sync)
	rc := rendezvous.NewClient(this.ConnMgr)
	err := rc.Put("http://localhost:3030", base.Ident, "localhost", port)
	this.C.Assert(err, IsNil)

	tn := &TestNode{
		id:     base.Ident,
		port:   base.Port,
		log:    base.Log,
		link:   link,
		sync:   sync,
		advert: advert,
	}
	tn.advert.Start()
	return tn
}

func (this *TestNode) Stop() {
	this.advert.Stop()
}

func CreateLink(lhs, rhs *TestNode) {
	lhs.link.AddUpdateFriend(rhs.id.Fingerprint(), "localhost:3030")
	rhs.link.AddUpdateFriend(lhs.id.Fingerprint(), "localhost:3030")
}

func KillLink(lhs, rhs *TestNode) {
	lhs.link.RemoveFriend(rhs.id.Fingerprint())
	rhs.link.RemoveFriend(lhs.id.Fingerprint())
}

func (this *TestAdvertSuite) TestAdvert1(c *C) {
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

	// Alice has some data
	alice.advert.HasCopy("foo")
	carol.advert.IncRef("foo")

	// Wait for adverts to propagate
	time.Sleep(3 * time.Second)

	// Carol get's data from alice
	var buf bytes.Buffer
	err := carol.advert.Request("foo", &buf)
	c.Assert(err, IsNil)

	// Stop everyone
	alice.Stop()
	bob.Stop()
	carol.Stop()
	dave.Stop()
}

func (this *TestAdvertSuite) TestAdvert2(c *C) {
	this.C = c

	// Make some users
	alice := this.NewTestNode("A", 10001)
	bob := this.NewTestNode("B", 10002)
	carol := this.NewTestNode("C", 10003)
	dave := this.NewTestNode("D", 10004)

	// Make some links
	CreateLink(alice, bob)
	CreateLink(bob, carol)
	CreateLink(carol, dave)
	CreateLink(alice, dave)

	// Alice has some data
	alice.advert.HasCopy("foo")
	// Dave and Bob care about the data
	bob.advert.IncRef("foo")
	dave.advert.IncRef("foo")
	
	// Wait for adverts to propagate
	time.Sleep(3 * time.Second)

	// Dave get's data from alice
	var buf bytes.Buffer
	err := dave.advert.Request("foo", &buf)
	c.Assert(err, IsNil)

	// Now we kill the direct link between alice and dave
	KillLink(alice, dave)

	// Nothing should happen	
	time.Sleep(3 * time.Second)	

	// Dave tries again, and fails initially, setting off events to repair
	err = dave.advert.Request("foo", &buf)
		
	// Sleep during repair
	time.Sleep(3 * time.Second)

	// Try again, it should work
	err = dave.advert.Request("foo", &buf)
	c.Assert(err, IsNil)

	// Stop everyone
	alice.Stop()
	bob.Stop()
	carol.Stop()
	dave.Stop()
}

