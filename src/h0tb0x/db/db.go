package db

import (
	"database/sql"
	_ "github.com/h0tb0x/go-sqlite/go1/sqlite3"
)

// Represents an open database connection
type Database struct {
	db *sql.DB
}

// Represents a database row
type Row interface {
	Scan(vars ...interface{}) error
}

// Makes or opens a database at the path specified.
func NewDatabase(path string) *Database {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		panic(err)
	}
	return &Database{db}
}

// Does a proper close of the database.
func (this *Database) Close() {
	this.db.Close()
}

// Sets up the schema (if needed).
func (this *Database) Install() {
	this.Exec(string(schema_sql()))
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
