package actions

import (
	"testing"

	"creaves/models"
)

// requireMySQLTestDB skips tests that drive the full application stack
// (App(), admin login, export/report queries) against the complete MySQL
// schema. Under the "sqlite" build tag, TestMain in webhook_pusher_test.go
// points models.DB at the minimal pusher_test.db (event_streams, resync_runs,
// config and the reference tables only), so full-stack fixtures would fail on
// missing tables (users, localities, ...). In the default (untagged) suite
// models.DB is the creaves_test MySQL database and these tests run normally.
func requireMySQLTestDB(t *testing.T) {
	t.Helper()
	if models.DB != nil && models.DB.Dialect != nil && models.DB.Dialect.Name() == "sqlite3" {
		t.Skip("MySQL-only: under the sqlite tag models.DB serves the minimal pusher schema")
	}
}
