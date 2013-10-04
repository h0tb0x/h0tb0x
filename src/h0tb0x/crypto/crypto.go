package crypto

import (
	"bytes"
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"fmt"
	"github.com/h0tb0x/go.crypto/scrypt"
	"h0tb0x/transfer"
	"hash"
	"io"
	"math/big"
	"time"
)

// Digest represents the output of a cryptographic hash function.
type Digest struct{ impl []byte }

// Implements the h0tb0x transfer protocol
func (this *Digest) Encode(stream io.Writer) error {
	_, err := stream.Write(this.impl)
	return err
}

// Implements the h0tb0x transfer protocol
func (this *Digest) Decode(stream io.Reader) error {
	this.impl = make([]byte, 28)
	_, err := io.ReadFull(stream, this.impl)
	return err
}

// Compares two digests for equality
func (this *Digest) Equal(other *Digest) bool { return bytes.Equal(this.impl, other.impl) }

// Converts to a string
func (this *Digest) String() string { return transfer.AsString(this) }

// Returns as a series of bytes
func (this *Digest) Bytes() []byte { return transfer.AsBytes(this) }

// Signature represents the signature of some data signed by an Identity.
// It supports h0tb0x.transfer.
type Signature struct{ impl []byte }

// Implements the h0tb0x transfer protocol
func (this *Signature) Encode(stream io.Writer) error { return transfer.Encode(stream, this.impl) }

// Implements the h0tb0x transfer protocol
func (this *Signature) Decode(stream io.Reader) error { return transfer.Decode(stream, &this.impl) }

// SKSignature represents the signature of some data signed by a Symmetric Key.
// It supports h0tb0x.transfer.
type SKSignature struct{ impl []byte }

// Implements the h0tb0x transfer protocol
func (this *SKSignature) Encode(stream io.Writer) error { return transfer.Encode(stream, this.impl) }

// Implements the h0tb0x transfer protocol
func (this *SKSignature) Decode(stream io.Reader) error { return transfer.Decode(stream, &this.impl) }

// EncryptedKey represents a Symmetric key encrypted to a Identity.
// It supports h0tb0x.transfer.
type EncryptedKey struct{ impl []byte }

// Implements the h0tb0x transfer protocol
func (this *EncryptedKey) Encode(stream io.Writer) error { return transfer.Encode(stream, this.impl) }

// Implements the h0tb0x transfer protocol
func (this *EncryptedKey) Decode(stream io.Reader) error { return transfer.Decode(stream, &this.impl) }

// LockedIdentity represents a encrypted verison of an Identity protected by a password.
// It is safe to serialize
type LockedIdentity struct{ impl []byte }

// Implements the h0tb0x transfer protocol
func (this *LockedIdentity) Encode(stream io.Writer) error { return transfer.Encode(stream, this.impl) }

// Implements the h0tb0x transfer protocol
func (this *LockedIdentity) Decode(stream io.Reader) error { return transfer.Decode(stream, &this.impl) }

// Hasher represents a cryptographic hashing function which can produce a digest.
type Hasher interface {
	// Write can be used to send data to the hasher
	io.Writer
	// Finalizes computes the Digest of the entire data sent to the Hasher
	Finalize() (hash *Digest)
}

// Crypter represents an object which processes bytes to encrypt or decrypt them
type Crypter interface {
	// Process takes some bytes in and returns some bytes out, however the number of bytes
	// in may not match the number out in general, due to issues such as block size
	Process(in []byte) (out []byte)
	// Finalize gets any final output bytes
	Finalize() (out []byte)
}

// PublicIdentity represents the public part of an identity
type PublicIdentity struct {
	key *rsa.PublicKey
}

// SecretIdentity represents the secret (and public) part of an identity
// It cannot be transfered, only it's locked version may be serialized
type SecretIdentity struct {
	key      *rsa.PrivateKey
	password string
}

// SymmetricKey represents a shared or session secret.
// It cannot be transfered, only encrypted versions may be serialized.
// Currently, no methods, but someday it will include Encrypt, Decrypt, Sign, and Verify
type SymmetricKey struct {
	key []byte
	//Encrypt() (io Crypter)  // IV is placed in cypter stream as first bytes
	//Decrypt() (io Crypter)  // Expects IV as first byte
	//Sign(hash *Digest) (signature *SKSignature)
	//Verify(hash *Digest, signature *SKSignature) (valid bool)
}

type implHasher struct {
	impl hash.Hash
}

func (this *implHasher) Write(data []byte) (int, error) {
	return this.impl.Write(data)
}

