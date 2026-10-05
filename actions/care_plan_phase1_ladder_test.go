package actions

// Phase 1 / D1 — medication line responsive ladder + bucket divider
// (docs/defects/d1-medication-line-responsive-bucket-divider.md,
// docs/care-presentation-guideline.md §5/§6).
//
// Viewport behaviour (no horizontal overflow at 1600/1024/767 px, correct
// stacking level, click flips to green, undo restores the tier class) is
// verified in the browser via agent-browser probes and recorded on the D1
// defect entry; the tests here pin the CONTRACTS those probes depend on:
//
//   1-T1  ladder CSS: no global nowrap on medication rows/groups, no
//         ellipsis on medication labels, the dashboard override intact,
//         the level-3 media query and the JS-confirmed --stacked hook.
//   1-T2  level 1: the med row animal cell renders the year/number with
//         the full label in the title attribute.
//   1-T3  the viewmodel feeds AnimalYear (YearNumberFormatted) to the row.
//   1-T4  §6.2: the bucket caption gate stays server-side
//         (DividerBefore && !medSeriesCompact) and the bucket-fit JS is
//         wired on the shared toggle partial.
//   1-T5  markOpen restores the slot's ORIGINAL tier class from
//         data-tier-class — never a hardcoded btn-warning — and every live
//         med toggle carries data-tier-class.
//   1-T6  locale forks of every Phase-1-touched component are
//         byte-identical (project x4 rule).
//   1-T7  dashboard regression: the .dash-med-cell override block is
//         still scoped and unchanged in behaviour (wrap + grow + no clip).

import (
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

const carePlanSCSS = "../assets/css/care-plan.scss"

// readFile is a tiny helper so each pin names its file in the failure.
func readPhase1File(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, path)
	return string(raw)
}

// TestPhase1LadderCSS (1-T1): the R9 global single-line overrides that
// caused D1 are gone for medication rows and the ladder hooks exist.
// Phase 4 / D4 extended the SAME ladder to the row-kind lines
// (.plan-item-line) — their old scoped nowrap/ellipsis overrides are gone.
func TestPhase1LadderCSS(t *testing.T) {
	css := readPhase1File(t, carePlanSCSS)

	// The defect: global nowrap on the medication line and button group.
	require.NotContains(t, css, ".plan-med-row .plan-med-line {\n  flex-wrap: nowrap;",
		"D1: the medication line must wrap (ladder level 2)")
	// A bare global `.plan-med-btns { flex-wrap: nowrap }` is the exact
	// rule D1 calls out (scss:333); no variant may keep nowrap.
	require.NotRegexp(t, `(?m)^\.plan-med-btns \{\s*\n\s*flex-wrap: nowrap;`,
		"D1: no GLOBAL .plan-med-btns{flex-wrap:nowrap}")
	// Medication labels never ellipsize (R4-7.24).
	require.NotRegexp(t, `(?m)^\.plan-med-label \{\s*\n\s*white-space: nowrap;`,
		"D1: no ellipsis on the medication label")

	// Phase 4 / D4 (§5): the row-kind lines follow the SAME ladder — the
	// old scoped nowrap single-line override and the ellipsis label cut are
	// REMOVED, and the row-kind level-3 full-stack hooks exist.
	require.NotContains(t, css, ".plan-item-line.plan-med-line {\n  flex-wrap: nowrap;",
		"D4: the row-kind line wraps (ladder level 2)")
	require.NotRegexp(t, `(?m)\.plan-item-line \.plan-med-label \{\s*\n\s*white-space: nowrap;`,
		"D4: no ellipsis on the row-kind label (R4-7.24)")
	require.Contains(t, css, "@media (max-width: 767.98px)",
		"level 3: the narrow-viewport full-stack media query")
	require.Contains(t, css, ".plan-item-line .plan-med-animal {\n    flex: 1 1 100%;",
		"D4 level 3: the row-kind animal line stacks first")

	// Ladder level 2 hook: the JS-confirmed stacked state is a full-width
	// row; level 3: the medication full-stack media query.
	require.Contains(t, css, ".plan-med-btns.plan-med-btns--stacked",
		"level 2: JS-confirmed stacked groups go full-width")
	require.Contains(t, css, "div.plan-med-row:not(.plan-item-line) .plan-med-animal",
		"level 3: the medication animal line stacks first")
}

// TestPhase1MedRowAnimalYear (1-T2): level 1 — the care-plan medication
// row renders the year/number in the animal cell, the full label in the
// cell title (and it stays on the ℹ button's data-animal-label).
func TestPhase1MedRowAnimalYear(t *testing.T) {
	row := readPhase1File(t, "../templates/care_plan/_plan_med_row.plush.html")
	require.Contains(t, row, `title="<%= mg.AnimalLabel %>"`,
		"full label one hover away (guideline §5 level 1)")
	require.Contains(t, row, `<%= mg.AnimalYear %>`,
		"the cell renders the year/number only")
	require.NotContains(t, row, `<a href="<%= mg.AnimalLink %>"><%= mg.AnimalLabel %></a>`,
		"the long label no longer burns the line width")

	series := readPhase1File(t, "../templates/care_plan/_med_series.plush.html")
	require.Contains(t, series, `data-animal-label="<%= mg.AnimalLabel %>"`,
		"the full label also stays one click away in the ℹ detail modal")
}

