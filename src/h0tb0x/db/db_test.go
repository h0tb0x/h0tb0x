package db

import (
	"io/ioutil"
	. "launchpad.net/gocheck"
	"os"
	"testing"
)

// Hook up gocheck into the "go test" runner.
func Test(t *testing.T) { TestingT(t) }

type TestDbSuite struct {
	path string
}

func init() {
	Suite(&TestDbSuite{})
}

func (this *TestDbSuite) SetUpTest(c *C) {
	schemas["test"] = &schema{
		name: "test",
		latest: `
CREATE TABLE Foo(
	id INT NOT NULL,
	name TEXT
)
`,
		migrations: []string{
			`
CREATE TABLE Foo(
	id INT NOT NULL
)
`,
			`
ALTER TABLE Foo ADD COLUMN name TEXT;
`,
		},
	}

	ftmp, err := ioutil.TempFile("", "")
	c.Assert(err, IsNil)
	defer ftmp.Close()
	this.path = ftmp.Name()
}

func (this *TestDbSuite) TearDownTest(c *C) {
	os.Remove(this.path)
}

func (this *TestDbSuite) TestNew(c *C) {
	db := NewDatabase(this.path, "test")
	c.Assert(db.version, Equals, int64(2))
}

func (this *TestDbSuite) TestMigration(c *C) {
	db := NewDatabase(this.path, "test")
	c.Assert(db.version, Equals, int64(2))

	latest := `
CREATE TABLE Foo(
	id INT NOT NULL,
	name TEXT,
	author TEXT
)
	`

	migration := `
ALTER TABLE Foo ADD COLUMN author TEXT;
	`

	schema := schemas["test"]
	schema.latest = latest
	schema.migrations = append(schema.migrations, migration)
	db = NewDatabase(this.path, "test")
	c.Assert(db.version, Equals, int64(3))
}

func (this *TestDbSuite) TestBroken(c *C) {
	schemas["broken"] = &schema{
		name: "broken",
		latest: `
CREATE TABLE Foo(
	id INT NOT NULL
)
`,
		migrations: []string{
			`
CREATE TABLE Foo(
	id INT NOT NULL
)
`,
		},
	}

	// start with broken install (like from initial install party)
	db := NewDatabase(this.path, "broken")
	c.Assert(db.version, Equals, int64(1))

	db = NewDatabase(this.path, "test")
	c.Assert(db.version, Equals, int64(2))

	latest := `
CREATE TABLE Foo(
	id INT NOT NULL,
	name TEXT,
	author TEXT
)
	`

	migration := `
ALTER TABLE Foo ADD COLUMN author TEXT;
	`

	schema := schemas["test"]
	schema.latest = latest
	schema.migrations = append(schema.migrations, migration)
	db = NewDatabase(this.path, "test")
	c.Assert(db.version, Equals, int64(3))
}
