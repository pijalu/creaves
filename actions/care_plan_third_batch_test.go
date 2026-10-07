package actions

// Bugs.md third user review batch (2026-10-06) — regression pins for the
// eight reported defects. Template pins run over ALL FOUR locale forks so a
// fix applied to one variant cannot silently drift from the others.

import (
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

func careForks(name string) []string {
	return []string{
		"../templates/care_plan/" + name + ".plush.html",
		"../templates/care_plan/" + name + ".plush.fr.html",
		"../templates/care_plan/" + name + ".plush.de.html",
		"../templates/care_plan/" + name + ".plush.nl.html",
	}
}

func animalShowForks() []string {
	return []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	}
}

// ---------------------------------------------------------------------------
// Item 2 — done feeding/cleanup actions move to History, never disappear.
// ---------------------------------------------------------------------------

func TestFeedingTerminalOccurrencesLandInHistory(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindFeeding, "src-feed", "Feed (conversion)", map[string]interface{}{"food": "Croquettes"})
	applied := testItem(src, 1, careplan.StatusApplied)
	superseded := testItem(src, 2, careplan.StatusLate)
	superseded.Applicable = false
	plan.Items = []careplan.PlanItem{applied, superseded}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, "", time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	require.Len(t, v.History, 2, "applied + superseded feeding occurrences show in history")
	statuses := map[string]bool{}
	for _, c := range v.History {
		statuses[c.Status] = true
	}
	require.True(t, statuses["applied"], "the applied feeding occurrence is in history")
	require.True(t, statuses["late"], "the superseded late feeding occurrence is in history")
}

// ---------------------------------------------------------------------------
// Item 6 — one merged animal cell per animal per medication tier.
// ---------------------------------------------------------------------------

func medSeriesOf(label string, dueAt time.Time, tierClass string) MedSeriesView {
	return MedSeriesView{
		Key: label, Label: label,
		Rows: []MedSeriesRow{{Slots: []MedSlotView{{
			Slot: "noon", Detail: label, DueAtHM: "12:00", DueAt: dueAt,
			Status: "due", Applicable: true, Tier: 1, TierClass: tierClass,
		}}}},
		FirstDueAt: dueAt,
	}
}

func TestMedLinesMergePerAnimalWithinTier(t *testing.T) {
	at := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	lines := []MedTierLine{
		{AnimalID: 7, AnimalLabel: "7/26 · Hérisson · A1", Series: []MedSeriesView{medSeriesOf("Baycox", at, "btn-warning")}},
		{AnimalID: 7, AnimalLabel: "7/26 · Hérisson · A1", Series: []MedSeriesView{medSeriesOf("Quadrosol", at.Add(-time.Hour), "btn-danger")}},
		{AnimalID: 9, AnimalLabel: "9/26 · Hérisson · A2", Series: []MedSeriesView{medSeriesOf("Catosal", at, "btn-warning")}},
	}
	merged := mergeMedLinesByAnimal(lines)
	require.Len(t, merged, 2, "one line per animal")
	// Animal 7 first: its line carries BOTH series, most-urgent first.
	require.Equal(t, 7, merged[0].AnimalID)
	require.Len(t, merged[0].Series, 2)
	require.Equal(t, "Quadrosol", merged[0].Series[0].Label, "series sort most-urgent-first")
	require.Equal(t, "Baycox", merged[0].Series[1].Label)
	require.Equal(t, 9, merged[1].AnimalID)
}

func TestMedTierViewHoldsOneLinePerAnimalPerTier(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	plan := testPlan()
	// TWO different sources (two drug series) of ONE animal, both due in the
	// same urgency window: before the merge each series rendered its own row
	// with its own animal cell — now they share one line.
	medA := testSource(careplan.KindMedication, "med-a", "Med A", map[string]interface{}{"drug": "Baycox", "dosage": "0.1 ml"})
	medB := testSource(careplan.KindMedication, "med-b", "Med B", map[string]interface{}{"drug": "Quadrosol", "dosage": "0.08 ml"})
	a := testItem(medA, 1, careplan.StatusDue)
	a.Occurrence.DueAt = now.Add(-time.Hour)
	b := testItem(medB, 1, careplan.StatusDue)
	b.Occurrence.DueAt = now.Add(-30 * time.Minute)
	plan.Items = []careplan.PlanItem{a, b}
	plan.Now = now

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	open := 0
	for i := 0; i < 3; i++ {
		for _, line := range v.MedTiers[i] {
			open++
			require.Equal(t, 1, line.AnimalID)
			require.LessOrEqual(t, len(line.Series), 2)
		}
	}
	require.Equal(t, 1, open, "both series of one animal render as ONE merged line")
	totalSeries := 0
	for i := 0; i < 3; i++ {
		for _, line := range v.MedTiers[i] {
			totalSeries += len(line.Series)
		}
	}
	require.Equal(t, 2, totalSeries, "the merged line carries both drug series")
}

