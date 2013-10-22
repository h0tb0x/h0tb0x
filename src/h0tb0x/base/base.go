package base

import (
	"h0tb0x/crypto"
	"h0tb0x/db"
	"log"
	"os"
	"sync"
)

type Base struct {
	Log   *log.Logger
	Db    *db.Database
	Ident *crypto.SecretIdentity
	Port  uint16
}

type RWLocker interface {
	Lock()
	Unlock()
	RLock()
	RUnlock()
}

type NoisyLocker struct {
	sync.RWMutex
	log *log.Logger
}

func NewNoisyLocker(prefix string) *NoisyLocker {
	return &NoisyLocker{log: log.New(os.Stderr, prefix, log.LstdFlags)}
}

func (this *NoisyLocker) Lock() {
	// this.log.Println("Locking")
	this.RWMutex.Lock()
	// this.log.Println("Locked")
}

func (this *NoisyLocker) Unlock() {
	// this.log.Println("Unlocking")
	this.RWMutex.Unlock()
	// this.log.Println("Unlocked")
}

func (this *NoisyLocker) RLock() {
	// this.log.Println("RLocking")
	this.RWMutex.RLock()
	// this.log.Println("RLocked")
}

func (this *NoisyLocker) RUnlock() {
	// this.log.Println("RUnlocking")
	this.RWMutex.RUnlock()
	// this.log.Println("RUnlocked")
}
