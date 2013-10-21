package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"h0tb0x/base"
	"h0tb0x/conn"
	"h0tb0x/data"
	"h0tb0x/link"
	"h0tb0x/meta"
	"h0tb0x/rendezvous"
	"h0tb0x/sync"
	"h0tb0x/test"
	. "launchpad.net/gocheck"
	"net"
	"net/http"
	"testing"
	"time"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestApiSuite struct {
	test.TestMgr
	rm *rendezvous.RendezvousMgr
}

func init() {
	Suite(&TestApiSuite{})
}

func (this *TestApiSuite) SetUpTest(c *C) {
	this.TestMgr.SetUpTest(c)
	this.rm = rendezvous.NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	c.Assert(this.rm.Start(), IsNil)
}

func (this *TestApiSuite) TearDownTest(c *C) {
	this.rm.Stop()
	this.TestMgr.TearDownTest(c)
}

type node struct {
	*base.Base
	c       *C
	client  *http.Client
	api     *ApiMgr
	tm      *test.TestMgr
	baseUrl string
}

func (this *TestApiSuite) NewTestNode(name string, linkPort uint16, apiPort uint16) *node {
	base := this.NewBase(name, linkPort)
	link := link.NewLinkMgr(base, this.ConnMgr)
	sync := sync.NewSyncMgr(link)
	meta := meta.NewMetaMgr(sync)
	data := data.NewDataMgr(this.GetTempDir(), meta)
	api := NewApiMgr("localhost:3030", apiPort, data, this.ConnMgr)
	api.SetExt(net.IPv4(127, 0, 0, 1), linkPort)
	api.Start()
	return &node{
		Base:    base,
		c:       this.C,
		client:  conn.NewHttpClient(this.ConnMgr),
		api:     api,
		baseUrl: fmt.Sprintf("http://localhost:%d", apiPort),
	}
}

func (this *node) Stop() {
	this.api.Stop()
}

func (this *node) get(url string, result interface{}) *http.Response {
	resp, err := this.client.Get(this.baseUrl + url)
	this.c.Assert(err, IsNil)
	this.c.Assert(resp.StatusCode, Equals, http.StatusOK)
	if result != nil {
		err = json.NewDecoder(resp.Body).Decode(result)
		this.c.Assert(err, IsNil)
	}
	return resp
}

func (this *node) post(url string, data interface{}, result interface{}) *http.Response {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	this.c.Assert(err, IsNil)
	resp, err := this.client.Post(this.baseUrl+url, "application/json", &buf)
	this.c.Assert(err, IsNil)
	this.c.Assert(resp.StatusCode, Equals, http.StatusOK)
	if result != nil {
		err = json.NewDecoder(resp.Body).Decode(result)
		this.c.Assert(err, IsNil)
	}
	return resp
}

func (this *node) put(url string, data interface{}, result interface{}) *http.Response {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(data)
	this.c.Assert(err, IsNil)
	req, _ := http.NewRequest("PUT", this.baseUrl+url, &buf)
	req.Header.Add("Content-Type", "application/json")
	resp, err := this.client.Do(req)
	this.c.Assert(err, IsNil)
	this.c.Assert(resp.StatusCode, Equals, http.StatusOK)
	if result != nil {
		err = json.NewDecoder(resp.Body).Decode(result)
		this.c.Assert(err, IsNil)
	}
	return resp
}

func (this *TestApiSuite) TestBasic(c *C) {
	this.C = c

	alice := this.NewTestNode("A", 10001, 2001)
	bob := this.NewTestNode("B", 10002, 2002)

	var selfAlice SelfJson
	alice.get("/api/self", &selfAlice)
	bob.post("/api/friends", &FriendJson{SelfJson: SelfJson{Passport: selfAlice.Passport}}, nil)

	var selfBob SelfJson
	bob.get("/api/self", &selfBob)
	alice.post("/api/friends", &FriendJson{SelfJson: SelfJson{Passport: selfBob.Passport}}, nil)

	time.Sleep(1 * time.Second)

	var cj CollectionJson
	alice.post("/api/collections", "", &cj)

	cid := cj.Id
	alice.Log.Printf("Made a new collection: %s", cid)
	alice.post("/api/invites", &InviteJson{Cid: cid, Friend: selfBob.Id}, nil)
	bob.post("/api/invites", &InviteJson{Cid: cid, Friend: selfAlice.Id}, nil)

	alice.put("/api/collections/"+cid+"/data/some_key", "SomeJsonCrap", nil)

	time.Sleep(1 * time.Second)

	var keys []CollectionItemJson
	bob.get("/api/collections/"+cid+"/data", &keys)
	bob.Log.Printf("Got keys: %v", keys)

	var r string
	bob.get("/api/collections/"+cid+"/data/some_key", &r)

	bob.Log.Printf("GOT: %s", r)
	c.Assert(r, Equals, "SomeJsonCrap")

	alice.Stop()
	bob.Stop()
}