// ---------------------------------------------------------------------------
// Item 7 — the care table shares the medication line treatment.
// ---------------------------------------------------------------------------

func TestCareViewSharesAnimalColumnWidth(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindCare, "src-care", "Soin", map[string]interface{}{"note": "bandage"})
	it := testItem(src, 1, careplan.StatusDue)
	plan.Items = []careplan.PlanItem{it}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindCare, "", time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	require.Positive(t, v.MedAnimalColCh, "the care view computes the shared animal column width")
}

func TestItemLineAnimalCellPrecedesInfoButton(t *testing.T) {
	for _, f := range careForks("_plan_item_line") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		animal := strings.Index(src, `class="plan-med-animal"`)
		lead := strings.Index(src, `class="plan-med-lead"`)
		require.GreaterOrEqual(t, animal, 0, f)
		require.GreaterOrEqual(t, lead, 0, f)
		require.Less(t, animal, lead, f+": the animal cell leads the line (medication row order)")
		require.Contains(t, src, `style="width: <%= view.MedAnimalColCh %>ch"`, f+": shared column width (exact — uniform column, no staircase)")
	}
}

// ---------------------------------------------------------------------------
// Item 4 — uniform to-do toggles, hors-délai/terminal markers, no oversized
// disabled buttons.
// ---------------------------------------------------------------------------

func TestSlotToggleUniformGlyphsAndMarkers(t *testing.T) {
	for _, f := range careForks("_plan_slot_toggle") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		require.NotContains(t, src, ">– ", f, "the old dimmed glyph is gone")
		require.Contains(t, src, "plan-med-state", f, "terminal/hors-délai markers")
		require.Contains(t, src, "plan-med-locked", f, "hors-délai lock marker")
		require.NotContains(t, src, `plan-med-slot plan-med-btn" disabled`, f, "no oversized disabled button")
		require.NotContains(t, src, `class="btn btn-light plan-med-slot`, f, "no hardcoded disabled state button")
	}
}

func TestItemLineNoDisabledButtonsInSlots(t *testing.T) {
	for _, f := range careForks("_plan_item_line") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		require.NotContains(t, src, `plan-item-slot-btn" disabled`, f, "hors-délai slots render the lock marker, not a dead button")
		require.NotContains(t, src, ">– ", f, "no dimmed glyph")
	}
}

// ---------------------------------------------------------------------------
// Item 5 — confirm ⇒ execute: the late flag travels end-to-end; the hors-
// délai 409 becomes unreachable from the UI; the server keeps rejecting
// future-due records.
// ---------------------------------------------------------------------------

func TestServerKeepsRejectingFutureLateRecord(t *testing.T) {
	src := testSource(careplan.KindFeeding, "src-fut", "Feed", nil)
	it := testItem(src, 1, careplan.StatusScheduled)
	it.Occurrence.DueAt = time.Now().Add(24 * time.Hour)
	it.Applicable = false
	item := it
	status, msg := checkPlanApplyItem(&item, true)
	require.Equal(t, 409, status, "late bypass is past-due only — future stays unrecordable")
	require.NotEmpty(t, msg)
}

func TestMedToggleSendsLateFlagAndRetries(t *testing.T) {
	for _, f := range careForks("_plan_med_toggle") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		require.Contains(t, src, "data-late') === 'true' || retriedLate", f, "applySlot sends late for late-recordable slots")
		require.Contains(t, src, "applySlot(btn, true)", f, "stale 409 retries once as a late record")
		require.Contains(t, src, "isPastDue", f, "the retry is bounded to past-due occurrences")
		require.Contains(t, src, "pending.late", f, "the dosage resubmit keeps the late semantics")
	}
}

func TestApplyToggleCarriesLateEndToEnd(t *testing.T) {
	for _, f := range careForks("_apply_toggle") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		require.Contains(t, src, `btn.getAttribute('data-late') === 'true'`, f, "instant apply forwards the late flag")
		require.Contains(t, src, "instantApply(btn, true)", f, "stale 409 retries once as a late record")
		require.Contains(t, src, `btn.getAttribute('data-late') === 'true'`, f)
		require.Contains(t, src, "applyRef.late = btn.classList.contains('plan-late-btn') || btn.getAttribute('data-late') === 'true'", f, "the apply modal submits late for data-late buttons")
		require.Contains(t, src, "submit(true)", f, "the modal path retries once as a late record")
	}
}

func TestIndexHistoryLateButtonPassesElement(t *testing.T) {
	for _, locale := range []string{"", ".fr", ".de", ".nl"} {
		index := readTemplate(t, "../templates/care_plan/index.plush"+locale+".html")
		require.Contains(t, index, "if (isInputKind(btn))", locale, "Record-late routes the ELEMENT (the string call threw a TypeError)")
		require.NotContains(t, index, "isInputKind(btn.getAttribute", locale)
		require.Contains(t, index, "function lateRetryOne(ref)", locale, "batch items retry once as a late record")
		require.Contains(t, index, "late: true", locale)
	}
}

