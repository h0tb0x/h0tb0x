package db

import (
	_ "code.google.com/p/go-sqlite/go1/sqlite3"
	"database/sql"
	"fmt"
)

var (
	schemas = make(map[string]*Schema)
)

// Represents an open database connection
type Database struct {
	db      *sql.DB
	version int
}

type Schema struct {
	name       string
	latest     string
	migrations []string
}

// Represents a database row
type Row interface {
	Scan(vars ...interface{}) error
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
	this := &Database{db: db}
	this.apply(schema)
	return this
}

func (this *Database) apply(schema *Schema) {
	var version int
	row := this.SingleQuery("PRAGMA schema_version")
	this.Scan(row, &version)
	if version == 0 {
		// initial install, use latest schema
		fmt.Printf("Installing latest schema %q\n", schema.name)
		this.Exec(schema.latest)
		this.db.Exec(fmt.Sprintf("PRAGMA user_version = %d;", len(schema.migrations)))
	} else {
		row := this.SingleQuery("PRAGMA user_version")
		this.Scan(row, &this.version)

		fmt.Printf("Schema %q version is: %d\n", schema.name, this.version)
		this.migrate(schema)
	}
	row = this.SingleQuery("PRAGMA user_version")
	this.Scan(row, &version)

	if version != this.version {
		this.version = version
		fmt.Printf("Schema %q now at version: %d\n", schema.name, this.version)
	}
}

func (this *Database) migrate(schema *Schema) {
	target := len(schema.migrations)
	if this.version == target {
		return
	}
	if this.version == 0 {
		// HACK: migration from install party
		this.db.Exec("PRAGMA user_version = 1;")
		this.version = 1
	}
	fmt.Printf("Migrating schema %q from version %d to version %d\n",
		schema.name, this.version, target)
	tx, err := this.db.Begin()
	if err != nil {
		panic(err)
	}
	for i := this.version; i < target; i++ {
		fmt.Printf("Applying version %d\n", i+1)
		sql := schema.migrations[i]
		_, err := tx.Exec(sql)
		if err != nil {
			tx.Rollback()
			panic(err)
		}
		tx.Exec(fmt.Sprintf("PRAGMA user_version = %d;", i+1))
	}
	tx.Commit()
}

// Does a proper close of the database.
func (this *Database) Close() {
	this.db.Close()
}

// Executes a SQL statement.
func (this *Database) Exec(sql string, args ...interface{}) {
	//fmt.Printf("About to EXEC\n")
	_, err := this.db.Exec(sql, args...)
	//fmt.Printf("Done EXEC\n")
	if err != nil {
		panic(err)
	}
}

// Runs a query which should always return at most one row.
func (this *Database) SingleQuery(sql string, args ...interface{}) *sql.Row {
	return this.db.QueryRow(sql, args...)
}

// Runs a query which may return 0-n rows.
func (this *Database) MultiQuery(sql string, args ...interface{}) *sql.Rows {
	rows, err := this.db.Query(sql, args...)
	if err != nil {
		panic(err)
	}
	return rows
}

// Scans a row and panics if there are any problems.
func (this *Database) Scan(row Row, vars ...interface{}) {
	err := row.Scan(vars...)
	if err != nil {
		panic(err)
	}
}

// Scans a row (if any) and returns true, panics on problems, returns false on no row.
func (this *Database) MaybeScan(row Row, vars ...interface{}) bool {
	err := row.Scan(vars...)
	if err == sql.ErrNoRows {
		return false
	}
	if err != nil {
		panic(err)
	}
	return true
}
