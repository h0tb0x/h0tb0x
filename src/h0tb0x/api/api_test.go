package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"h0tb0x/base"
	"h0tb0x/data"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"
)

func NewTestNode(name string, linkPort uint16, apiPort uint16) *ApiMgr {
	base := base.NewBase(name, linkPort)
	link := link.NewLinkMgr(base)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := data.NewDataMgr("/tmp/data/"+name, meta)
	api := NewApiMgr("localhost:3030", apiPort, data)
	api.SetExt(net.ParseIP("127.0.0.1"), linkPort)
	api.Run()
	return api
}

func SafeGet(client *http.Client, url string) *http.Response {
	resp, err := client.Get(url)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("Invalid status: %v", resp.Status))
	}
	return resp
}

func SafePost(client *http.Client, url string, data interface{}) *http.Response {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		panic(err)
	}
	resp, err := client.Post(url, "application/json", &buf)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("Invalid status: %v", resp.Status))
	}
	return resp
}

func SafePut(client *http.Client, url string, data interface{}) *http.Response {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	if err != nil {
		panic(err)
	}
	req, _ := http.NewRequest("PUT", url, &buf)
	req.Header.Add("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		panic(fmt.Errorf("Invalid status: %v", resp.Status))
	}
	return resp
}

func SafeDecode(in io.Reader, obj interface{}) {
	err := json.NewDecoder(in).Decode(obj)
	if err != nil {
		panic(err)
	}
}

func TestApi(t *testing.T) {
	os.Remove("/tmp/rtest.db")
	rm := rendezvous.NewRendezvousMgr(3030, "/tmp/rtest.db")
	rm.Run()

	alice := NewTestNode("Alice", 10001, 2001)
	bob := NewTestNode("Bob", 10002, 2002)

	alice.Run()
	bob.Run()

	client := &http.Client{}
	resp := SafeGet(client, "http://localhost:2001/api/self")
	var selfAlice SelfJson
	SafeDecode(resp.Body, &selfAlice)
	SafePut(client, "http://localhost:2002/api/friends/"+selfAlice.Id,
		&FriendJson{SelfJson: SelfJson{Passport: selfAlice.Passport}})

	resp = SafeGet(client, "http://localhost:2002/api/self")
	var selfBob SelfJson
	SafeDecode(resp.Body, &selfBob)
	SafePut(client, "http://localhost:2001/api/friends/"+selfBob.Id,
		&FriendJson{SelfJson: SelfJson{Passport: selfBob.Passport}})

	time.Sleep(1 * time.Second)

	resp = SafePost(client, "http://localhost:2001/api/collections", "")
	var cj *CollectionJson 
	SafeDecode(resp.Body, &cj)
	cid := cj.Id
	alice.Log.Printf("Made a new collection: %s", cid)
	resp = SafePost(client, "http://localhost:2001/api/invites",
		&InviteJson{Cid: cid, Friend: selfBob.Id})
	resp = SafePost(client, "http://localhost:2002/api/invites",
		&InviteJson{Cid: cid, Friend: selfAlice.Id})

	SafePut(client, "http://localhost:2001/api/collections/"+cid+"/data/some_key", "SomeJsonCrap")

	time.Sleep(1 * time.Second)

	var keys []CollectionItemJson
	resp = SafeGet(client, "http://localhost:2002/api/collections/"+cid+"/data")
	SafeDecode(resp.Body, &keys)
	bob.Log.Printf("Got keys: %v", keys)

	var r string
	resp = SafeGet(client, "http://localhost:2002/api/collections/"+cid+"/data/some_key")
	SafeDecode(resp.Body, &r)

	bob.Log.Printf("GOT: %s", r)
	if r != "SomeJsonCrap" {
		t.Fatal("Invalid result: %s", r)
	}

	alice.Stop()
	bob.Stop()
	rm.Stop()
}
