package rendezvous

import (
	"h0tb0x/crypto"
	"h0tb0x/test"
	"h0tb0x/transfer"
	. "launchpad.net/gocheck"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestRendezvousSuite struct {
	test.TestMgr
}

func init() {
	Suite(&TestRendezvousSuite{})
}

func (this *TestRendezvousSuite) Test(c *C) {
	this.C = c

	rm := NewRendezvousMgr(this.ConnMgr, 3030, this.GetTempFile())
	rm.Start()
	defer rm.Stop()

	rc := NewClient(this.ConnMgr)

	ident := crypto.NewSecretIdentity("empty")
	err := rc.Put("http://localhost:3030", ident, "localhost", 31337)
	c.Assert(err, IsNil, Commentf("Put"))

	rec, err := rc.Get("http://localhost:3030", ident.Public().Fingerprint().String())
	c.Assert(err, IsNil, Commentf("Get"))

	c.Assert(rec.PublicKey, Equals, transfer.AsString(ident.Public()))
}
