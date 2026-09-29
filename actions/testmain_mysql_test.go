//go:build !sqlite
// +build !sqlite

package actions

import (
	"fmt"
	"os"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"golang.org/x/crypto/bcrypt"
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
	// models' package init() ran before any TestMain with GO_ENV unset and
	// set the GLOBAL pop.Debug=true ("development"). Flip the test-only SQL
	// log mute (see models.QuietSQLLogs): under -race the per-statement log
	// formatting + stderr of the whole suite dominates the package runtime.
	models.QuietSQLLogs.Store(true)
	// Same for bcrypt: hundreds of fixture users hashed at DefaultCost is
	// tens of seconds of pure CPU under -race; logins stay real bcrypt
	// compares (models.PasswordHashCost).
	models.PasswordHashCost = bcrypt.MinCost

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
		sweepStaleFixtureRows(models.DB)
	}

	code := m.Run()

	// Belt and braces: never leak the worker past the test binary.
	StopWebhookWorker()
	os.Exit(code)
}

// sweepStaleFixtureRows deletes garbage left by earlier suite runs whose
// process was hard-killed before their t.Cleanup ran (bash timeouts, Ctrl-C).
// Fixture animals are recognizable by their marker cages (CP-<hex> for the
// plan fixtures, OTHER-<hex> for the "must not match" control animal);
// a single leaked CP-A3 skews any test that counts matches or plan items
// across the shared database (e.g. the matcher preview "only CP-A1
// matches"). Rows older than a day cannot belong to a live run — every
// fixture lives for seconds — so the age guard bounds the sweep even if a
// concurrent suite shares the database. Errors are ignored row by row:
// the sweep is best-effort hygiene, never a test failure.
func sweepStaleFixtureRows(db *pop.Connection) {
	dayAgo := "DATE_SUB(NOW(), INTERVAL 1 DAY)"
	cage := "a.cage LIKE 'CP-%' OR a.cage LIKE 'OTHER-%'"
	// animal-scoped rows of stale fixture animals
	db.RawQuery("DELETE cpa FROM care_plan_applications cpa JOIN animals a ON cpa.animal_id = a.id WHERE " + cage).Exec()
	db.RawQuery("DELETE c FROM cares c JOIN animals a ON c.animal_id = a.id WHERE " + cage).Exec()
	db.RawQuery("DELETE t FROM treatments t JOIN animals a ON t.animal_id = a.id WHERE " + cage).Exec()
	db.RawQuery("DELETE cap FROM care_animal_plans cap JOIN animals a ON cap.animal_id = a.id WHERE " + cage).Exec()
	db.RawQuery("DELETE e FROM treatment_time_entries e JOIN animals a ON e.animal_id = a.id WHERE " + cage).Exec()
	// the animals themselves, then orphaned provenance
	db.RawQuery("DELETE FROM animals WHERE cage LIKE 'CP-%' OR cage LIKE 'OTHER-%'").Exec()
	db.RawQuery("DELETE d FROM discoveries d LEFT JOIN animals a ON a.discovery_id = d.id WHERE a.id IS NULL").Exec()
	db.RawQuery("DELETE dc FROM discoverers dc LEFT JOIN discoveries d ON d.discoverer_id = dc.id WHERE d.id IS NULL").Exec()
	db.RawQuery("DELETE FROM intakes WHERE id NOT IN (SELECT intake_id FROM animals WHERE intake_id IS NOT NULL)").Exec()
	// fixture rules/matchers/applications abandoned by killed runs (older
	// than a day; live fixtures are seconds old and marker-unique)
	db.RawQuery("DELETE cre FROM care_rule_exclusions cre JOIN care_rules cr ON cre.rule_id = cr.id WHERE cr.updated_at < " + dayAgo).Exec()
	db.RawQuery("DELETE FROM care_rules WHERE updated_at < " + dayAgo).Exec()
	db.RawQuery("DELETE FROM care_matchers WHERE updated_at < " + dayAgo).Exec()
	db.RawQuery("DELETE FROM care_plan_applications WHERE created_at < " + dayAgo).Exec()
	db.RawQuery("DELETE FROM treatment_time_entries WHERE created_at < " + dayAgo).Exec()
	db.RawQuery("DELETE FROM treatments WHERE updated_at < " + dayAgo).Exec()
}
