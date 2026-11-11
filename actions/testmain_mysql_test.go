//go:build !sqlite
// +build !sqlite

package actions

import (
	"fmt"
	"os"
	"strings"
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
	if models.DB != nil && models.DB.Dialect.Name() != "sqlite3" && isDisposableTestDB(models.DB) {
		models.DB.RawQuery("DELETE FROM event_deliveries").Exec()
		models.DB.RawQuery("DELETE FROM event_streams").Exec()
		sweepStaleFixtureRows(models.DB)
	}

	code := m.Run()

	// Belt and braces: never leak the worker past the test binary.
	StopWebhookWorker()
	os.Exit(code)
}

// isDisposableTestDB reports whether db is the throwaway test database.
// TestMain only repoints models.DB when GO_ENV was NOT already "test", so
// `GO_ENV=development go test` leaves models.DB on the DEVELOPMENT database
// while the sweep below deletes rows in bulk. It must therefore refuse to run
// anywhere but a database whose name says test. An unreadable name is treated
// as "not disposable" — hygiene is best effort, never a reason to touch data.
func isDisposableTestDB(db *pop.Connection) bool {
	var row struct {
		Name string `db:"name"`
	}
	if err := db.RawQuery("SELECT DATABASE() AS name").First(&row); err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(row.Name), "test")
}

// animalScopedDeletes builds the DELETE for every table holding a foreign key
// to animals(id): care_animal_plans, care_plan_applications,
// care_rule_exclusions, cares, travels, treatments, veterinaryvisits — taken
// from information_schema rather than guessed. Leaving one out fails the
// animals DELETE on a foreign key and, because the sweep ignores errors,
// silently keeps every leftover.
//
// join is a function of the table alias so the caller can append a predicate
// verbatim: building it with fmt.Sprintf would treat the LIKE wildcards in
// that predicate as format verbs.
func animalScopedDeletes(join func(alias string) string) []string {
	return []string{
		"DELETE cpa FROM care_plan_applications cpa " + join("cpa"),
		"DELETE c FROM cares c " + join("c"),
		"DELETE t FROM treatments t " + join("t"),
		"DELETE cap FROM care_animal_plans cap " + join("cap"),
		"DELETE e FROM treatment_time_entries e " + join("e"),
		"DELETE v FROM veterinaryvisits v " + join("v"),
		"DELETE tr FROM travels tr " + join("tr"),
		"DELETE cre FROM care_rule_exclusions cre " + join("cre"),
	}
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
	// The animal-search fixtures carry NO cage at all (createAnimalSearchFixtures
	// leaves it NULL), so the cage predicate above never matched them and they
	// accumulated until two runs drew the same (year, yearNumber) and the
	// fixture died on Error 1062 animals_year_yearNumber_idx. They are
	// recognizable by their discoverer: createAnimalSearchFixtures names it
	// "TS-<marker>". Same marker discipline as the care-plan fixtures, so this
	// can never match a real row. TEST-1.
	search := "(a.cage IS NULL OR a.cage = '') AND a.discovery_id IN (" +
		"SELECT d.id FROM discoveries d JOIN discoverers dc ON d.discoverer_id = dc.id WHERE dc.lastname LIKE 'TS-%')"
	// Same predicate for a single-table DELETE, where no `a` alias exists.
	searchNoAlias := "(cage IS NULL OR cage = '') AND discovery_id IN (" +
		"SELECT d.id FROM discoveries d JOIN discoverers dc ON d.discoverer_id = dc.id WHERE dc.lastname LIKE 'TS-%')"
	// animal-scoped rows of stale fixture animals
	for _, q := range animalScopedDeletes(func(al string) string {
		return "JOIN animals a ON " + al + ".animal_id = a.id WHERE " + cage
	}) {
		db.RawQuery(q).Exec()
	}
	// Same for the cage-less search fixtures: every animal-scoped dependent
	// must go first or the animals DELETE fails on a foreign key (the sweep
	// ignores errors, so the leftovers simply stayed — 651 of them).
	// Every table that declares a foreign key to animals(id) — enumerated from
	// information_schema, not guessed: care_animal_plans, care_plan_applications,
	// care_rule_exclusions, cares, travels, treatments, veterinaryvisits. Leaving
	// one out fails the animals DELETE on a foreign key and, because the sweep
	// ignores errors, silently keeps every leftover.
	for _, q := range animalScopedDeletes(func(al string) string {
		return "JOIN animals a ON " + al + ".animal_id = a.id WHERE " + search
	}) {
		db.RawQuery(q).Exec()
	}
	// animals.outtake_id references outtakes, so a fixture outtake can only go
	// once its animal is gone: collect the ids first, delete them after.
	var searchOuttakes []string
	db.RawQuery("SELECT DISTINCT outtake_id FROM animals WHERE " + searchNoAlias).All(&searchOuttakes)
	// the animals themselves, then orphaned provenance
	db.RawQuery("DELETE FROM animals WHERE cage LIKE 'CP-%' OR cage LIKE 'OTHER-%'").Exec()
	db.RawQuery("DELETE FROM animals WHERE " + searchNoAlias).Exec()
	for _, id := range searchOuttakes {
		if id != "" {
			db.RawQuery("DELETE FROM outtakes WHERE id = ?", id).Exec()
		}
	}
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

	// Fixture-shaped rule/matcher names all end in the run marker — the 8
	// hex characters of a fresh uuid ("R-9d8cf3d8-2231", "R2-1a2b3c4d",
	// "MLatch-1a2b3c4d", "R-fc-1a2b3c4d-1504") — which no seeded or
	// user-authored row carries, so they are swept regardless of age.
	// The age-only guard above left a killed run's rules in place for a
	// whole day, and a rule with NO matcher applies to EVERY animal: three
	// such leftovers (measured on creaves_test) broke seven care-plan tests
	// at once. Age is the wrong guard here; the marker is the guard (TEST-3).
	marked := "name REGEXP '-[0-9a-f]{8}(-[0-9]{4})?$'"
	db.RawQuery("DELETE cre FROM care_rule_exclusions cre JOIN care_rules cr ON cre.rule_id = cr.id WHERE cr." + marked).Exec()
	db.RawQuery("DELETE FROM care_rules WHERE " + marked).Exec()
	db.RawQuery("DELETE FROM care_matchers WHERE " + marked).Exec()

	// Reference rows of the animal-search fixtures. Their names end in the same
	// run marker, so the shape - not the age - is the guard (TEST-1).
	db.RawQuery("DELETE FROM outtaketypes WHERE name REGEXP '^TS(OutOK|OutErr)-[0-9a-f]{8}$'").Exec()
	db.RawQuery("DELETE FROM entry_causes WHERE cause REGEXP '^TSC[12]-[0-9a-f]{8}$'").Exec()
	db.RawQuery("DELETE FROM animaltypes WHERE name REGEXP '^TSType[12]-[0-9a-f]{8}$'").Exec()
	db.RawQuery("DELETE FROM animalages WHERE name REGEXP '^TSAge[12]-[0-9a-f]{8}$'").Exec()
}
