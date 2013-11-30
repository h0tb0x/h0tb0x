package db

import (
	_ "code.google.com/p/go-sqlite/go1/sqlite3"
	"database/sql"
	"fmt"
	"github.com/coopernurse/gorp"
	"log"
	"os"
)

var (
	schemas = make(map[string]*schema)
)

type schema struct {
	name       string
	latest     string
	migrations []string
	version    int64
}

type Database struct {
	*gorp.DbMap
	version int64
}

// Makes or opens a database at the path specified.
func NewDatabase(path, name string) *Database {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}
	schema, ok := schemas[name]
	if !ok {
		panic(fmt.Errorf("Unknown schema: %q", name))
	}
	dbmap := &gorp.DbMap{Db: db, Dialect: gorp.SqliteDialect{}}
	schema.apply(dbmap)
	dbmap.TraceOn("", log.New(os.Stderr, "", log.LstdFlags))
	return &Database{DbMap: dbmap, version: schema.version}
}

func (this *schema) apply(db *gorp.DbMap) {
	schemaVersion, err := db.SelectInt("PRAGMA schema_version")
	if err != nil {
		panic(err)
	}
	if schemaVersion == 0 {
		// initial install, use latest schema
		fmt.Printf("Installing latest schema %q\n", this.name)
		_, err := db.Exec(this.latest)
		if err != nil {
			panic(err)
		}
		_, err = db.Exec(fmt.Sprintf("PRAGMA user_version = %d;", len(this.migrations)))
		if err != nil {
			panic(err)
		}
	} else {
		this.version, err = db.SelectInt("PRAGMA user_version")
		if err != nil {
			panic(err)
		}

		fmt.Printf("Schema %q version is: %d\n", this.name, this.version)
		this.migrate(db)
	}
	newVersion, err := db.SelectInt("PRAGMA user_version")
	if err != nil {
		panic(err)
	}

	if this.version != newVersion {
		this.version = newVersion
		fmt.Printf("Schema %q now at version: %d\n", this.name, this.version)
	}
}

func (this *schema) migrate(db *gorp.DbMap) {
	target := int64(len(this.migrations))
	if this.version == target {
		return
	}
	if this.version == 0 {
		// HACK: migration from install party
		db.Exec("PRAGMA user_version = 1;")
		this.version = 1
	}
	fmt.Printf("Migrating schema %q from version %d to version %d\n",
		this.name, this.version, target)
	tx, err := db.Begin()
	if err != nil {
		panic(err)
	}
	for i := this.version; i < target; i++ {
		fmt.Printf("Applying version %d\n", i+1)
		sql := this.migrations[i]
		_, err := tx.Exec(sql)
		if err != nil {
			tx.Rollback()
			panic(err)
		}
		_, err = tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", i+1))
		if err != nil {
			tx.Rollback()
			panic(err)
		}
	}
	tx.Commit()
}
