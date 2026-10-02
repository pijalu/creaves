//go:build !sqlite

package actions

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// bugs.md TEST-4: a care rule whose `description` column is NULL used to make
// EVERY scan of care_rules fail ("Scan error on column index 4, name
// description: converting NULL to string is unsupported"), which turned the
// whole day plan into a 500 for every animal. The Go field was a plain
// string while the column is nullable (`t.Column("description", "text",
// {null: true})`), so any row inserted without a description — a seeder, an
// import, raw SQL — took /care_plan down.
//
// These tests pin the read model against rows no fixture can produce: the
// model constructors always write a non-NULL description.

// nullDescriptionRule inserts a rule BYPASSING the model, exactly like an
// import or a raw INSERT would, so the description column really is NULL.
func nullDescriptionRule(t *testing.T, f *planFixture, kind string) {
	t.Helper()
	var payload json.RawMessage
	var sched json.RawMessage
	var err error
	due := itemDueSoon(time.Now())
	switch kind {
	case "feeding":
		payload, err = json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String(), "food": "grenouilles"})
		require.NoError(t, err)
		sched, err = json.Marshal(map[string]interface{}{
			"times": []string{due.Format("15:04")}, "anchor": "fixed", "anchor_date": due.Format("2006-01-02"),
		})
		require.NoError(t, err)
	default:
		t.Fatalf("unsupported kind %q", kind)
	}
	id := uuid.Must(uuid.NewV4())
	require.NoError(t, models.DB.RawQuery(
		"INSERT INTO care_rules (id, name, description, action_kind, action_payload, schedule, active, priority, stop_on_outtake, latch_membership, created_at, updated_at)"+
			" VALUES (?, ?, NULL, ?, ?, ?, 1, 0, 1, 0, NOW(), NOW())",
		id.String(), "NULLDESC-"+f.marker, kind, string(payload), string(sched)).Exec())
	t.Cleanup(func() { models.DB.RawQuery("DELETE FROM care_rules WHERE id = ?", id).Exec() })
}

// TestDayPlanSurvivesARuleWithNullDescription is the user-visible half: the
// page must render, not crash.
func TestDayPlanSurvivesARuleWithNullDescription(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	nullDescriptionRule(t, f, "feeding")

	code, body := planGetJSON(t, client, baseURL, "/care_plan")
	require.Equal(t, http.StatusOK, code, "a NULL description must not 500 the day plan: %s", body)
	require.NotEmpty(t, planItemsOf(t, body))
}

// TestCareRuleScannerAcceptsNullDescription pins the model itself: the
// nullable column must survive a round-trip through pop.
func TestCareRuleScannerAcceptsNullDescription(t *testing.T) {
	requireMySQLTestDB(t)
	id := uuid.Must(uuid.NewV4())
	require.NoError(t, models.DB.RawQuery(
		"INSERT INTO care_rules (id, name, description, action_kind, action_payload, schedule, active, priority, stop_on_outtake, latch_membership, created_at, updated_at)"+
			" VALUES (?, ?, NULL, 'care', '{}', ?, 1, 0, 1, 0, NOW(), NOW())",
		id.String(), "NULLDESC-SCAN", `{"times":["10:00"],"anchor":"fixed","anchor_date":"2026-01-01"}`).Exec())
	t.Cleanup(func() { models.DB.RawQuery("DELETE FROM care_rules WHERE id = ?", id).Exec() })

	var got models.CareRule
	require.NoError(t, models.DB.Find(&got, id), "a NULL description must scan")
	require.False(t, got.Description.Valid)
	require.Equal(t, "", got.Description.String)
}

// TestCareMatcherAndExclusionScanNullDescription covers the two sibling
// models with the same nullable-column mismatch, so the fix cannot be undone
// for one table and left broken for the others.
func TestCareMatcherAndExclusionScanNullDescription(t *testing.T) {
	requireMySQLTestDB(t)
	mid := uuid.Must(uuid.NewV4())
	expr := `species = "CP-A1"`
	require.NoError(t, models.DB.RawQuery(
		"INSERT INTO care_matchers (id, name, description, expression, created_at, updated_at) VALUES (?, ?, NULL, ?, NOW(), NOW())",
		mid.String(), "NULLDESC-MATCHER", expr).Exec())
	var matcher models.CareMatcher
	require.NoError(t, models.DB.Find(&matcher, mid), "a NULL matcher description must scan")
	require.False(t, matcher.Description.Valid)
	models.DB.RawQuery("DELETE FROM care_matchers WHERE id = ?", mid).Exec()

	rid := uuid.Must(uuid.NewV4())
	f := setupPlanFixture(t)
	rule := &models.CareRule{
		ID: rid, Name: "NULLDESC-EXCL-" + f.marker, ActionKind: "care",
		ActionPayload: json.RawMessage(`{}`),
		Schedule:      json.RawMessage(`{"times":["10:00"],"anchor":"fixed","anchor_date":"2026-01-01"}`),
		Active:        true,
	}
	require.NoError(t, models.DB.Create(rule))
	require.NoError(t, models.DB.RawQuery(
		"INSERT INTO care_rule_exclusions (id, rule_id, animal_id, reason, created_at, updated_at) VALUES (?, ?, ?, NULL, NOW(), NOW())",
		uuid.Must(uuid.NewV4()).String(), rid.String(), f.animalIDs[0]).Exec())

	var exclusions []models.CareRuleExclusion
	require.NoError(t, models.DB.Where("rule_id = ?", rid).All(&exclusions), "a NULL exclusion reason must scan")
	require.Len(t, exclusions, 1)
	require.False(t, exclusions[0].Reason.Valid)
	require.False(t, exclusions[0].CreatedBy.Valid, "created_by is nullable too")
}
