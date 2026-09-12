package grifts

import (
	"database/sql"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/gobuffalo/pop/v6"
)

// Migration-replay verification: the outtaketype migrations must apply
// cleanly to an EMPTY center database without any reference-dump UUID
// literals, and the portable code-based translation re-key must behave like
// the hardcoded version it replaced (legacy content wins on conflict).

const replayDBName = "creaves_migrate_replay"

func replayEnvOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// fizzSQLStatements extracts every sql("...") argument from a fizz migration.
func fizzSQLStatements(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(`(?s)sql\("((?:[^"\\]|\\.)*)"\)`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		out = append(out, strings.ReplaceAll(m[1], `\"`, `"`))
	}
	return out
}

// sqlFileStatements splits a raw .sql migration into individual statements,
// dropping -- comment lines.
func sqlFileStatements(t *testing.T, path string) []string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var lines []string
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "--") {
			continue
		}
		lines = append(lines, line)
	}
	var out []string
	for _, stmt := range strings.Split(strings.Join(lines, "\n"), ";\n") {
		if s := strings.TrimSpace(stmt); s != "" {
			out = append(out, s)
		}
	}
	return out
}

func mustExec(t *testing.T, c *pop.Connection, kind, stmt string) {
	t.Helper()
	if _, err := c.Store.Exec(stmt); err != nil {
		t.Fatalf("%s statement failed: %v\n%s", kind, err, stmt)
	}
}