func (this *implHasher) Finalize() *Digest {
	return &Digest{impl: this.impl.Sum(nil)}
}

// Makes a new Hasher
func NewHasher() Hasher {
	return &implHasher{impl: sha256.New224()}
}

// Encrypts a symmetric key to this identity
func (this *PublicIdentity) Encrypt(key *SymmetricKey) (ek *EncryptedKey) {
	out, err := rsa.EncryptOAEP(sha256.New224(), rand.Reader, this.key, key.key, nil)
	if err != nil {
		panic(err)
	}
	return &EncryptedKey{impl: out}
}

// Encrypts a symmetric key to this identity
func (this *SecretIdentity) Encrypt(key *SymmetricKey) (ek *EncryptedKey) {
	return (&PublicIdentity{key: &this.key.PublicKey}).Encrypt(key)
}

// Verifies that sig is the signature of digest by this identity
func (this *PublicIdentity) Verify(digest *Digest, sig *Signature) (valid bool) {
	err := rsa.VerifyPKCS1v15(this.key, crypto.SHA224, digest.impl, sig.impl)
	return (err == nil)
}

// Verifies that sig is the signature of digest by this identity
func (this *SecretIdentity) Verify(digest *Digest, sig *Signature) (valid bool) {
	return (&PublicIdentity{key: &this.key.PublicKey}).Verify(digest, sig)
}

// Computes a cryptographic fingerprint of this identity
func (this *PublicIdentity) Fingerprint() (fingerprint *Digest) {
	hash := sha256.New224()
	data, err := x509.MarshalPKIXPublicKey(this.key)
	if err != nil {
		panic(err)
	}
	hash.Write(data)
	return &Digest{impl: hash.Sum(nil)}
}

// Implements the h0tb0x transfer protocol
func (this *PublicIdentity) Decode(stream io.Reader) error {
	var data []byte
	transfer.Decode(stream, &data)
	pub, err := x509.ParsePKIXPublicKey(data)
	if err != nil {
		return err
	}
	rsakey, ok := pub.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("Public key in decode was not RSA")
	}
	this.key = rsakey
	return nil
}

// Implements the h0tb0x transfer protocol
func (this *PublicIdentity) Encode(stream io.Writer) error {
	data, err := x509.MarshalPKIXPublicKey(this.key)
	if err != nil {
		return err
	}
	return transfer.Encode(stream, data)
}

// Computes a cryptographic fingerprint of this identity
func (this *SecretIdentity) Fingerprint() (fingerprint *Digest) {
	return (&PublicIdentity{key: &this.key.PublicKey}).Fingerprint()
}

// Signs a digest using this Identity
func (this *SecretIdentity) Sign(digest *Digest) (sig *Signature) {
	sigout, err := rsa.SignPKCS1v15(rand.Reader, this.key, crypto.SHA224, digest.impl)
	if err != nil {
		panic(err)
	}
	return &Signature{impl: sigout}
}

// Decrypts a symmetric key encrypted to this Identity
func (this *SecretIdentity) Decrypt(ek *EncryptedKey) (key *SymmetricKey) {
	out, err := rsa.DecryptOAEP(sha256.New224(), rand.Reader, this.key, ek.impl, nil)
	if err != nil {
		panic(err)
	}
	return &SymmetricKey{key: out}
}

// Creation a new Identity
func NewSecretIdentity(password string) *SecretIdentity {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		panic(err)
	}
	return &SecretIdentity{key: key, password: password}
}

// Lock a key for safe serialization
// Use scrypt, keep params fixed for now
func (this *SecretIdentity) Lock() (id *LockedIdentity) {
	salt := make([]byte, 16)
	// I think IV is actually worthless here since *key* differs every time
	// due to salt, but whatever, better safe than sorry.
	iv := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, salt)
	if err != nil {
		panic(err)
	}
	_, err = io.ReadFull(rand.Reader, iv)
	if err != nil {
		panic(err)
	}
	//fmt.Printf("IV = %v\n", iv)
	//fmt.Printf("SALT = %v\n", salt)
	dk, err := scrypt.Key([]byte(this.password), salt, 16384, 8, 1, 32)
	if err != nil {
		panic(err)
	}
	//fmt.Printf("DK = %v\n",dk)
	flat := x509.MarshalPKCS1PrivateKey(this.key)
	hasher := NewHasher()
	hasher.Write(flat)
	digest := hasher.Finalize()
	//fmt.Printf("DIGEST = %v\n",digest.impl)
	ac, err := aes.NewCipher(dk)
	if err != nil {
		panic(err)
	}
	stream := cipher.NewOFB(ac, iv)
	//fmt.Printf("FLAT ~= %v\n",flat[:10])
	stream.XORKeyStream(flat, flat)
	//fmt.Printf("CRYPTOFLAT ~= %v\n", flat[:10])
	final := []byte{}
	final = append(final, salt...)
	final = append(final, iv...)
	final = append(final, digest.impl...)
	final = append(final, flat...)
	return &LockedIdentity{impl: final}
}

