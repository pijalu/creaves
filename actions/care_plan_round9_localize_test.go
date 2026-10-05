package actions

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// R9-6: care rules / matchers ship localized display names. The canonical
// French name stays the base column (fr); en-US/de/nl translations are
// resolved through the shared tname mechanism like every other reference
// record (zones, caretypes, …).

// TestSeedNameTranslationsComplete pins that every §7.4 seed matcher
// (SM1–SM14 + derived SR*) and every seed rule (SR1–SR13) carries a
// non-empty name in each editable locale (en-US, de, nl).
func TestSeedNameTranslationsComplete(t *testing.T) {
	matcherTr := SeedMatcherNameTranslations()
	for _, def := range SeedMatchers() {
		requireSeedNamesComplete(t, "matcher", def.Key, def.Name, matcherTr[def.Key])
	}

	ruleTr := SeedRuleNameTranslations()
	for _, def := range SeedRules() {
		requireSeedNamesComplete(t, "rule", def.Key, def.Name, ruleTr[def.Key])
	}
}

// requireSeedNamesComplete asserts one seed entry carries a non-empty name
// in each editable locale (en-US, de, nl).
func requireSeedNamesComplete(t *testing.T, kind, key, base string, names map[string]string) {
	t.Helper()
	if names == nil {
		t.Errorf("seed %s %s (%s): no translation entry", kind, key, base)
		return
	}
	for _, loc := range []string{"en-US", "de", "nl"} {
		if names[loc] == "" {
			t.Errorf("seed %s %s (%s): missing %s name", kind, key, base, loc)
		}
	}
}

// TestSeedNameTranslationsNoStrayKeys guards the reverse direction: no
// translation entry may reference a seed key that does not exist (a typo
// would silently never be applied).
func TestSeedNameTranslationsNoStrayKeys(t *testing.T) {
	matcherKeys := map[string]bool{}
	for _, def := range SeedMatchers() {
		matcherKeys[def.Key] = true
	}
	for k := range SeedMatcherNameTranslations() {
		if !matcherKeys[k] {
			t.Errorf("matcher translation key %q has no matching seed matcher", k)
		}
	}

	ruleKeys := map[string]bool{}
	for _, def := range SeedRules() {
		ruleKeys[def.Key] = true
	}
	for k := range SeedRuleNameTranslations() {
		if !ruleKeys[k] {
			t.Errorf("rule translation key %q has no matching seed rule", k)
		}
	}
}

// r96Context is a minimal buffalo.Context carrying only what tnameResolve
// needs: Request (language cookie), Set/Value (tx + per-request caches).
type r96Context struct {
	buffalo.Context
	req  *http.Request
	vals map[string]interface{}
}

func (c *r96Context) Request() *http.Request      { return c.req }
func (c *r96Context) Set(k string, v interface{}) { c.vals[k] = v }
func (c *r96Context) Value(key interface{}) interface{} {
	if key == "tx" {
		return c.vals["tx"]
	}
	return nil
}

// newR96TnameContext builds a context for one UI language with the shared
// test DB bound, mirroring the tnameMiddleware wiring.
func newR96TnameContext(t *testing.T, lang string) buffalo.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	c := &r96Context{req: req, vals: map[string]interface{}{}}
	c.Set("tx", searchTestDB(t))
	return c
}

// resolveViaTname invokes the real tname resolution path (bulk-load +
// id lookup + by-base fallback) for one record, exactly as a template would.
func resolveViaTname(c buffalo.Context, table, id, base string) string {
	lang := currentLang(c)
	field := "name"
	if f, ok := tnameDefaultField[table]; ok {
		field = f
	}
	var mu sync.Mutex
	cache := map[string]map[string]string{}
	return tnameResolve(c, lang, field, table, id, base, cache, &mu)
}

