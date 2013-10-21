package data

import (
	"bytes"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"h0tb0x/test"
	. "launchpad.net/gocheck"
	"testing"
	"time"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestDataSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestDataSuite{})
}

func (this *TestDataSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestDataSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

func (this *TestDataSuite) NewTestNode(name string, port uint16) *DataMgr {
	base := this.NewBase(name, port)
	link := link.NewLinkMgr(base, this.ConnMgr)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := NewDataMgr(this.GetTempDir(), meta)
	rc := rendezvous.NewClient(this.ConnMgr)
	err := rc.Put("http://localhost:3030", base.Ident, "localhost", port)
	this.C.Assert(err, IsNil)

	data.Start()
	return data
}

func CreateLink(lhs, rhs *DataMgr) {
	lhs.AddUpdateFriend(rhs.Ident.Fingerprint(), "localhost:3030")
	rhs.AddUpdateFriend(lhs.Ident.Fingerprint(), "localhost:3030")
}

func (this *TestDataSuite) TestData(c *C) {
	this.C = c

	alice := this.NewTestNode("A", 10001)
	bob := this.NewTestNode("B", 10002)

	CreateLink(alice, bob)
	time.Sleep(1 * time.Second)

	cid := alice.CreateNewCollection(alice.Ident)
	alice.Subscribe(bob.Ident.Fingerprint(), cid, true)
	bob.Subscribe(alice.Ident.Fingerprint(), cid, true)
	time.Sleep(1 * time.Second)

	file := bytes.NewBuffer([]byte("A GIF of a cute kitten"))
	err := alice.PutData(cid, "Kitten", alice.Ident, file)
	c.Assert(err, IsNil)
	time.Sleep(1 * time.Second)

	var outbuf bytes.Buffer
	err = bob.GetData(cid, "Kitten", &outbuf)
	c.Assert(err, IsNil)
	c.Assert(outbuf.Bytes(), DeepEquals, []byte("A GIF of a cute kitten"))

	alice.Stop()
	bob.Stop()
}
