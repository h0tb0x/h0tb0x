package rendezvous

import (
	"h0tb0x/crypto"
	"h0tb0x/transfer"
	"io/ioutil"
	"net/http/httptest"
	"os"
	"testing"
)

func TestRendezvous(t *testing.T) {
	tmp, err := ioutil.TempFile("", "rtest")
	if err != nil {
		t.Fatalf("Could not create temp db file", err)
	}
	tmpName := tmp.Name()
	tmp.Close()
	defer os.Remove(tmpName)
	rm := NewRendezvousMgr(0, tmpName)
	ts := httptest.NewServer(rm.server.Handler)
	defer ts.Close()

	ident := crypto.NewSecretIdentity("empty")
	err = Publish(ts.URL, ident, "localhost", 31337)
	if err != nil {
		t.Fatalf("Unable to write record: %s", err)
	}
	t.Logf("Getting")
	rec2, err := GetRendezvous(ts.URL, ident.Public().Fingerprint().String())
	if err != nil {
		t.Fatalf("Unable to read record: %s", err)
	}
	if rec2.PublicKey != transfer.AsString(ident.Public()) {
		t.Fatalf("Stuff doesn't match")
	}
	t.Logf("Got: %v", rec2)
}