// TestTnameCareRuleMatcherLocalization seeds one care rule + one care
// matcher with translations in every locale, then asserts the tname helper
// resolves the localized name for en-US/de/nl and falls back to the
// canonical French base for fr.
func TestTnameCareRuleMatcherLocalization(t *testing.T) {
	db := searchTestDB(t)

	ruleID := uuid.Must(uuid.NewV4())
	matcherID := uuid.Must(uuid.NewV4())
	ruleBase := "R96 Règle Hérisson"
	matcherBase := "R96 Matcher Hérisson"

	ruleNames := map[string]string{"en-US": "R96 Rule Hedgehog", "de": "R96 Regel Igel", "nl": "R96 Regel Egel"}
	matcherNames := map[string]string{"en-US": "R96 Matcher Hedgehog", "de": "R96 Matcher Igel", "nl": "R96 Matcher Egel"}

	// Minimal rows: matcher (expression must parse) + rule referencing it.
	require.NoError(t, db.RawQuery(
		"INSERT INTO care_matchers (id, name, description, expression, created_at, updated_at) VALUES (?, ?, ?, ?, NOW(), NOW())",
		matcherID.String(), matcherBase, DefaultRuleDescription, `animal_age = "bébé"`).Exec())
	require.NoError(t, db.RawQuery(
		"INSERT INTO care_rules (id, name, description, action_kind, action_payload, schedule, matcher_id, active, priority, stop_on_outtake, latch_membership, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, 0, 0, 1, 0, NOW(), NOW())",
		ruleID.String(), ruleBase, DefaultRuleDescription, "cleanup", `{"note":"x"}`, `{"times":["09:00"],"every_days":1,"anchor":"intake"}`, matcherID.String()).Exec())

	for loc, v := range ruleNames {
		require.NoError(t, db.RawQuery(
			"INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES (?, 'care_rules', ?, 'name', ?, ?, NOW(), NOW())",
			uuid.Must(uuid.NewV4()).String(), ruleID.String(), loc, v).Exec())
	}
	for loc, v := range matcherNames {
		require.NoError(t, db.RawQuery(
			"INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES (?, 'care_matchers', ?, 'name', ?, ?, NOW(), NOW())",
			uuid.Must(uuid.NewV4()).String(), matcherID.String(), loc, v).Exec())
	}

	t.Cleanup(func() {
		db.RawQuery("DELETE FROM translations WHERE table_name IN ('care_rules','care_matchers') AND record_id IN (?, ?)", ruleID.String(), matcherID.String()).Exec()
		db.RawQuery("DELETE FROM care_rules WHERE id = ?", ruleID.String()).Exec()
		db.RawQuery("DELETE FROM care_matchers WHERE id = ?", matcherID.String()).Exec()
	})

	// fr (base) and the empty cookie both fall back to the canonical French.
	t.Run("fr-falls-back-to-base", func(t *testing.T) {
		c := newR96TnameContext(t, "fr")
		require.Equal(t, ruleBase, resolveViaTname(c, "care_rules", ruleID.String(), ruleBase))
		require.Equal(t, matcherBase, resolveViaTname(c, "care_matchers", matcherID.String(), matcherBase))
	})
	for _, loc := range []string{"en-US", "de", "nl"} {
		loc := loc
		t.Run("care_rules-"+loc, func(t *testing.T) {
			c := newR96TnameContext(t, loc)
			require.Equal(t, ruleNames[loc], resolveViaTname(c, "care_rules", ruleID.String(), ruleBase),
				fmt.Sprintf("care_rules name must localize to %s", loc))
		})
		t.Run("care_matchers-"+loc, func(t *testing.T) {
			c := newR96TnameContext(t, loc)
			require.Equal(t, matcherNames[loc], resolveViaTname(c, "care_matchers", matcherID.String(), matcherBase),
				fmt.Sprintf("care_matchers name must localize to %s", loc))
		})
	}
}

