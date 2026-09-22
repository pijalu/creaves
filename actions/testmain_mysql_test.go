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

	// Bugs.md #10: the webhook worker is a process-global goroutine that
	// queries the shared models.DB connection on every wake/tick. If a test
	// starts it through a handler path (event publish, sync-target CRUD,
	// DLQ reset), it keeps running for the rest of the package run and its
	// DB access races with later tests' requests — the order-dependent
	// "invalid connection" / i/o-timeout 500s. No test in the MySQL suite
	// asserts live delivery (they all assert DB state directly), so implicit
	// starts are disabled suite-wide here; tests that need the worker start
	// it explicitly with StartWebhookWorker and stop it in their cleanup.
	webhookWorkerAutoStartDisabled.Store(true)

	// Drop delivery bookkeeping left over by earlier runs: stale
	// event_streams/event_deliveries rows slow every listing endpoint the
	// suite exercises (dashboard, exports) and skew DLQ assertions.
	// creaves_test is disposable by contract; the dev database is untouched.
	if models.DB != nil && models.DB.Dialect.Name() != "sqlite3" {
		models.DB.RawQuery("DELETE FROM event_deliveries").Exec()
		models.DB.RawQuery("DELETE FROM event_streams").Exec()
	}

	code := m.Run()

	// Belt and braces: never leak the worker past the test binary.
	StopWebhookWorker()
	os.Exit(code)
}
