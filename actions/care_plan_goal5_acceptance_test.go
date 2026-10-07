//go:build !sqlite
// +build !sqlite

package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// BUGS 2026-10-27 — acceptance regression tests for the five user-reported
// /care_plan defects (creaves/bugs.md, "Care-plan deliverable review").
// Each test pins the CONFIRMED behavior contract from
// docs/care-plan-improvement-plan-2026-10-26.md. All template assertions
// run against every locale fork (en-US, fr, de, nl) — UI changes must land
// in all four variants.

var goal5Forks = []string{"", ".fr", ".de", ".nl"}

// ---------------------------------------------------------------------------
// Bug 1+2 (merged): /care_plan?kind=feeding&group=cage — the cage-group
// Apply button shows a bare clock icon; the contract (§3 cleanup parity)
// requires the due time ON the button ("○ <day> <HH:MM>", like
// plan-cage-apply). User: "the button shows a clock instead of a time".
// ---------------------------------------------------------------------------

func TestFeedingCageApplyButtonCarriesDueTime(t *testing.T) {
	for _, loc := range goal5Forks {
		raw := readTemplate(t, "../templates/care_plan/_plan_tier_feed_table.plush"+loc+".html")
		start := strings.Index(raw, "plan-feeding-apply")
		require.GreaterOrEqual(t, start, 0, loc+": cage-group Apply button must exist")
		end := strings.Index(raw[start:], "</button>")
		require.GreaterOrEqual(t, end, 0, loc+": malformed button")
		button := raw[start : start+end]

		require.NotContains(t, button, `class="far fa-clock"`, loc+
			": the group Apply button must not be a bare clock icon — it carries the due time")
		// Item 3 (2026-10-07): the day qualifier is a plan-med-day badge
		// BESIDE the button — short date beyond one day, else the localized
		// day key — while the button keeps the fixed "○ HH:MM" shape.
		require.Contains(t, button, "○ <%= fcard.FirstTimeLabel %>", loc+
			": due-time text must include the HH:MM label")
		require.Contains(t, raw, `plan-med-day"><%= fcard.FirstTimeShortDate %>`, loc+
			": the day badge must include the short-date variant")
		require.Contains(t, raw, `plan-med-day"><%= t(fcard.FirstTimeDayKey) %>`, loc+
			": the day badge must include the localized day key")
	}
}

// ---------------------------------------------------------------------------
// Bug 3b: /care_plan?kind=medication — applied (terminal) occurrences from
// BEFORE today still render as ✓ undo buttons inside the work-screen series
// block (reproduced live: 1905/26 Baycox "hier ✓ 12:00" on the production
// screen). todaysSlots keeps every slot due before end-of-today, and the
// preference caps only filter OPEN late/scheduled work — so a yesterday
// DONE slot survives forever. The work screen shows today; terminal slots
// from earlier days belong to the animal's Plan history.
// ---------------------------------------------------------------------------

func TestMedWorkScreenDropsBeforeTodayTerminalSlots(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	med := testSource(careplan.KindMedication, "med-X", "Baycox", map[string]interface{}{"drug": "Baycox", "dosage": "0.1 ml"})
	// Yesterday's dose: applied at 12:00 (terminal — the stale ✓ button).
	doneYesterday := testItem(med, 1, careplan.StatusApplied)
	doneYesterday.Occurrence.DueAt = time.Date(2026, 9, 27, 12, 0, 0, 0, time.Local)
	doneYesterday.Application = &careplan.ApplicationView{Status: "applied", AppliedAt: time.Date(2026, 9, 27, 12, 5, 0, 0, time.UTC)}
	// Today's dose: still open, due 12:00 (the real work).
	openToday := testItem(med, 1, careplan.StatusDue)
	openToday.Occurrence.DueAt = time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	// Today's already-applied morning dose: stays (today's record, R9-2).
	doneToday := testItem(med, 1, careplan.StatusApplied)
	doneToday.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	doneToday.Application = &careplan.ApplicationView{Status: "applied", AppliedAt: time.Date(2026, 9, 28, 8, 5, 0, 0, time.UTC)}

	plan.Items = []careplan.PlanItem{doneYesterday, openToday, doneToday}
	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)

	var beforeTodayTerminal []MedSlotView
	collect := func(s MedSlotView) {
		if s.Done && s.DueAt.Before(time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)) {
			beforeTodayTerminal = append(beforeTodayTerminal, s)
		}
	}
	for ti := range v.MedTiers {
		for _, line := range v.MedTiers[ti] {
			for _, series := range line.Series {
				for _, row := range series.Rows {
					for _, s := range row.Slots {
						collect(s)
					}
				}
			}
		}
	}
	for _, line := range v.MedDone {
		for _, series := range line.Series {
			for _, row := range series.Rows {
				for _, s := range row.Slots {
					collect(s)
				}
			}
		}
	}
	require.Empty(t, beforeTodayTerminal,
		"terminal (applied) occurrences from before today must not render on the work screen — they are the user's stale 'hier ✓' entries")
}

// ---------------------------------------------------------------------------
// Bug 3a: /care_plan?kind=medication&group=animal — the animal cells have
// per-row widths (measured live: 117px / 157px / 124px on one page). The
// confirmed contract (improvement plan §4, 2026-10-26): each table's animal
// column adapts to its OWN longest cell — the R4-7.15 computed page-wide
// width (min-width in ch from the longest label, never a hard width, never
// ellipsis) is reinstated. This intentionally supersedes the R9 removal of
// the shared width; care_plan_r415_animal_column_test.go is updated in the
// same change.
// ---------------------------------------------------------------------------

