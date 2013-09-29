package rendezvous

import (
	"h0tb0x/crypto"
	"h0tb0x/transfer"
	"os"
	"testing"
)

func TestRendezvous(t *testing.T) {
	os.Remove("/tmp/rtest.db")
	rm := NewRendezvousMgr(3030, "/tmp/rtest.db")
	rm.Run()

	ident := crypto.NewSecretIdentity("empty")
	raddr := "localhost:3030"
	rec := &RecordJson{
		Version: 1,
		Host:    "localhost",
		Port:    31337,
	}
	rec.Sign(ident)
	t.Logf("Sending: %v", rec)
	err := PutRendezvous(raddr, rec)
	if err != nil {
		t.Fatalf("Unable to write record: %s", err)
	}
	t.Logf("Getting")
	rec2, err := GetRendezvous(raddr, ident.Public().Fingerprint().String())
	if err != nil {
		t.Fatalf("Unable to read record: %s", err)
	}
	if rec2.PublicKey != transfer.AsString(ident.Public()) {
		t.Fatalf("Stuff doesn't match")
	}
	t.Logf("Got: %v", rec2)

	rm.Stop()
}
