package base

import (
	"fmt"
	"h0tb0x/crypto"
	"h0tb0x/db"
	"log"
	"os"
)

type Base struct {
	Log   *log.Logger
	Db    *db.Database
	Ident *crypto.SecretIdentity
	Port  uint16
}

func NewBase(name string, port uint16) *Base {
	log := log.New(os.Stderr, fmt.Sprintf("[%v] ", name), log.LstdFlags)
	ident := crypto.NewSecretIdentity("")
	path := fmt.Sprintf("/tmp/%v.db", name)
	os.Remove(path)
	db := db.NewDatabase(path)
	db.Install()

	return &Base{
		Log:   log,
		Db:    db,
		Ident: ident,
		Port:  port,
	}
}