// TestSeedLibraryEmitsNameTranslations drives the real §7.4 seed step and
// asserts every freshly inserted matcher and rule carries a non-empty
// en-US/de/nl name translation keyed by its own record id. A second run is
// a strict no-op (deduped by name), so no translation is ever duplicated
// and administrator edits survive.
func TestSeedLibraryEmitsNameTranslations(t *testing.T) {
	db := searchTestDB(t)

	// Self-contained: drop every seed-library row (rule bindings cascade via
	// matcher_id/rule_id) and its translations so the insert path actually
	// fires. These are pure reference rows — the test recreates them, and the
	// cleanup purge leaves the empty seed library the converter round-trip
	// test expects.
	purgeCareSeeds := func() {
		db.RawQuery("DELETE FROM translations WHERE table_name = 'care_rules' AND record_id IN (SELECT id FROM care_rules)").Exec()
		db.RawQuery("DELETE FROM translations WHERE table_name = 'care_matchers' AND record_id IN (SELECT id FROM care_matchers)").Exec()
		db.RawQuery("DELETE FROM care_rule_exclusions").Exec()
		db.RawQuery("DELETE FROM care_rules").Exec()
		db.RawQuery("DELETE FROM care_matchers").Exec()
	}
	purgeCareSeeds()
	t.Cleanup(purgeCareSeeds)

	// Caretypes the feeding/care seeds resolve by canonical name.
	for _, name := range []string{"Repas", "Alimentation", "Soin"} {
		var ct models.Caretype
		if err := db.Where("name = ?", name).First(&ct); err == nil {
			continue
		}
		ct = models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: name}
		if name == "Repas" || name == "Alimentation" {
			ct.Type = models.CareTypeFeed
		}
		require.NoError(t, db.Create(&ct))
		t.Cleanup(func() {
			db.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec()
		})
	}

	report := &ConversionReport{}
	require.NoError(t, convertSeedLibrary(db, report))
	require.Equal(t, 18, report.Seeds.MatchersInserted, "14 canonical + 4 derived matchers insert")
	require.Equal(t, 13, report.Seeds.RulesInserted, "SR1–SR13 insert")

	countTr := func(table string, id string) int {
		var rows []struct {
			C int `db:"c"`
		}
		require.NoError(t, db.RawQuery(
			"SELECT count(*) AS c FROM translations WHERE table_name = ? AND record_id = ? AND field = 'name' AND locale IN ('en-US','de','nl') AND value <> ''",
			table, id).All(&rows))
		require.Len(t, rows, 1)
		return rows[0].C
	}

	// Every seeded matcher + rule must carry the three editable-locale names.
	require.Equal(t, 18, countSeededWithTranslations(t, db, countTr, "care_matchers"))
	require.Equal(t, 13, countSeededWithTranslations(t, db, countTr, "care_rules"))

	// Spot-check the actual localized values resolve through tname.
	oneMatcher := requireSeedByName(t, db, "Hérisson bébé")
	for loc, want := range SeedMatcherNameTranslations()["SM1"] {
		c := newR96TnameContext(t, loc)
		require.Equal(t, want, resolveViaTname(c, "care_matchers", oneMatcher.ID.String(), oneMatcher.Name))
	}
	var oneRule models.CareRule
	require.NoError(t, db.Where("name = ?", "Nettoyage des cages occupées").First(&oneRule))
	for loc, want := range SeedRuleNameTranslations()["SR13"] {
		c := newR96TnameContext(t, loc)
		require.Equal(t, want, resolveViaTname(c, "care_rules", oneRule.ID.String(), oneRule.Name))
	}

	// Second run: deduped by name, nothing re-inserted, no translation added.
	before := countTr("care_matchers", oneMatcher.ID.String())
	report2 := &ConversionReport{}
	require.NoError(t, convertSeedLibrary(db, report2))
	require.Equal(t, 0, report2.Seeds.MatchersInserted)
	require.Equal(t, 0, report2.Seeds.RulesInserted)
	require.Equal(t, before, countTr("care_matchers", oneMatcher.ID.String()))
}

// countSeededWithTranslations returns how many §7.4 seed rows of the given
// table carry all three editable-locale name translations, asserting each
// seed row individually along the way.
func countSeededWithTranslations(t *testing.T, db *pop.Connection, countTr func(string, string) int, table string) int {
	t.Helper()
	type idName struct {
		ID   string `db:"id"`
		Name string `db:"name"`
	}
	var rows []idName
	require.NoError(t, db.RawQuery("SELECT id, name FROM "+table).All(&rows))
	seedNames := map[string]bool{}
	if table == "care_matchers" {
		for _, def := range SeedMatchers() {
			seedNames[def.Name] = true
		}
	} else {
		for _, def := range SeedRules() {
			seedNames[def.Name] = true
		}
	}
	checked := 0
	for _, r := range rows {
		if !seedNames[r.Name] {
			continue
		}
		require.Equal(t, 3, countTr(table, r.ID), "%s %s must carry en-US/de/nl names", table, r.Name)
		checked++
	}
	return checked
}

// requireSeedByName fetches one seeded matcher by its canonical French name.
func requireSeedByName(t *testing.T, db *pop.Connection, name string) models.CareMatcher {
	t.Helper()
	var m models.CareMatcher
	require.NoError(t, db.Where("name = ?", name).First(&m))
	return m
}
