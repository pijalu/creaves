package actions

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// traceSourceTestItem builds one plan item for the given source with the
// given override state — the raw input of traceSources.
func traceSourceTestItem(src *careplan.Source, animalID int, due time.Time, status careplan.PlanStatus, overriddenBy string) careplan.PlanItem {
	return careplan.PlanItem{
		Occurrence: careplan.Occurrence{
			Source:   src,
			AnimalID: animalID,
			DueAt:    due,
		},
		Status:       status,
		OverriddenBy: overriddenBy,
	}
}

// TestAnimalShowTraceMarksOverloadedRules (user ruling, full stack): a
// feeding rule whose every occurrence is suppressed by a replaces_kind
// specific protocol renders in the Details trace with the gray
// plan-trace-overridden row + the localized naming badge, while an
// unaffected care-kind rule stays plain.
func TestAnimalShowTraceMarksOverloadedRules(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	animalID := f.animalIDs[0]

	due := itemDueSoon(time.Now())
	// Rule A (feeding, no matcher → matches every animal): fully suppressed
	// by the specific protocol below (replaces_kind).
	ruleA := f.feedRule(t, models.DB, due, due)
	// Rule B (care kind): different kind — never interacts, stays active.
	careSched, err := json.Marshal(map[string]interface{}{
		"times":       []string{due.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
	})
	require.NoError(t, err)
	carePayload, err := json.Marshal(map[string]interface{}{"caretype_id": f.defCare.String()})
	require.NoError(t, err)
	ruleB := &models.CareRule{
		ID:            uuid.Must(uuid.NewV4()),
		Name:          "R-OVR-B-" + f.marker,
		ActionKind:    "care",
		ActionPayload: carePayload,
		Schedule:      careSched,
		Active:        true,
	}
	require.NoError(t, models.DB.Create(ruleB))

	// The specific protocol: same kind + slot, replaces_kind.
	planSched, err := json.Marshal(map[string]interface{}{
		"times":       []string{due.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
	})
	require.NoError(t, err)
	plan := &models.CareAnimalPlan{
		ID:            uuid.Must(uuid.NewV4()),
		AnimalID:      animalID,
		Name:          "P-OVR-" + f.marker,
		ActionKind:    "feeding",
		ActionPayload: json.RawMessage(`{"food":"crickets"}`),
		Schedule:      planSched,
		ReplacesKind:  true,
		Active:        true,
	}
	require.NoError(t, models.DB.Create(plan))

	html := fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d", animalID))

	require.Contains(t, html, "plan-trace-overridden",
		"the fully suppressed feeding rule row must carry the gray class")
	require.Equal(t, 1, strings.Count(html, "plan-trace-overridden"),
		"exactly one row (rule A) may be marked — rule B stays plain")
	require.Contains(t, html, "Overridden by a specific protocol",
		"the naming badge must render")
	require.Contains(t, html, "("+plan.Name+")",
		"the badge title names the suppressing specific protocol")
	require.Contains(t, html, ruleA.Name)
	require.Contains(t, html, plan.Name)
	require.Contains(t, html, ruleB.Name,
		"the unaffected rule still lists")
	// The popup guard + details-keep-open wiring ride on every show page.
	require.Contains(t, html, "js-plan-confirm-form")
	require.Contains(t, html, "planDetailsOpen")
	// Regression guard: the confirm message must render as a QUOTED JS string
	// (jsString). A bare <%= t(...) %> emits the translation unquoted and
	// HTML-escaped — a syntax error that silently kills the whole script
	// block (measured live 2026-10-14: handler never registered, form
	// submitted without any popup).
	require.Contains(t, html, `window.confirm("`,
		"confirm message must be a quoted JS literal (jsString), not bare output")
}

// TestTraceSourcesMarksFullyOverloadedRules (user ruling): a general rule
// whose EVERY window occurrence is suppressed by a specific protocol must
// surface Overridden=true with the suppressing protocol's name — the
// Details row grays out instead of reading as active. Partial overload
// (one occurrence still open) stays unmarked; animal plans are never
// marked (plans never suppress plans).
func TestTraceSourcesMarksFullyOverloadedRules(t *testing.T) {
	animal := &models.Animal{ID: 53}
	ruleA := careplan.NewSource(careplan.SourceRule, "11111111-1111-1111-1111-111111111111", "General A", careplan.KindCare, careplan.Schedule{})
	ruleB := careplan.NewSource(careplan.SourceRule, "22222222-2222-2222-2222-222222222222", "General B", careplan.KindCare, careplan.Schedule{})
	plan := careplan.NewSource(careplan.SourceAnimal, "33333333-3333-3333-3333-333333333333", "Specific X", careplan.KindCare, careplan.Schedule{})
	due := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

	planItems := []careplan.PlanItem{
		// ruleA: fully overloaded by Specific X (2/2 occurrences).
		traceSourceTestItem(ruleA, 53, due, careplan.StatusOverridden, "Specific X"),
		traceSourceTestItem(ruleA, 53, due.Add(time.Hour), careplan.StatusOverridden, "Specific X"),
		// ruleB: partially overloaded (1/2) — stays active-looking.
		traceSourceTestItem(ruleB, 53, due, careplan.StatusOverridden, "Specific X"),
		traceSourceTestItem(ruleB, 53, due.Add(2*time.Hour), careplan.StatusScheduled, ""),
		// The specific plan itself rides along unmarked.
		traceSourceTestItem(plan, 53, due, careplan.StatusScheduled, ""),
		// Another animal's occurrence must not leak in.
		traceSourceTestItem(ruleA, 99, due, careplan.StatusOverridden, "Specific X"),
	}

	trace := &ProtocolTraceView{}
	ruleIDs, planIDs := traceSources(&DayPlan{Items: planItems}, animal, false, trace)

	if len(ruleIDs) != 2 || len(planIDs) != 1 {
		t.Fatalf("ids: rules=%v plans=%v", ruleIDs, planIDs)
	}

	a := traceRowByID(t, trace, ruleA)
	assertOverloadState(t, a, true, "Specific X", 2, 2)
	b := traceRowByID(t, trace, ruleB)
	assertOverloadState(t, b, false, "", 1, 2)
	p := traceRowByID(t, trace, plan)
	assertOverloadState(t, p, false, "", 0, 1)
}

// traceRowByID fetches the trace row of one source, failing the test if the
// source produced no row at all.
func traceRowByID(t *testing.T, trace *ProtocolTraceView, src careplan.PlanSource) *ProtocolSourceView {
	t.Helper()
	for i := range trace.Sources {
		if trace.Sources[i].SourceID == src.SourceID() {
			return &trace.Sources[i]
		}
	}
	t.Fatalf("trace row for %q missing", src.Name())
	return nil
}

// assertOverloadState checks one row's overload marking: whether it is
// marked, the joined suppressor names, and both counters.
func assertOverloadState(t *testing.T, row *ProtocolSourceView, wantMarked bool,
	wantBy string, wantOverridden, wantOccurrences int) {
	t.Helper()
	if row.Overridden != wantMarked {
		t.Errorf("%s: Overridden=%v, want %v", row.Name, row.Overridden, wantMarked)
	}
	if row.OverriddenBy != wantBy {
		t.Errorf("%s: OverriddenBy=%q, want %q", row.Name, row.OverriddenBy, wantBy)
	}
	if row.OverriddenCount != wantOverridden || row.Occurrences != wantOccurrences {
		t.Errorf("%s: counts overridden=%d occurrences=%d, want %d/%d",
			row.Name, row.OverriddenCount, row.Occurrences, wantOverridden, wantOccurrences)
	}
}

// TestTraceSourcesOverriddenByJoinsDistinctNames: several specific
// protocols share the suppression — the badge names each one once.
func TestTraceSourcesOverriddenByJoinsDistinctNames(t *testing.T) {
	animal := &models.Animal{ID: 53}
	rule := careplan.NewSource(careplan.SourceRule, "44444444-4444-4444-4444-444444444444", "General C", careplan.KindCare, careplan.Schedule{})
	planX := careplan.NewSource(careplan.SourceAnimal, "55555555-5555-5555-5555-555555555555", "Specific X", careplan.KindCare, careplan.Schedule{})
	planY := careplan.NewSource(careplan.SourceAnimal, "66666666-6666-6666-6666-666666666666", "Specific Y", careplan.KindCare, careplan.Schedule{})
	due := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)

	planItems := []careplan.PlanItem{
		traceSourceTestItem(rule, 53, due, careplan.StatusOverridden, "Specific X"),
		traceSourceTestItem(rule, 53, due.Add(time.Hour), careplan.StatusOverridden, "Specific Y"),
		traceSourceTestItem(rule, 53, due.Add(2*time.Hour), careplan.StatusOverridden, "Specific X"),
		traceSourceTestItem(planX, 53, due, careplan.StatusScheduled, ""),
		traceSourceTestItem(planY, 53, due, careplan.StatusScheduled, ""),
	}

	trace := &ProtocolTraceView{}
	traceSources(&DayPlan{Items: planItems}, animal, false, trace)
	if len(trace.Sources) != 3 {
		t.Fatalf("sources=%d, want 3", len(trace.Sources))
	}
	got := trace.Sources[0].OverriddenBy
	if got != "Specific X, Specific Y" {
		t.Errorf("OverriddenBy=%q, want %q", got, "Specific X, Specific Y")
	}
	if !trace.Sources[0].Overridden {
		t.Error("Overridden=false, want true")
	}
}
