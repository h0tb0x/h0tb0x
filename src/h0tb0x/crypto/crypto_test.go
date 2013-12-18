package crypto

import (
	"h0tb0x/transfer"
	"testing"
)

func TestDigest(t *testing.T) {
	hasher1 := NewHasher()
	hasher2 := NewHasher()
	hasher1.Write([]byte("Hello world"))
	hasher2.Write([]byte("Test123"))
	hash1 := hasher1.Finalize()
	hash2 := hasher2.Finalize()
	enc1, err1 := transfer.EncodeString(hash1)
	enc2, err2 := transfer.EncodeString(hash2)
	if hash1.Equal(hash2) {
		t.Fatal("Hashes match")
	}
	if err1 != nil || err2 != nil {
		t.Fatalf("Error encoding hashes")
	}
	t.Logf("Hash of 'Hello world' is: %s", enc1)
	t.Logf("Hash of 'Test123' is: %s", enc2)
	var dh1 *Digest
	err := transfer.DecodeString(enc1, &dh1)
	if err != nil {
		t.Fatalf("Error decoding hash: %s", err)
	}
	if !hash1.Equal(dh1) {
		t.Fatal("Hash didn't deserialize right")
	}
	hasher3 := NewHasher()
	hasher3.Write([]byte("Hello"))
	hasher3.Write([]byte(" "))
	hasher3.Write([]byte("world"))
	hash3 := hasher3.Finalize()
	if !hash1.Equal(hash3) {
		t.Fatal("Write alternate failed")
	}
}

func TestIdent(t *testing.T) {
	s1 := NewSecretIdentity("pass1")
	s2 := NewSecretIdentity("pass2")
	p1 := s1.Public()
	if s1.Fingerprint().Equal(s2.Fingerprint()) {
		t.Fatal("Two identies with same fingerprint")
	}
	if !s1.Fingerprint().Equal(p1.Fingerprint()) {
		t.Fatal("Public and private fingerprints don't match")
	}
	x509 := s1.X509Certificate()
	p3, err := PublicFromCert(x509)
	if err != nil {
		t.Fatalf("Error during cert parse: %s", err)
	}
	if !p3.Fingerprint().Equal(p1.Fingerprint()) {
		t.Fatal("Certificate round trip failed")
	}
	p1enc, err := transfer.EncodeString(p1)
	if err != nil {
		t.Fatalf("Unable to encode public key")
	}
	t.Logf("Public key #1: %s", p1enc)
	var p4 *PublicIdentity
	err = transfer.DecodeString(p1enc, &p4)
	if err != nil {
		t.Fatalf("Unable to decode public key")
	}
	if !p4.Fingerprint().Equal(p1.Fingerprint()) {
		t.Fatal("Round trip of public key fails")
	}
	s1l := s1.Lock()
	s1le, err := transfer.EncodeString(s1l)
	if err != nil {
		t.Fatal("Unable to write locked private key: %s", err)
	}
	t.Logf("Locked private key: %s", s1le)
	var s1ld *LockedIdentity
	err = transfer.DecodeString(s1le, &s1ld)
	if err != nil {
		t.Fatal("Unable to decode locked key")
	}
	_, err = UnlockSecretIdentity(s1ld, "badpass")
	if err == nil {
		t.Fatal("Was able to unlock secret id with the wrong key")
	}
	t.Logf("Tried to use bad password, got: %s", err)
	s3, err := UnlockSecretIdentity(s1ld, "pass1")
	if err != nil {
		t.Fatalf("Unable to unlock id with correct password: %s", err)
	}
	if !s1.Fingerprint().Equal(s3.Fingerprint()) {
		t.Fatal("Unlocked new private key doesn't match original")
	}
	hasher := NewHasher()
	hasher.Write([]byte("Hello world"))
	digest := hasher.Finalize()
	sig2 := s2.Sign(digest)
	sig3 := s3.Sign(digest)
	sig3s, err := transfer.EncodeString(sig3)
	if err != nil {
		t.Fatalf("Unable to encode signature: %s", err)
	}
	t.Logf("Here is a signature: %s", sig3s)
	var sig4 *Signature
	err = transfer.DecodeString(sig3s, &sig4)
	if p1.Verify(digest, sig2) {
		t.Fatal("Bad signature worked")
	}
	if !p1.Verify(digest, sig4) {
		t.Fatal("Good signature failed")
	}
}

func TestTinyMessage(t *testing.T) {
	sk1 := NewSymmetricKey()
	sk2 := NewSymmetricKey()
	m1 := uint64(123456)
	e1 := sk1.EncodeMessage(m1)
	e2 := sk1.EncodeMessage(m1)
	t.Logf("e1 = %s", e1.String())
	t.Logf("e2 = %s", e2.String())
	if e1.String() == e2.String() {
		t.Fatal("IV doesn't work")
	}
	m1d1, v1 := sk1.DecodeMessage(e1)
	m1d2, v2 := sk1.DecodeMessage(e2)
	_, v3 := sk2.DecodeMessage(e1)
	t.Logf("m1d1 = %d", m1d1)
	t.Logf("m1d2 = %d", m1d2)
	if m1d1 != m1 || !v1 {
		t.Fatal("Doesn't decode")
	}
	if m1d2 != m1 || !v2 {
		t.Fatal("Doesn't decode")
	}
	if v3 {
		t.Fatal("Decoding with wrong key works")
	}
}
