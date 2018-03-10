package models

import (
	"os"
	"path"

	"github.com/mattes/migrate"

	"github.com/jinzhu/gorm"

	_ "github.com/jinzhu/gorm/dialects/postgres"
	_ "github.com/mattes/migrate/database/postgres"
	_ "github.com/mattes/migrate/source/file"
)

// TestDB is the database used for testing.
var TestDB *gorm.DB

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func init() {
	// Override the test DB URI with TEST_DB_URI!
	// -- DO NOT USE A PRODUCTION DATABASE; IT WILL BE WIPED --
	uri := os.Getenv("TEST_DB_URI")
	if uri == "" {
		uri = "postgres:///gubal_test?sslmode=disable"
	}

	// Connect to the database!
	db, err := gorm.Open("postgres", uri)
	must(err)
	TestDB = db.LogMode(true).Debug()

	// Drop everything in the database, then bring it back up again
	wd, err := os.Getwd()
	must(err)
	migr, err := migrate.New("file://"+path.Join(wd, "..", "migrations"), uri)
	must(err)
	must(migr.Drop())
	must(migr.Up())
	srcerr, dberr := migr.Close()
	must(srcerr)
	must(dberr)
}