func TestMedAnimalColumnWidthIsWiredAgain(t *testing.T) {
	for _, loc := range goal5Forks {
		raw := readTemplate(t, "../templates/care_plan/_plan_med_row.plush"+loc+".html")
		require.Contains(t, raw, `style="min-width: <%= view.MedAnimalColCh %>ch"`, loc+
			": the animal cell must carry the computed shared column width")
		require.NotContains(t, raw, "text-overflow", loc+": never ellipsise")
		require.NotContains(t, raw, `overflow: hidden`, loc+": never clip")
	}
}

// ---------------------------------------------------------------------------
// Bug 4: /care_plan?kind=care&group=animal — care rows do not match the
// medication row visual style: the wrapper misses the med-row layout
// classes (d-flex / border-bottom / py-1) and the task text shares the
// animal text color instead of a distinct task styling (text-info, the
// diet-label treatment on the feeding page).
// ---------------------------------------------------------------------------

func TestCareLineMatchesMedicationRowStyle(t *testing.T) {
	for _, loc := range goal5Forks {
		raw := readTemplate(t, "../templates/care_plan/_plan_item_line.plush"+loc+".html")
		require.Contains(t, raw, `class="d-flex align-items-start flex-wrap border-bottom py-1 plan-med-line plan-item-line plan-med-row`, loc+
			": the item-line wrapper must carry the medication row layout classes")
		require.Contains(t, raw, `<strong class="text-info">`, loc+
			": the task text must use a distinct color (text-info), not the animal text color")
	}
}

// ---------------------------------------------------------------------------
// Bug 5: /care_plan?kind=cleanup&group=cage — a single-animal cage renders
// a bare "○" per-animal control (contract §3: single-animal cage shows an
// EMPTY cell; the cage Apply-all does the job), and an expanded
// multi-animal cage renders every animal inline on ONE wrapping flex line
// (contract §3: animal number + time-bearing control PER LINE).
// ---------------------------------------------------------------------------

func TestCleanupCageSingleSpacerAndMultiAnimalLines(t *testing.T) {
	for _, loc := range goal5Forks {
		raw := readTemplate(t, "../templates/care_plan/_plan_care_line.plush"+loc+".html")
		// Single-animal cage: no per-animal toggle — an invisible spacer
		// keeps the row footprint (the feeding table's plan-apply-space
		// precedent).
		require.Contains(t, raw, "plan-apply-space", loc+
			": a single-animal cage must render an empty slot cell, not a bare ○ button")
		// Multi-animal cage: one line per animal — the slots cell loops
		// per-animal lines, each carrying the animal number link.
		require.Contains(t, raw, "tg.AnimalLines", loc+
			": the time group must expose per-animal lines for multi-animal cages")
		require.Contains(t, raw, `class="plan-animal-row`, loc+
			": each animal gets its own line (feeding-table row classes)")
	}
}

// TestCleanupCageRowsBuildPerAnimalLines: the viewmodel groups each
// time-group's slots by animal (sorted by animal id) for cage rows with
// more than one animal, and leaves AnimalLines empty for single-animal
// cages (which render the spacer).
func TestCleanupCageRowsBuildPerAnimalLines(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	cleanup := testSource(careplan.KindCleanup, "clean-1", "Nettoyage des cages occupées", map[string]interface{}{"note": "cage + eau"})
	a := testItem(cleanup, 1, careplan.StatusDue)
	a.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	b := testItem(cleanup, 2, careplan.StatusDue)
	b.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{a, b}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindCleanup, "", now)
	require.NotEmpty(t, v.CareTiers[1], "due 09:00 cleanup is now-tier work")
	var multi *CareView
	for i := range v.CareTiers[1] {
		if v.CareTiers[1][i].AnimalCount > 1 {
			multi = &v.CareTiers[1][i]
			break
		}
	}
	require.NotNil(t, multi, "one cage row groups the two animals")
	require.Len(t, multi.TimeGroups, 1)
	lines := multi.TimeGroups[0].AnimalLines
	require.Len(t, lines, 2, "one line per animal")
	for _, l := range lines {
		require.NotEmpty(t, l.AnimalYear, "each line names its animal")
		require.NotEmpty(t, l.Slots, "each line carries its toggles")
	}
	// Ascending animal order for a stable, scannable list.
	require.Less(t, lines[0].AnimalID, lines[1].AnimalID)

	// Single-animal cage: no AnimalLines — the template renders the spacer.
	solo := testSource(careplan.KindCleanup, "clean-2", "Nettoyage des cages occupées", map[string]interface{}{"note": "cage + eau"})
	s1 := testItem(solo, 3, careplan.StatusDue)
	s1.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{s1}
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindCleanup, "", now)
	var single *CareView
	for i := range v.CareTiers[1] {
		if v.CareTiers[1][i].AnimalCount == 1 {
			single = &v.CareTiers[1][i]
			break
		}
	}
	require.NotNil(t, single, "the single-animal cage row exists")
	for _, tg := range single.TimeGroups {
		require.Empty(t, tg.AnimalLines, "single-animal cage: spacer, no per-animal lines")
	}
}