// TestMigrationsReplayOnEmptyDatabase replays every migration against a
// freshly created empty MySQL database, then exercises the outtaketype
// re-key/normalize statements on a legacy-shaped dataset inside that scratch
// database. It fails if any migration (or the portable re-key) depends on
// hardcoded reference-dump UUIDs.
func TestMigrationsReplayOnEmptyDatabase(t *testing.T) {
	if testing.Short() {
		t.Skip("short mode: skipping migration replay")
	}
	admin, err := sql.Open("mysql", replayEnvOr("MIGRATION_REPLAY_ADMIN_DSN",
		"creaves:creaves@tcp(127.0.0.1:3306)/?parseTime=true&multiStatements=true"))
	if err != nil {
		t.Fatalf("connect admin: %v", err)
	}
	defer admin.Close()
	if _, err := admin.Exec("DROP DATABASE IF EXISTS " + replayDBName); err != nil {
		t.Fatalf("drop scratch db: %v", err)
	}
	if _, err := admin.Exec("CREATE DATABASE " + replayDBName + " CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci"); err != nil {
		t.Fatalf("create scratch db: %v", err)
	}
	defer func() {
		_, _ = admin.Exec("DROP DATABASE IF EXISTS " + replayDBName)
	}()

	c, err := pop.NewConnection(&pop.ConnectionDetails{
		Dialect: "mysql",
		URL: replayEnvOr("MIGRATION_REPLAY_URL",
			"mysql://creaves:creaves@(127.0.0.1:3306)/"+replayDBName+"?parseTime=true&multiStatements=true&readTimeout=30s"),
	})
	if err != nil {
		t.Fatalf("connection details: %v", err)
	}
	if err := c.Open(); err != nil {
		t.Fatalf("open scratch db: %v", err)
	}
	defer c.Close()

	// go test runs with the package directory as CWD; locate the repo's
	// migrations/ relative to it.
	migDir := ""
	for _, cand := range []string{"migrations", "../migrations", "../../migrations"} {
		if fi, err := os.Stat(filepath.Join(cand, "20210219013121_create_users.up.fizz")); err == nil && !fi.IsDir() {
			migDir = cand
			break
		}
	}
	if migDir == "" {
		t.Fatal("could not locate migrations directory")
	}
	fm, err := pop.NewFileMigrator(migDir, c)
	if err != nil {
		t.Fatalf("file migrator: %v", err)
	}
	fm.SchemaPath = "" // never overwrite migrations/schema.sql from the scratch DB
	if err := fm.Up(); err != nil {
		t.Fatalf("migrate up on empty database: %v", err)
	}

	// Every up-migration file must be recorded as applied.
	want, err := filepath.Glob(filepath.Join(migDir, "*.up.*"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	mtn := c.MigrationTableName()
	var got int
	if err := c.Store.Get(&got, "SELECT COUNT(*) FROM "+mtn); err != nil {
		t.Fatalf("count %s: %v", mtn, err)
	}
	if got != len(want) {
		t.Fatalf("applied %d migrations, want %d", got, len(want))
	}

	codeStmts := fizzSQLStatements(t, filepath.Join(migDir, "20260912120000_add_outtaketype_code.up.fizz"))
	if len(codeStmts) != 11 {
		t.Fatalf("expected 11 sql statements in add_outtaketype_code, got %d", len(codeStmts))
	}
	// The outtaketype re-key + normalize statements must work on a
	// legacy-shaped center: English-named rows (which receive codes) plus
	// French dump-named orphan rows carrying the shipped translations.
	seed := func(q string) {
		t.Helper()
		mustExec(t, c, "seed", q)
	}
	seed(`INSERT INTO outtaketypes (id, name, code, def, dead, error, rating, created_at, updated_at) VALUES
		('a1000000-0000-4000-8000-000000000001', 'Relacher', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('a1000000-0000-4000-8000-000000000002', 'DCD', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('a1000000-0000-4000-8000-000000000003', 'Euthanasier', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('a1000000-0000-4000-8000-000000000004', 'Transferer', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('b1000000-0000-4000-8000-000000000001', 'Relaché', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('b1000000-0000-4000-8000-000000000002', 'Décédé', NULL, 0, 0, 0, 0, NOW(), NOW()),
		('b1000000-0000-4000-8000-000000000004', 'Transféré', NULL, 0, 0, 0, 0, NOW(), NOW())`)
	seed(`INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES
		('c1000000-0000-4000-8000-000000000001', 'outtaketypes', 'b1000000-0000-4000-8000-000000000001', 'name', 'fr', 'Relaché fr', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000002', 'outtaketypes', 'b1000000-0000-4000-8000-000000000001', 'name', 'en-US', 'Released', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000003', 'outtaketypes', 'b1000000-0000-4000-8000-000000000002', 'name', 'fr', 'Décédé fr', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000004', 'outtaketypes', 'b1000000-0000-4000-8000-000000000002', 'name', 'en-US', 'Deceased', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000005', 'outtaketypes', 'a1000000-0000-4000-8000-000000000002', 'name', 'de', 'DCD de', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000006', 'outtaketypes', 'b1000000-0000-4000-8000-000000000004', 'name', 'fr', 'Transféré fr', NOW(), NOW()),
		('c1000000-0000-4000-8000-000000000007', 'outtaketypes', 'a1000000-0000-4000-8000-000000000002', 'name', 'fr', 'DCD cible fr', NOW(), NOW())`)

	// Run the whole portable pipeline on the seeded legacy shape: code
	// assignment + swap (the replay pass ran them before seeding, so re-run),
	// the name-based translation re-key, the DCD translation repair, and the
	// reviewed data migration.
	for _, stmt := range codeStmts[1:] { // skip add_column DDL (already applied)
		if strings.HasPrefix(stmt, "DELETE FROM outtaketypes") {
			continue // no OT-named rows in this fixture
		}
		mustExec(t, c, "code-assign", stmt)
	}
	for _, stmt := range fizzSQLStatements(t, filepath.Join(migDir, "20261002090000_repair_dcd_translations.up.fizz")) {
		mustExec(t, c, "repair", stmt)
	}
	for _, stmt := range sqlFileStatements(t, filepath.Join(migDir, "20261002091000_normalize_outtaketype_data.up.sql")) {
		mustExec(t, c, "normalize", stmt)
	}

	// Codes assigned to the English-named rows, post OT1<->OT2 / OT3<->OT4 swap.
	for _, tc := range []struct{ name, code string }{
		{"Relacher", "OT1"}, {"DCD", "OT2"}, {"Euthanasier", "OT3"}, {"Transferer", "OT4"},
	} {
		var n int
		if err := c.Store.Get(&n, "SELECT COUNT(*) FROM outtaketypes WHERE name = ? AND code = ?", tc.name, tc.code); err != nil || n != 1 {
			t.Fatalf("row %s: want 1 with code %s (n=%d err=%v)", tc.name, tc.code, n, err)
		}
	}

	// Translations re-keyed by code: orphan dump rows drained onto the coded
	// rows, legacy content winning the (field, locale) conflict.
	check := func(wantRecord, wantLocale, wantValue string) {
		t.Helper()
		var gotValue string
		err := c.Store.Get(&gotValue, "SELECT value FROM translations WHERE table_name = 'outtaketypes' AND field = 'name' AND locale = ? AND record_id = ?", wantLocale, wantRecord)
		if err != nil {
			t.Fatalf("translation %s/%s: %v", wantRecord, wantLocale, err)
		}
		if gotValue != wantValue {
			t.Fatalf("translation %s/%s = %q, want %q", wantRecord, wantLocale, gotValue, wantValue)
		}
	}
	const (
		relacherID = "a1000000-0000-4000-8000-000000000001"
		dcdID      = "a1000000-0000-4000-8000-000000000002"
	)
	check(relacherID, "fr", "Relaché fr")  // moved from orphan 'Relaché' row
	check(relacherID, "en-US", "Released") // moved from orphan 'Relaché' row
	check(dcdID, "de", "DCD de")           // target copy on DCD row preserved
	check(dcdID, "fr", "Décédé fr")        // moved from orphan 'Décédé' row
	check(dcdID, "en-US", "Deceased")      // moved from orphan 'Décédé' row
	var transfererID string
	if err := c.Store.Get(&transfererID, "SELECT id FROM outtaketypes WHERE code = 'OT4'"); err != nil {
		t.Fatalf("OT4 lookup: %v", err)
	}
	check(transfererID, "fr", "Transféré fr") // moved from orphan 'Transféré' row

	// No translations may keep pointing at the orphan rows after the re-key.
	var orphans int
	if err := c.Store.Get(&orphans, "SELECT COUNT(*) FROM translations WHERE table_name = 'outtaketypes' AND record_id IN ('b1000000-0000-4000-8000-000000000001','b1000000-0000-4000-8000-000000000002','b1000000-0000-4000-8000-000000000004')"); err != nil || orphans != 0 {
		t.Fatalf("orphan-attached translations = %d, want 0 (err=%v)", orphans, err)
	}

	// Reviewed data migration: canonical display values and flags keyed by code.
	var ot struct {
		Name        string  `db:"name"`
		Description *string `db:"description"`
		Dead        bool    `db:"dead"`
		Error       bool    `db:"error"`
		Rating      int     `db:"rating"`
	}
	if err := c.Store.Get(&ot, "SELECT name, description, dead, error, rating FROM outtaketypes WHERE code = ?", "OT1"); err != nil {
		t.Fatalf("OT1 lookup: %v", err)
	}
	if ot.Name != "Relacher" || ot.Dead || ot.Error || ot.Rating != 1 {
		t.Fatalf("OT1 = %+v, want Relacher dead=false error=false rating=1", ot)
	}
	if err := c.Store.Get(&ot, "SELECT name, description, dead, error, rating FROM outtaketypes WHERE code = ?", "OT2"); err != nil {
		t.Fatalf("OT2 lookup: %v", err)
	}
	if ot.Name != "DCD" || !ot.Dead || ot.Error || ot.Rating != -1 {
		t.Fatalf("OT2 = %+v, want DCD dead=true error=false rating=-1", ot)
	}
	if ot.Description == nil || !strings.HasPrefix(*ot.Description, "Animal décédé naturellement") {
		t.Fatalf("OT2 description = %v, want canonical DCD description", ot.Description)
	}
}