// ---------------------------------------------------------------------------
// Item 3 — preferences: exactly ONE save for all modified values.
// ---------------------------------------------------------------------------

func TestPreferencesBulkSaveContract(t *testing.T) {
	raw, err := os.ReadFile("../templates/preferences/index.plush.html")
	require.NoError(t, err)
	src := string(raw)
	require.Contains(t, src, `id="preference-bulk"`, "the single bulk form")
	require.Contains(t, src, `action="/preferences/save"`, "posts to the bulk route")
	require.Equal(t, 1, strings.Count(src, `type="submit" form="preference-bulk"`), "exactly ONE save button")
	require.NotContains(t, src, `form id="pref-`, "no per-row forms")
	require.Contains(t, src, `name="late[<%= p.Kind %>]"`, "per-kind field names")
	require.Contains(t, src, `name="future[<%= p.Kind %>]"`)
	require.Contains(t, src, `name="now[<%= p.Kind %>]"`)

	appSrc := readTemplate(t, "app.go")
	require.Contains(t, appSrc, `app.POST("/preferences/save", PreferencesResource{}.SaveAll)`)
	require.NotContains(t, appSrc, "preferences/{preference_id}/save", "the per-row route is gone")
}

// ---------------------------------------------------------------------------
// Item 8 — the animal Protocol tab: Medication title + day-plan line band.
// ---------------------------------------------------------------------------

func TestAnimalProtocolTabMedicationSeparator(t *testing.T) {
	for _, f := range animalShowForks() {
		src := readTemplate(t, f)
		sep := strings.Index(src, `plan-kind-separator"><%= t("care_plan.kind.medication") %>`)
		require.GreaterOrEqual(t, sep, 0, f+": the medication block carries its own kind title")
		series := strings.Index(src, `care_plan/med_series.plush.html`)
		require.GreaterOrEqual(t, series, 0, f)
		require.Less(t, sep, series, f+": the separator announces the series block")
	}
}

func TestMedSeriesAnimalPageLineSharesItemLineBand(t *testing.T) {
	for _, f := range careForks("_med_series") {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		src := string(raw)
		require.Contains(t, src, "plan-med-row plan-item-line", f+": the animal-page med line uses the shared line band")
	}
}

// ---------------------------------------------------------------------------
// Item 1 — the feed table can never blow its column past the tier panel.
// ---------------------------------------------------------------------------

func TestFeedTableColumnNeverMaxContent(t *testing.T) {
	for _, locale := range []string{"", ".fr", ".de", ".nl"} {
		index := readTemplate(t, "../templates/care_plan/index.plush"+locale+".html")
		require.NotContains(t, index, "td.plan-med-animal { width: max-content", locale,
			"the rule that made Chrome blow the column up to the table's one-line width")
		require.Contains(t, index, ".plan-tier-body table { table-layout: auto; max-width: 100%; }", locale)
		require.Contains(t, index, ".plan-feed-animal .text-info, .plan-care-animal .text-info { white-space: normal; }", locale,
			"the diet text wraps instead of widening the column")
	}
}

// The item-line renders with the width without error (plush: view must be in
// context) — mirrors TestPlanRowLineLabelRenders but asserts the new markup.
func TestItemLineRendersMergedAnimalCell(t *testing.T) {
	raw := readTemplate(t, "../templates/care_plan/_plan_item_line.plush.html")
	card := CardView{
		SourceType: "animal", SourceID: "59da89ac",
		SourceName: "Traitement — Sexage", Detail: "Sexage",
		ActionKind: "observation", AnimalID: 9866,
		AnimalLabel: "1557/26 · Hérisson · R4", AnimalLink: "/animals/9866",
		SourceLink:  "/animals/9866",
		DueAt:       time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local),
		DueHM:       "12:00", Status: "due", Applicable: true,
	}
	ctx := plush.NewContextWith(map[string]interface{}{
		"card": card, "showAnimal": true, "view": DayPlanView{MedAnimalColCh: 12},
		"t":       func(s string, h plush.HelperContext) (string, error) { return s, nil },
		"tbase":   func(group, field, base string, h plush.HelperContext) (string, error) { return base, nil },
		"tspecies": func(base interface{}, h plush.HelperContext) (string, error) {
			if b, ok := base.(string); ok {
				return b, nil
			}
			return "", nil
		},
		"dueLabel": func(hm, dayKey, shortDate string, h plush.HelperContext) (string, error) { return hm, nil },
	})
	out, err := plush.Render(raw, ctx)
	require.NoError(t, err)
	animal := strings.Index(out, `class="plan-med-animal"`)
	lead := strings.Index(out, `class="plan-med-lead"`)
	require.GreaterOrEqual(t, animal, 0)
	require.Greater(t, lead, animal, "animal cell renders before the ℹ lead")
	require.Contains(t, out, "width: 12ch")
}
