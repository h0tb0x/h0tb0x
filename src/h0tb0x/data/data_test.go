package data

import (
	"bytes"
	"h0tb0x/base"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/sync"
	"testing"
	"time"
)

func NewTestNode(name string, port uint16) *DataMgr {
	base := base.NewBase(name, port)
	link := link.NewLinkMgr(base)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := NewDataMgr("/tmp/wtf/"+name, meta)
	data.Run()
	return data
}

func CreateLink(lhs, rhs *DataMgr) {
	lhs.AddUpdateFriend(rhs.Ident.Fingerprint(), "localhost", rhs.Port)
	rhs.AddUpdateFriend(lhs.Ident.Fingerprint(), "localhost", lhs.Port)
}

func TestData(t *testing.T) {
	alice := NewTestNode("Alice", 10001)
	bob := NewTestNode("Bob", 10002)

	CreateLink(alice, bob)
	time.Sleep(1 * time.Second)

	cid := alice.CreateNewCollection(alice.Ident)
	alice.Subscribe(bob.Ident.Fingerprint(), cid, true)
	bob.Subscribe(alice.Ident.Fingerprint(), cid, true)
	time.Sleep(1 * time.Second)

	file := bytes.NewBuffer([]byte("A GIF of a cute kitten"))
	err := alice.PutData(cid, "Kitten", alice.Ident, file)
	if err != nil {
		t.Fatal("Unable to write: %s", err)

	}
	time.Sleep(1 * time.Second)

	var outbuf bytes.Buffer
	err = bob.GetData(cid, "Kitten", &outbuf)
	if err != nil {
		t.Fatal("Unable to read: %s", err)
	}
	if !bytes.Equal(outbuf.Bytes(), []byte("A GIF of a cute kitten")) {
		t.Fatal("Invalid file: %v", outbuf.Bytes())
	}
	bob.Log.Printf("Received kitten picture: %s", string(outbuf.Bytes()))

	alice.Stop()
	bob.Stop()
}
