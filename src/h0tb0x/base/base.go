package base

import (
	"h0tb0x/crypto"
	"h0tb0x/db"
	"log"
)

type Base struct {
	Log   *log.Logger
	Db    *db.Database
	Ident *crypto.SecretIdentity
	Port  uint16
}
