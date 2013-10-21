package link

import (
	"bytes"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"h0tb0x/rendezvous"
	"h0tb0x/test"
	"io"
	"io/ioutil"
	. "launchpad.net/gocheck"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestLinkSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestLinkSuite{})
}

func (this *TestLinkSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestLinkSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

type TestNode struct {
	*base.Base
	Link *LinkMgr
	c    *C
}

func (this *TestLinkSuite) NewTestNode(name string, port uint16) *TestNode {
	base := this.NewBase(name, port)
	link := NewLinkMgr(base, this.ConnMgr)
	rc := rendezvous.NewClient(this.ConnMgr)
	err := rc.Put("http://localhost:3030", base.Ident, "localhost", port)
	this.C.Assert(err, IsNil)
	return &TestNode{
		Base: base,
		Link: link,
		c:    this.C,
	}
}

func (this *TestNode) Start() {
	this.Link.AddListener(this.OnFriendChange)
	this.Link.AddHandler(0, this.OnData)
	this.c.Assert(this.Link.Start(), IsNil)
}

func (this *TestNode) Stop() {
	this.Link.Stop()
}

func CreateLink(lhs, rhs *TestNode) {
	lhs.Link.AddUpdateFriend(rhs.Link.Ident.Fingerprint(), "localhost:3030")
	rhs.Link.AddUpdateFriend(lhs.Link.Ident.Fingerprint(), "localhost:3030")
}

func (this *TestNode) OnFriendChange(id int, fingerprint *crypto.Digest, what FriendStatus) {
	this.Log.Printf("OnFriendChange(%d, %s, %d)", id, fingerprint.String(), what)
}

func (this *TestNode) OnData(id int, fp *crypto.Digest, req io.Reader, resp io.Writer) (err error) {
	buf, _ := ioutil.ReadAll(req)
	this.Log.Printf("OnData(%d): %v", id, buf)
	resp.Write([]byte("123"))
	return nil
}

func (this *TestLinkSuite) TestLink(c *C) {
	this.C = c

	alice := this.NewTestNode("A", 10001)
	alice.Start()
	bob := this.NewTestNode("B", 10002)
	bob.Start()
	CreateLink(alice, bob)

	buf := new(bytes.Buffer)
	err := alice.Link.Send(0, 1, bytes.NewBuffer([]byte{4, 5, 6}), buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "123")

	buf = new(bytes.Buffer)
	err = alice.Link.Send(0, 1, bytes.NewBuffer([]byte{5, 4, 3}), buf)
	c.Assert(err, IsNil)
	c.Assert(buf.String(), Equals, "123")

	buf = new(bytes.Buffer)
	err = alice.Link.Send(3, 1, bytes.NewBuffer([]byte{5, 4, 3}), buf)
	c.Assert(err, ErrorMatches, "*403$")

	alice.Stop()
	bob.Stop()
}