// Update the password for an unlocked key
func (this *SecretIdentity) ChangePassword(password string) {
	this.password = password
}

// Unlock a key with a password
func UnlockSecretIdentity(id *LockedIdentity, password string) (*SecretIdentity, error) {
	if len(id.impl) <= 60 {
		return nil, fmt.Errorf("Locked secret identity too short")
	}
	salt := id.impl[0:16]
	iv := id.impl[16:32]
	//fmt.Printf("IV = %v\n", iv)
	//fmt.Printf("SALT = %v\n", salt)
	digest := &Digest{impl: id.impl[32:60]}
	//fmt.Printf("DIGEST = %v\n", digest.impl)
	flat := append([]byte{}, id.impl[60:]...) // Copy it so I don't modify id
	dk, err := scrypt.Key([]byte(password), salt, 16384, 8, 1, 32)
	//fmt.Printf("DK = %v\n",dk)
	if err != nil {
		return nil, err
	}
	ac, err := aes.NewCipher(dk)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewOFB(ac, iv)
	//fmt.Printf("CRYPTOFLAT ~= %v\n", flat[:10])
	stream.XORKeyStream(flat, flat)
	//fmt.Printf("FLAT ~= %v\n", flat[:10])
	hasher := NewHasher()
	hasher.Write(flat)
	digest2 := hasher.Finalize()
	//fmt.Printf("DIGEST2 = %v\n", digest2.impl)
	if !digest.Equal(digest2) {
		return nil, fmt.Errorf("Unable to unlock secret identity, password incorrect or format mismatch")
	}
	key, err := x509.ParsePKCS1PrivateKey(flat)
	if err != nil {
		return nil, err
	}
	return &SecretIdentity{key: key, password: password}, nil
}

// Extract the public part of an Identity
func (this *SecretIdentity) Public() *PublicIdentity {
	return &PublicIdentity{&this.key.PublicKey}
}

// Generates an X509 cert from this identity
func (this *SecretIdentity) X509Certificate() *x509.Certificate {
	self := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "nobody"},
		NotBefore:             time.Now().AddDate(0, 0, -1),
		NotAfter:              time.Now().AddDate(1, 0, 0),
		KeyUsage:              x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:       true,
		MaxPathLen: 1,
	}
	der, err := x509.CreateCertificate(rand.Reader, self, self, &this.key.PublicKey, this.key)
	if err != nil {
		panic(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		panic(err)
	}
	return cert
}

// Generates an TLS cert from this identity
func (this *SecretIdentity) TlsCertificate() *tls.Certificate {
	cert := this.X509Certificate()
	return &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  this.key,
	}
}

// Generate a new public Identity from a x509 cert
func PublicFromCert(cert *x509.Certificate) (*PublicIdentity, error) {
	key, ok := cert.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("Trying to parse cert for non-RSA key")
	}
	return &PublicIdentity{key: key}, nil
}

// Makes a new random symmetric key
func NewSymmetricKey() *SymmetricKey {
	key := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, key)
	if err != nil {
		panic(err)
	}
	return &SymmetricKey{key: key}
}

// Simplified hashing for things which serialize via transfer.Encode, panics on error.
func HashOf(objs ...interface{}) *Digest {
	h := NewHasher()
	err := transfer.Encode(h, objs...)
	if err != nil {
		panic(err)
	}
	return h.Finalize()
}

// Returns a random string with 128 bits of entropy and char set [A-Z][a-z][0-9]_-
func RandomString() string {
	bytes := make([]byte, 16)
	_, err := io.ReadFull(rand.Reader, bytes)
	if err != nil {
		panic(err)
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

/*
func SerializeAndSign(out io.Writer, signer *SecretIdentity, objs ...interface{}) (*Signature, error) {
	h := NewHasher()
	w := io.MultiWriter(out, h)
	err := transfer.Encode(w, objs...)
	if (err != nil) { return nil, err }
	digest := h.Finalize()
	return signer.Sign(digest), nil
}
*/
