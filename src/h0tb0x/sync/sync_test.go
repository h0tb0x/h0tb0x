package sync

import (
	"h0tb0x/crypto"
	"h0tb0x/link"
	"h0tb0x/rendezvous"
	"h0tb0x/test"
	. "launchpad.net/gocheck"
	"log"
	"sync"
	"testing"
	"time"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestSyncSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestSyncSuite{})
}

func (this *TestSyncSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestSyncSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

type TestNode struct {
	log   *log.Logger
	port  uint16
	ident *crypto.SecretIdentity
	link  *link.LinkMgr
	sync  *SyncMgr
	wg    sync.WaitGroup
}

func (this *TestSyncSuite) NewTestNode(name string, port uint16) *TestNode {
	base := this.NewBase(name, port)
	link := link.NewLinkMgr(base, this.ConnMgr)
	sync := NewSyncMgr(link)
	rc := rendezvous.NewClient(this.ConnMgr)
	err := rc.Put("http://localhost:3030", base.Ident, "localhost", port)
	this.C.Assert(err, IsNil)

	tn := &TestNode{
		log:   base.Log,
		port:  base.Port,
		ident: base.Ident,
		link:  link,
		sync:  sync,
	}
	tn.sync.SetSink(RTData, tn.OnData)
	tn.sync.Start()
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

func (this *TestSyncSuite) TestSync(c *C) {
	this.C = c

	alice := this.NewTestNode("Alice", 10001)
	bob := this.NewTestNode("Bob", 10002)

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
