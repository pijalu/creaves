package grifts

import (
	"bytes"
	"compress/gzip"
	"strings"
	"testing"

	"creaves/models"
	"creaves/utils"

	"github.com/gobuffalo/nulls"

	"github.com/gofrs/uuid"
)

func TestNormKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Relacher", "relacher"},
		{"Relaché", "relache"},
		{"Transferer", "transferer"},
		{"Transféré", "transfere"},
		{"Euthanasier", "euthanasier"},
		{"Chauves-souris", "chauves souris"},
		{"Autres mammifères (Procyonidé, Viverridés, ...)", "autres mammiferes procyonide viverrides"},
		{"Mort à l'arrivée avant l'encodage", "mort a l arrivee avant l encodage"},
		{"Mort à l’arrivée avant l’encodage", "mort a l arrivee avant l encodage"},
		{"DCD", "dcd"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normKey(c.in); got != c.want {
			t.Errorf("normKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStartupAliasMergeIDsPrefersLegacyName(t *testing.T) {
	rows := []trRow{
		{ID: "canonical-id", Value: nulls.String{String: "Hérissons / Insectivore", Valid: true}},
		{ID: "legacy-id", Value: nulls.String{String: "Hérissons et mammifères insectivores", Valid: true}},
	}
	canonical := "Hérissons / Insectivore"
	retained, duplicates := startupAliasMergeIDs(rows, canonical, startupNameAliases["animaltypes"][canonical])
	if retained != "legacy-id" {
		t.Fatalf("retained ID = %q, want legacy row ID", retained)
	}
	if len(duplicates) != 1 || duplicates[0] != "canonical-id" {
		t.Fatalf("duplicates = %#v, want canonical row only", duplicates)
	}
}

func TestReptileAnimaltypeAlias(t *testing.T) {
	canonical := "Reptiles, Amphibiens"
	legacy := "Reptiles et Amphibiens"
	aliases := startupNameAliases["animaltypes"][canonical]
	if len(aliases) != 1 || aliases[0] != legacy {
		t.Fatalf("aliases for %q = %#v, want %q", canonical, aliases, legacy)
	}
	rows := []trRow{
		{ID: "canonical-id", Value: nulls.String{String: canonical, Valid: true}},
		{ID: "legacy-id", Value: nulls.String{String: legacy, Valid: true}},
	}
	retained, duplicates := startupAliasMergeIDs(rows, canonical, aliases)
	if retained != "legacy-id" || len(duplicates) != 1 || duplicates[0] != "canonical-id" {
		t.Fatalf("merge = retained %q, duplicates %#v; want legacy-id and canonical-id", retained, duplicates)
	}
}

func TestAnimaltypeLegacyAlias(t *testing.T) {
	canonical := "Hérissons / Insectivore"
	legacy := "Hérissons et mammifères insectivores"
	aliases := startupNameAliases["animaltypes"][canonical]
	found := false
	for _, alias := range aliases {
		if alias == legacy {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("animal type %q missing legacy alias %q", canonical, legacy)
	}
	if normKey(canonical) == normKey(legacy) {
		t.Fatalf("regression fixture must cover semantic, not textual, rename")
	}
}

func TestMatchRefName(t *testing.T) {
	index := map[string]string{
		"relacher":       "id-old-relacher",
		"euthanasier":    "id-old-eutha",
		"transferer":     "id-old-transfer",
		"adoption":       "id-old-adoption",
		"dcd":            "id-old-dcd",
		"grands oiseaux": "id-old-grands",
	}
	used := map[string]bool{}

	// Exact normalized match.
	if got := matchRefName(index, used, "relache"); got != "id-old-relacher" {
		t.Errorf("prefix match Relaché -> Relacher: got %q", got)
	}
	used["id-old-relacher"] = true

	// Claimed targets are never returned twice.
	if got := matchRefName(index, used, "relache"); got != "" {
		t.Errorf("used target must not match again, got %q", got)
	}

	// Exact match wins over anything else.
	if got := matchRefName(index, used, "adoption"); got != "id-old-adoption" {
		t.Errorf("exact match: got %q", got)
	}
	used["id-old-adoption"] = true

	// Too short for prefix matching (DCD vs Décédé) -> new item.
	if got := matchRefName(index, used, "decede"); got != "" {
		t.Errorf("short prefix must not match, got %q", got)
	}

	// No candidate at all -> new item.
	if got := matchRefName(index, used, "rongeurs"); got != "" {
		t.Errorf("unrelated name must not match, got %q", got)
	}

	// Ambiguous prefix matches -> new item (safety).
	ambiguous := map[string]string{"rapace": "a1", "rapaces": "a2"}
	if got := matchRefName(ambiguous, map[string]bool{}, "rapac"); got != "" {
		t.Errorf("ambiguous prefix must not match, got %q", got)
	}

	// Empty input -> no match.
	if got := matchRefName(index, used, ""); got != "" {
		t.Errorf("empty norm must not match, got %q", got)
	}
}

func TestReconcileFrTranslationsPreservesIdempotencyAndCorrections(t *testing.T) {
	oldID := uuid.Must(uuid.FromString("11111111-1111-1111-1111-111111111111"))
	existing := []models.Translation{{ID: oldID, TableName: "animaltypes", RecordID: "r1", Field: "name", Locale: "fr", Value: "Ancien"}}
	source := map[string][]trRow{"name": {{ID: "r1", Value: nulls.String{String: "Nouveau", Valid: true}}, {ID: "r2", Value: nulls.String{String: "Nouveau 2", Valid: true}}, {ID: "r3", Value: nulls.String{String: "", Valid: false}}}}
	inserts, updates := reconcileFrTranslations("animaltypes", []string{"name"}, existing, source)
	if len(inserts) != 1 || inserts[0].RecordID != "r2" {
		t.Fatalf("inserts = %#v", inserts)
	}
	if len(updates) != 1 || updates[0].ID != oldID || updates[0].Value != "Nouveau" {
		t.Fatalf("updates = %#v", updates)
	}
	inserts, updates = reconcileFrTranslations("animaltypes", []string{"name"}, append(existing, inserts...), source)
	if len(inserts) != 0 || len(updates) != 1 {
		t.Fatalf("second reconciliation: inserts=%d updates=%d", len(inserts), len(updates))
	}
}

func TestRowIDCaseInsensitive(t *testing.T) {
	if got := rowID(map[string]string{"ID": "SP1", "species": "x"}); got != "SP1" {
		t.Errorf("rowID uppercase ID: got %q", got)
	}
	if got := rowID(map[string]string{"id": "u1"}); got != "u1" {
		t.Errorf("rowID lowercase id: got %q", got)
	}
	if got := rowID(map[string]string{"name": "n"}); got != "" {
		t.Errorf("rowID without id: got %q, want empty", got)
	}
}

func TestRenameInsertUsesDeterministicNewID(t *testing.T) {
	// A dump row whose id is taken by a DIFFERENT canonical name is inserted
	// under a fresh id derived deterministically from table+name, so re-runs
	// produce the same id (idempotency) and artifacts can be re-keyed.
	dumpID := "b3e9ca56-b28e-407a-8b6d-5f39970d2a17"
	raw := "('" + dumpID + "','Autres mammifères (Procyonidé, Viverridés, ...)','desc','2025-01-04','2025-01-04')"
	newID := uuid.NewV5(startupRowNamespace, "animaltypes|Autres mammifères (Procyonidé, Viverridés, ...)").String()
	if newID == dumpID {
		t.Fatal("derived id equals the colliding dump id")
	}
	if again := uuid.NewV5(startupRowNamespace, "animaltypes|Autres mammifères (Procyonidé, Viverridés, ...)").String(); again != newID {
		t.Fatalf("uuid v5 not deterministic: %s vs %s", newID, again)
	}
	out := strings.Replace(raw, "'"+dumpID+"'", "'"+newID+"'", 1)
	if !strings.HasPrefix(out, "('"+newID+"'") {
		t.Errorf("leading id not rewritten: %q", out)
	}
	if !strings.Contains(out, "'Autres mammifères (Procyonidé, Viverridés, ...)'") {
		t.Errorf("name field damaged: %q", out)
	}
}

func TestSplitRawTuples(t *testing.T) {
	stmt := "INSERT INTO `t` VALUES ('a','b','c'),('x, y','p (q)','it\\'s'),(\"no\");"
	// Note: double-quoted strings are not SQL strings; keep the fixture valid:
	stmt = "INSERT INTO `t` VALUES ('a','b','c'),('x, y','p (q)','it\\'s'),('z');"
	tuples, err := splitRawTuples([]string{stmt})
	if err != nil {
		t.Fatalf("splitRawTuples: %v", err)
	}
	if len(tuples) != 1 || len(tuples[0]) != 3 {
		t.Fatalf("want 1 statement with 3 tuples, got %v", tuples)
	}
	if tuples[0][1] != "('x, y','p (q)','it\\'s')" {
		t.Errorf("raw tuple with quotes/comma/paren altered: %q", tuples[0][1])
	}

	if _, err := splitRawTuples([]string{"SELECT 1"}); err == nil {
		t.Error("expected error for statement without VALUES")
	}
	if _, err := splitRawTuples([]string{"INSERT INTO t VALUES ('unterminated"}); err == nil {
		t.Error("expected error for unbalanced tuples")
	}
}

// startupTranslationDumpValues parses the embedded dump and returns
// (table, id, field) -> canonical French column value.
func startupTranslationDumpValues(t *testing.T) map[string]string {
	t.Helper()
	gz, err := gzip.NewReader(bytes.NewReader(startupSQLGz))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	_, statements, err := utils.ExtractInsertStatements(gz)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for table, fields := range startupTranslatableFields {
		for _, statement := range statements[table] {
			rows, err := utils.ParseInsertRows(statement, startupTableColumns[table])
			if err != nil {
				t.Fatalf("parse %s: %v", table, err)
			}
			for _, row := range rows {
				id := row["id"]
				if table == "species" {
					id = row["ID"]
				}
				for _, field := range fields {
					if value := row[field]; value != "" && value != "NULL" {
						out[table+"\x00"+id+"\x00"+field] = value
					}
				}
			}
		}
	}
	return out
}

func TestTranslationArtifactFrenchValuesMatchDump(t *testing.T) {
	// The fr artifact is generated from the dump; every fr row value must
	// equal the dump's canonical column value for the same (table, id, field).
	want := startupTranslationKeys(t)
	values := startupTranslationDumpValues(t)

	data, err := translationSQLFS.ReadFile("translations_fr.sql")
	if err != nil {
		t.Fatal(err)
	}
	_, statements, err := utils.ExtractInsertStatements(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	columns := []string{"id", "table_name", "record_id", "field", "locale", "value", "created_at", "updated_at"}
	for _, statement := range statements["translations"] {
		rows, err := utils.ParseInsertRows(statement, columns)
		if err != nil {
			t.Fatalf("parse translations_fr.sql: %v", err)
		}
		for _, row := range rows {
			key := row["table_name"] + "\x00" + row["record_id"] + "\x00" + row["field"]
			if !want[key] {
				t.Errorf("fr artifact unexpected key %s", strings.ReplaceAll(key, "\x00", "|"))
				continue
			}
			if dump := values[key]; row["value"] != dump {
				t.Errorf("fr value drift for %s: artifact %q, dump %q",
					strings.ReplaceAll(key, "\x00", "|"), row["value"], dump)
			}
		}
	}
}

func TestCorrectedTranslationArtifactValues(t *testing.T) {
	// Regression pins: these rows shipped with wrong values (the "Autres
	// mammifères" translations carried Reptiles/Amphibians text, and the
	// "Reptiles, Amphibiens" row had split/incorrect locales). Any change
	// here must be deliberate.
	pinned := map[string]string{
		"en-US": "Other mammals (Procyonids, Viverrids, ...)",
		"de":    "Andere Säugetiere (Procyoniden, Schleichkatzen, ...)",
		"nl":    "Andere zoogdieren (Procyoniden, Civetkatten, ...)",
	}
	for locale, want := range pinned {
		data, err := translationSQLFS.ReadFile("translations_" + locale + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "'"+want+"'") {
			t.Errorf("translations_%s.sql lacks corrected value %q", locale, want)
		}
	}
	reptile := map[string]string{
		"en-US": "Reptiles, Amphibians",
		"de":    "Reptilien, Amphibien",
		"nl":    "Reptielen, Amfibieën",
	}
	for locale, want := range reptile {
		data, err := translationSQLFS.ReadFile("translations_" + locale + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "'"+want+"'") {
			t.Errorf("translations_%s.sql lacks corrected Reptiles value %q", locale, want)
		}
	}
}
