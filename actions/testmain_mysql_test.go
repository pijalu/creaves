//go:build !sqlite
// +build !sqlite

package actions

import (
	"fmt"
	"os"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
)

// TestMain pins the default (untagged) test suite to the creaves_test MySQL
// database. models.DB is opened in the models package init(), which reads
// GO_ENV and defaults to development — so a plain `go test ./actions` would
// silently run full-stack fixtures (users, care templates, cares, configs)
// against the DEVELOPMENT database while searchTestDB() targets creaves_test,
// splitting the suite across two databases (and polluting dev data).
//
// When GO_ENV is not already "test", force it and repoint models.DB at the
// test database. Under the sqlite build tag, TestMain in
// webhook_pusher_test.go serves the minimal SQLite schema instead, so this
// file is excluded there.
func TestMain(m *testing.M) {
	if os.Getenv("GO_ENV") != "test" {
		os.Setenv("GO_ENV", "test")
		conn, err := pop.Connect("test")
		if err != nil {
			fmt.Printf("FAIL: cannot connect to the MySQL test database (creaves_test): %v\n"+
				"Start MySQL and create the test database, or run with GO_ENV=test explicitly.\n", err)
			os.Exit(1)
		}
		models.DB = conn
	}
	os.Exit(m.Run())
}
