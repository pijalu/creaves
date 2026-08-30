package actions

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
)

// Fixture ids for the annual-report category localizer. All values prefixed
// RAL_ to stay isolated from seed data.
const (
	ralAgeID   = "00000000-0000-0000-0000-00000000a101"
	ralOTID    = "00000000-0000-0000-0000-00000000a102"
	ralCauseID = "00000000-0000-0000-0000-00000000a103"
)

func ralExec(t *testing.T, q string, args ...interface{}) {
	t.Helper()
	if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
		t.Fatalf("fixture query failed: %v\n%s", err, q)
	}
}

func setupRALFixtures(t *testing.T) {
	t.Helper()
	cleanupRAL(t)

	ralExec(t, "INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES (?, 'RAL_Age', 0, NOW(), NOW())", ralAgeID)
	ralExec(t, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('00000000-0000-0000-0000-00000000b101', 'animalages', ?, 'name', 'en-US', 'RAL_Age EN', NOW(), NOW())", ralAgeID)

	ralExec(t, "INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES (?, 'RAL_OT', 0, NOW(), NOW(), 0, 1, 0)", ralOTID)
	ralExec(t, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('00000000-0000-0000-0000-00000000b102', 'outtaketypes', ?, 'name', 'en-US', 'RAL_OT EN', NOW(), NOW())", ralOTID)

	ralExec(t, "INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES (?, 'RAL_Cause', '', 'RAL_Nature', 'x', NOW(), NOW(), 999)", ralCauseID)
	ralExec(t, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('00000000-0000-0000-0000-00000000b103', 'entry_causes', ?, 'cause', 'en-US', 'RAL_Cause EN', NOW(), NOW())", ralCauseID)
	ralExec(t, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('00000000-0000-0000-0000-00000000b104', 'entry_causes', ?, 'nature', 'en-US', 'RAL_Nature EN', NOW(), NOW())", ralCauseID)

	t.Cleanup(func() {
		cleanupRAL(t)
	})
}

func cleanupRAL(t *testing.T) {
	t.Helper()
	ids := []string{ralAgeID, ralOTID, ralCauseID}
	for _, id := range ids {
		for _, q := range []string{
			"DELETE FROM translations WHERE record_id = ?",
			"DELETE FROM animalages WHERE id = ?",
			"DELETE FROM outtaketypes WHERE id = ?",
			"DELETE FROM entry_causes WHERE id = ?",
		} {
			if err := models.DB.RawQuery(q, id).Exec(); err != nil {
				t.Fatalf("fixture cleanup failed: %v\n%s", err, q)
			}
		}
	}
}

func ralContext(t *testing.T) buffalo.Context {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "en-US"})
	c := newRALContext(req)
	c.Set("tx", models.DB)
	return c
}

func newRALContext(req *http.Request) buffalo.Context {
	vals := map[string]interface{}{}
	return &ralContext_{req: req, vals: vals}
}

// ralContext_ is a minimal buffalo.Context carrying just what the localizer
// needs: Request (language cookie), Set and Value (tx).
type ralContext_ struct {
	buffalo.Context
	req  *http.Request
	vals map[string]interface{}
}

func (c *ralContext_) Request() *http.Request      { return c.req }
func (c *ralContext_) Set(k string, v interface{}) { c.vals[k] = v }
func (c *ralContext_) Value(key interface{}) interface{} {
	if s, ok := key.(string); ok {
		return c.vals[s]
	}
	return nil
}

func TestLocalizeAnnualSections(t *testing.T) {
	setupRALFixtures(t)
	c := ralContext(t)

	sections := []annualStatSection{
		{ID: "entry_age", Rows: []annualStatRow{{Category: "RAL_Age"}}},
		{ID: "outtake_type", Rows: []annualStatRow{{Category: "RAL_OT"}}},
		{ID: "outtake_rating", Rows: []annualStatRow{{Category: "Dead"}, {Category: "Neutral"}, {Category: "Alive"}}},
		{ID: "outtake_dead_released", Rows: []annualStatRow{{Category: "Dead"}, {Category: "Released"}}},
		{ID: "entry_cause", Rows: []annualStatRow{{Category: "RAL_Cause"}}},
		{ID: "entry_cause_detail", Rows: []annualStatRow{{Category: "RAL_Nature / RAL_Cause"}}},
		{ID: "species", Rows: []annualStatRow{{Category: "Unknown"}}},
	}

	localizeAnnualSections(c, sections)

	assert := func(section, category, want string) {
		t.Helper()
		for _, sec := range sections {
			if sec.ID != section {
				continue
			}
			for _, row := range sec.Rows {
				if row.Category == want {
					return
				}
			}
		}
		t.Errorf("section %s: no row localized to %q (category %q missing)", section, want, category)
	}

	assert("entry_age", "RAL_Age", "RAL_Age EN")
	assert("outtake_type", "RAL_OT", "RAL_OT EN")
	assert("outtake_rating", "Dead", "reports.annual.rating.dead")
	assert("outtake_rating", "Neutral", "reports.annual.rating.neutral")
	assert("outtake_rating", "Alive", "reports.annual.rating.alive")
	assert("outtake_dead_released", "Released", "reports.annual.outtake.released")
	assert("entry_cause", "RAL_Cause", "RAL_Cause EN")
	assert("entry_cause_detail", "RAL_Nature / RAL_Cause", "RAL_Nature EN / RAL_Cause EN")
	// T is nil in unit tests; the literal falls back to its locale key.
	assert("species", "Unknown", "reports.annual.unknown")
}

func TestLocalizeAnnualSectionsBaseLangIsNoop(t *testing.T) {
	setupRALFixtures(t)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	c := newRALContext(req)
	c.Set("tx", models.DB)

	sections := []annualStatSection{
		{ID: "entry_age", Rows: []annualStatRow{{Category: "RAL_Age"}}},
	}
	localizeAnnualSections(c, sections)
	if sections[0].Rows[0].Category != "RAL_Age" {
		t.Errorf("base language must keep canonical values, got %q", sections[0].Rows[0].Category)
	}
}

func TestAnnualLocaleKeysPresent(t *testing.T) {
	keys := []string{
		"reports.annual.unknown",
		"reports.annual.rating.dead",
		"reports.annual.rating.neutral",
		"reports.annual.rating.alive",
		"reports.annual.outtake.released",
		"nav.reports_annual",
	}
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		b, err := os.ReadFile("../locales/reports." + lang + ".yaml")
		if err != nil {
			t.Fatalf("read locale %s: %v", lang, err)
		}
		for _, key := range keys {
			if !strings.Contains(string(b), `id: "`+key+`"`) {
				t.Errorf("locale reports.%s.yaml missing key %s", lang, key)
			}
		}
		if strings.Contains(string(b), "(new)") || strings.Contains(string(b), "nouveau") || strings.Contains(string(b), "neu)") || strings.Contains(string(b), "nieuw") {
			t.Errorf("locale reports.%s.yaml still marks statistics as new", lang)
		}
	}
}
