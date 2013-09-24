package link

import (
	"bytes"
	"fmt"
	"h0tb0x/base"
	"h0tb0x/crypto"
	"io"
	"io/ioutil"
	"sync"
	"testing"
)

type TestNode struct {
	*base.Base
	Link *LinkMgr
	wg   sync.WaitGroup
}

func NewTestNode(name string, port uint16) *TestNode {
	base := base.NewBase(name, port)
	link := NewLinkMgr(base)

	return &TestNode{
		Base: base,
		Link: link,
	}
}

func (this *TestNode) Run() {
	this.Link.AddListener(this.OnFriendChange)
	this.Link.AddHandler(0, this.OnData)
	this.Link.Run()
}

func (this *TestNode) Stop() {
	this.Link.Stop()
}

func CreateLink(lhs, rhs *TestNode) {
	lhs.Link.AddUpdateFriend(rhs.Link.Ident.Fingerprint(), "localhost", rhs.Link.Port)
	rhs.Link.AddUpdateFriend(lhs.Link.Ident.Fingerprint(), "localhost", lhs.Link.Port)
}

func (this *TestNode) OnFriendChange(id int, fingerprint *crypto.Digest, what FriendStatus) {
	this.Log.Printf("OnFriendChange(%d, %s, %d)", id, fingerprint.String(), what)
}

func (this *TestNode) OnData(id int, fp *crypto.Digest, req io.Reader, resp io.Writer) (err error) {
	buf, _ := ioutil.ReadAll(req)
	this.Log.Printf("OnData(%d): %v", id, buf)
	resp.Write([]byte{1, 2, 3})
	return nil
}

func TestLink(t *testing.T) {
	alice := NewTestNode("Alice", 10001)
	alice.Run()
	bob := NewTestNode("Bob", 10002)
	bob.Run()
	CreateLink(alice, bob)

	var b1 bytes.Buffer
	err := alice.Link.Send(0, 1, bytes.NewBuffer([]byte{4, 5, 6}), &b1)
	if err != nil {
		fmt.Printf("Err: %s", err)
	}
	alice.Log.Printf("Received: %v", b1.Bytes())
	var b2 bytes.Buffer
	_ = alice.Link.Send(0, 1, bytes.NewBuffer([]byte{5, 4, 3}), &b2)
	alice.Log.Printf("Received: %v", b2.Bytes())
	err2 := alice.Link.Send(3, 1, bytes.NewBuffer([]byte{5, 4, 3}), &b2)
	alice.Log.Printf("%s", err2)

	alice.Stop()
	bob.Stop()
}