// TestPhase1MedGroupAnimalYear (1-T3): buildMedGroups feeds AnimalYear
// from the model's YearNumberFormatted for the care-plan projection.
func TestPhase1MedGroupAnimalYear(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	due := testItem(med, 1, careplan.StatusDue)
	due.Occurrence.DueAt = time.Date(2026, 10, 2, 12, 30, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{due}

	v := BuildDayPlanView(plan, ViewDetailed, "", careplan.KindMedication, now)
	var mg *MedGroupView
	for i := range v.MedTiers {
		for j := range v.MedTiers[i] {
			mg = &v.MedTiers[i][j]
		}
	}
	require.NotNil(t, mg, "the fixture renders one medication group")
	require.Equal(t, "11/26", mg.AnimalYear,
		"year/number form (fixture animal 1 = 11/26), the dashboard parity form")
	require.NotEqual(t, mg.AnimalYear, mg.AnimalLabel,
		"the full label is richer than the year/number — level 1 is a real shrink")
	require.Contains(t, mg.AnimalLabel, "11/26",
		"the year/number is a prefix of the full label, so the title adds info")
}

// TestPhase1BucketCaptionGate (1-T4): §6.2 — the divider gate stays
// server-side (dashboard suppresses via medSeriesCompact), the captions
// keep their full-width band class, and the bucket-fit JS measures stacks.
func TestPhase1BucketCaptionGate(t *testing.T) {
	series := readPhase1File(t, "../templates/care_plan/_med_series.plush.html")
	require.Contains(t, series, "row.DividerBefore && !medSeriesCompact",
		"the dashboard (compact) still suppresses captions server-side")
	require.Contains(t, series, `class="plan-med-bucket"`,
		"the caption keeps its full-width band class")

	js := readPhase1File(t, "../templates/care_plan/_apply_toggle.plush.html")
	for _, needle := range []string{
		"function fitMedBuckets(",
		"plan-med-btns--stacked",
		".plan-med-bucket",
		"window.addEventListener('load', fitMedBuckets)",
		"window.addEventListener('resize', scheduleFitMedBuckets)",
	} {
		require.Contains(t, js, needle, "bucket-fit JS wiring missing: "+needle)
	}
}

// TestPhase1UndoRestoresTierClass (1-T5): markOpen → setToggleState(false)
// restores the slot's ORIGINAL tier class from data-tier-class (§2.4) —
// never a hardcoded btn-warning — and every LIVE med toggle variant
// (applicable / late-recordable / applied-undoable) carries the attribute.
func TestPhase1UndoRestoresTierClass(t *testing.T) {
	js := readPhase1File(t, "../templates/care_plan/_apply_toggle.plush.html")
	require.Contains(t, js, "var tier = btn.getAttribute('data-tier-class')",
		"undo reads the original tier class off the button")
	// The fallback default may exist, but it must never be the only source:
	// the applied flip strips every tier class and the open flip re-adds
	// the button's own.
	require.Contains(t, js, "btn.classList.remove('btn-warning', 'btn-danger', 'btn-light', 'border',",
		"the applied flip strips the tier classes before going green")
	require.Contains(t, js, "tier.split(/\\s+/).forEach(function (c) { if (c) { btn.classList.add(c); } });",
		"the open flip restores exactly the classes the server rendered")

	toggle := readPhase1File(t, "../templates/care_plan/_plan_slot_toggle.plush.html")
	live := 0
	for _, line := range strings.Split(toggle, "\n") {
		if strings.Contains(line, "plan-med-apply") || strings.Contains(line, "plan-med-unapply") {
			live++
		}
	}
	require.GreaterOrEqual(t, live, 3, "applicable, late and applied variants all render")
	require.GreaterOrEqual(t, strings.Count(toggle, `data-tier-class="<%= slot.TierClass %>"`), 3,
		"every live variant carries its original tier class")
}

// TestPhase1ForksByteIdentical (1-T6): the x4-locale project rule for
// every template Phase 1 touched.
func TestPhase1ForksByteIdentical(t *testing.T) {
	for _, c := range []string{"_plan_med_row", "_med_series", "_apply_toggle"} {
		base := "../templates/care_plan/" + c + ".plush.html"
		raw := readPhase1File(t, base)
		for _, loc := range []string{"fr", "de", "nl"} {
			fork := "../templates/care_plan/" + c + ".plush." + loc + ".html"
			require.Equal(t, raw, readPhase1File(t, fork), fork+" drifted from "+base)
		}
	}
}

// TestPhase1DashboardOverrideIntact (1-T7): the R9-3 dashboard medication
// cell keeps its scoped wrap-and-grow override — the ladder rework must
// not regress the surface that already stacked correctly.
func TestPhase1DashboardOverrideIntact(t *testing.T) {
	css := readPhase1File(t, carePlanSCSS)
	for _, needle := range []string{
		".dash-med-cell .plan-med-line {\n  flex-wrap: wrap;",
		".dash-med-cell .plan-med-label {",
		".dash-med-cell .plan-med-btns {",
		".dash-med-cell .plan-med-cell {",
		"text-overflow: clip;",
	} {
		require.Contains(t, css, needle, "dashboard override missing: "+needle)
	}
	// The override stays SCOPED: no dash-med-cell rule may leak onto the
	// bare medication classes.
	require.NotContains(t, css, ".dash-med-cell .plan-med-row",
		"the dashboard override never touched the row shell")
}
