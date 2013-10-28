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
	cm      conn.ConnMgr
	baseUrl string
	profile *CollectionJson
	self    *SelfJson
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
	node := &node{
		Base:    base,
		c:       this.C,
		client:  conn.NewHttpClient(this.ConnMgr, 0),
		api:     api,
		baseUrl: fmt.Sprintf("http://localhost:%d", apiPort),
		cm:      this.ConnMgr,
	}
	node.loadSelf()
	node.createProfile(name)
	return node
}

func (this *node) Stop() {
	this.api.Stop()
}

func (this *node) poll(url string, result interface{}, timeout time.Duration) *http.Response {
	client := conn.NewHttpClient(this.cm, timeout)
	resp, err := client.Get(this.baseUrl + url)
	this.c.Assert(err, IsNil)
	this.c.Assert(resp.StatusCode, Equals, http.StatusOK)
	if result != nil {
		err = json.NewDecoder(resp.Body).Decode(result)
		this.c.Assert(err, IsNil)
	}
	return resp
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

func (this *node) addFriend(node *node) *FriendJson {
	var friend FriendJson
	this.post("/api/friends", &PassportJson{Passport: node.self.Passport}, &friend)
	this.c.Assert(friend.Passport, Equals, node.self.Passport)
	this.c.Assert(friend.Id, Equals, node.self.Id)
	this.c.Assert(friend.Rendezvous, Equals, node.self.Rendezvous)
	this.shareProfile(node)
	return &friend
}

func (this *node) createProfile(name string) *CollectionJson {
	this.post("/api/collections", "", &this.profile)
	url := fmt.Sprintf("/api/collections/%v/data/profile", this.profile.Id)
	record := struct{ name string }{name}
	this.put(url, record, nil)
	return this.profile
}

func (this *node) loadSelf() *SelfJson {
	this.get("/api/self", &this.self)
	return this.self
}

func (this *node) shareProfile(node *node) {
	url := fmt.Sprintf("/api/friends/%v/invites", node.self.Id)
	this.post(url, &InviteJson{Cid: this.profile.Id}, nil)
}

func (this *node) getInvite(timeout time.Duration) *InviteJson {
	var invite InviteJson
	this.poll("/api/invites", &invite, timeout)
	return &invite
}

func (this *node) acceptInvite(invite *InviteJson) {
}

func (this *node) rejectInvite(invite *InviteJson) {
}

func (this *TestApiSuite) TestBasic(c *C) {
	this.C = c

	alice := this.NewTestNode("A", 10001, 2001)
	bob := this.NewTestNode("B", 10002, 2002)

	alice.addFriend(bob)
	bob.addFriend(alice)

	ai := alice.getInvite(0)
	c.Assert(ai, NotNil)

	alice.acceptInvite(ai)

	// var keys []CollectionItemJson
	// bob.get("/api/collections/"+cid+"/data", &keys)
	// bob.Log.Printf("Got keys: %v", keys)

	// var r string
	// bob.get("/api/collections/"+cid+"/data/some_key", &r)

	// bob.Log.Printf("GOT: %s", r)
	// c.Assert(r, Equals, "SomeJsonCrap")

	alice.Stop()
	bob.Stop()
}
