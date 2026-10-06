package actions

import (
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

// Round-4 R4-1 tests: every medication slot carries its urgency tier
// (colour), the late slot of a series is visually dominant, and the
// medication tier badge counts OCCURRENCES like the summary strip.

// TestSlotTierClass pins the R4-1.1 tier→colour mapping. Colour is
// additive: the glyph and the title stay the accessible label.
func TestSlotTierClass(t *testing.T) {
	require.Equal(t, "btn-danger", slotTierClass(0), "late/missing = red")
	require.Equal(t, "btn-warning", slotTierClass(1), "due now = yellow")
	require.Equal(t, "btn-light border", slotTierClass(2), "future = white, bordered")
	require.Equal(t, "btn-success", slotTierClass(3), "applied = green")
	require.Equal(t, "btn-light", slotTierClass(-1), "terminal states keep the neutral button")
}

// TestMedSlotTiering: a fixture series spanning yesterday (late), today
// (due) and tomorrow (scheduled) — the late slot is Tier 0/red, the
// future slot Tier 2/white, applied stays green and skipped/deferred stay
// untiered (R4-1.1).
func TestMedSlotTiering(t *testing.T) {
	now := time.Date(2026, 10, 2, 17, 39, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }

	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	late := testItem(med, 1, careplan.StatusLate)
	late.Occurrence.DueAt = at(1, 12)
	late.Applicable = false
	due := testItem(med, 1, careplan.StatusDue)
	due.Occurrence.DueAt = at(2, 12)
	future := testItem(med, 1, careplan.StatusScheduled)
	future.Occurrence.DueAt = at(3, 12)
	future.Applicable = false
	done := testItem(med, 1, careplan.StatusApplied)
	done.Occurrence.DueAt = at(2, 8)
	skipped := testItem(med, 1, careplan.StatusSkipped)
	skipped.Occurrence.DueAt = at(2, 9)
	plan.Items = []careplan.PlanItem{late, due, future, done, skipped}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	meds := v.buildMedGroups(plan, "", false)
	require.Len(t, meds, 1)
	byDay := map[string]MedSlotView{}
	for _, s := range meds[0].Slots {
		byDay[s.DueAt.Format("02-01")+" "+s.DueAtHM] = s
	}

	require.Equal(t, 0, byDay["01-10 12:00"].Tier, "yesterday's unapplied slot is the late tier")
	require.Equal(t, "btn-danger", byDay["01-10 12:00"].TierClass)
	require.Equal(t, 1, byDay["02-10 12:00"].Tier, "due now")
	require.Equal(t, "btn-warning", byDay["02-10 12:00"].TierClass)
	require.Equal(t, 2, byDay["03-10 12:00"].Tier, "tomorrow is the future tier")
	require.Equal(t, "btn-light border", byDay["03-10 12:00"].TierClass)
	require.Equal(t, 3, byDay["02-10 08:00"].Tier, "applied stays the done tier")
	require.Equal(t, "btn-success", byDay["02-10 08:00"].TierClass)
	require.Equal(t, -1, byDay["02-10 09:00"].Tier, "skipped is terminal, never re-coloured")
	require.Equal(t, "btn-light", byDay["02-10 09:00"].TierClass)
}

// TestMedSeriesLateSlotDominant (R4-1.2): in a late series the late slot
// is the Urgent one (red ring) and the future sibling is Future (recedes)
// — on EVERY surface (the shared partial), not only the care plan.
func TestMedSeriesLateSlotDominant(t *testing.T) {
	now := time.Date(2026, 10, 2, 17, 39, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	late := testItem(med, 1, careplan.StatusLate)
	late.Occurrence.DueAt = at(1, 12)
	late.Applicable = false
	future := testItem(med, 1, careplan.StatusScheduled)
	future.Occurrence.DueAt = at(3, 12)
	future.Applicable = false
	plan.Items = []careplan.PlanItem{late, future}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	meds := v.buildMedGroups(plan, "", false)
	require.Len(t, meds, 1)
	require.Len(t, meds[0].Series, 1)

	var urgent, fut []MedSlotView
	for _, row := range meds[0].Series[0].Rows {
		for _, s := range row.Slots {
			if s.Urgent {
				urgent = append(urgent, s)
			}
			if s.Future {
				fut = append(fut, s)
			}
		}
	}
	require.Len(t, urgent, 1, "exactly one urgent slot per series")
	require.Equal(t, 0, urgent[0].Tier, "the urgent slot is the late one")
	require.Equal(t, at(1, 12), urgent[0].DueAt)
	require.Len(t, fut, 1, "the future sibling recedes")
	require.Equal(t, at(3, 12), fut[0].DueAt)
}

// TestMedTierBadgeCountsOccurrences (R4-1.3): the medication section
// badge counts OCCURRENCES — the same unit as the summary strip — so the
// two numbers can never disagree (the "Late 22 vs Late 20" defect).
func TestMedTierBadgeCountsOccurrences(t *testing.T) {
	now := time.Date(2026, 10, 2, 17, 39, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	// one series, two LATE occurrences + one future
	l1 := testItem(med, 1, careplan.StatusLate)
	l1.Occurrence.DueAt = at(1, 8)
	l2 := testItem(med, 1, careplan.StatusLate)
	l2.Occurrence.DueAt = at(1, 20)
	f := testItem(med, 1, careplan.StatusScheduled)
	f.Occurrence.DueAt = at(3, 8)
	plan.Items = []careplan.PlanItem{l1, l2, f}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	require.Len(t, v.MedTiers[0], 1, "one series in the late tier")
	require.Equal(t, 2, v.MedTierOpen[0], "badge counts occurrences, not series")
	require.Equal(t, "2", v.MedTierOpenCap[0])
	require.Equal(t, 2, v.Stats.Late, "badge and strip agree (R4-1.3)")
	// A series sits in ONE section, but its future slot is still an
	// occurrence of the later tier: the badges count occurrences across all
	// series, so every badge equals the strip (the live "Late 22 vs Late
	// 20" defect).
	require.Empty(t, v.MedTiers[2], "the series sits in its most urgent tier only")
	require.Equal(t, 1, v.MedTierOpen[2], "its future slot counts in the later tier")
	require.Equal(t, v.Stats.Later, v.MedTierOpen[2], "later badge == later strip")
	require.Equal(t, v.Stats.Now, v.MedTierOpen[1], "now badge == now strip")
}

// TestMedSeriesPartialTierClasses (R4-1.1, all four locales): the shared
// partial paints each slot button with its tier class, marks the urgent
// slot and recedes the future ones.
func TestMedSeriesPartialTierClasses(t *testing.T) {
	now := time.Date(2026, 10, 2, 17, 39, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	late := testItem(med, 1, careplan.StatusLate)
	late.Occurrence.DueAt = at(1, 12)
	late.Applicable = false
	due := testItem(med, 1, careplan.StatusDue)
	due.Occurrence.DueAt = at(2, 12)
	fut := testItem(med, 1, careplan.StatusScheduled)
	fut.Occurrence.DueAt = at(3, 12)
	// Bug 2026-10-06 #4: a future slot inside the window IS applicable (the
	// §10-A1 window runs until the next occurrence of the source) — it
	// renders as the white ○ toggle. Non-applicable future slots cannot be
	// produced by genuine plan state; the 🔒 lock covers the hors-délai
	// leftovers (overridden / past the record-late bound).
	fut.Applicable = true
	done := testItem(med, 1, careplan.StatusApplied)
	done.Occurrence.DueAt = at(2, 8)
	plan.Items = []careplan.PlanItem{late, due, fut, done}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	meds := v.buildMedGroups(plan, "", false)
	require.Len(t, meds, 1)

	forks := []string{
		"../templates/care_plan/_med_series.plush.html",
		"../templates/care_plan/_med_series.plush.de.html",
		"../templates/care_plan/_med_series.plush.fr.html",
		"../templates/care_plan/_med_series.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		out, err := plush.Render(string(raw), plush.NewContextWith(map[string]interface{}{
			"mg":            meds[0],
			"t":             func(s string) string { return s },
			"partialFeeder": carePlanPartialFeeder(t),
		}))
		require.NoError(t, err, f)

		require.Contains(t, out, "btn-danger", f, "late slot is red")
		require.Contains(t, out, "btn-warning", f, "due-now slot is yellow")
		require.Contains(t, out, "btn-light border", f, "future slot is white")
		require.Contains(t, out, "btn-success", f, "applied slot is green")
		require.Contains(t, out, "plan-med-urgent", f, "the urgent slot is dominant")
		require.Contains(t, out, "plan-med-future", f, "the future slot recedes")
		require.Contains(t, out, "plan-med-btn", f, "fixed-width button (R4-3.2)")
		// Bug 2026-10-06 #4: every ACTIONABLE slot carries the same ○ to-do
		// glyph (the old `–` late glyph is gone); colour is never the only
		// signal — the late slot's title carries "record late".
		for _, glyph := range []string{"✓ 08:00", "○ 12:00"} {
			require.Contains(t, out, glyph, f)
		}
		require.NotContains(t, out, ">– ", f, "the dimmed late glyph is gone")
		require.False(t, strings.Contains(out, `class="btn  btn-sm`), f, "empty tier class")
	}
}

// TestMedSeriesLayoutAndInfoFirst (R4-2.3/R4-2.4/R4-3.1/R4-3.2, all four
// locales): the line follows the A/B/C layout rule — a fixed-width LEADING
// column holding the ℹ (first entry, aligned), a growing label block and
// ONE right-aligned button group; the morning/noon/evening grouping is an
// explicit LABELLED divider, not an empty rule.
func TestMedSeriesLayoutAndInfoFirst(t *testing.T) {
	slots := []MedSlotView{
		{Slot: "morning", Detail: "Citramox — 0.5 ml", DueAt: time.Date(2026, 10, 2, 8, 0, 0, 0, time.Local), DueAtHM: "08:00", Status: "due", Tier: 1, TierClass: "btn-warning", Applicable: true},
		{Slot: "morning", Detail: "Citramox — 0.5 ml", DueAt: time.Date(2026, 10, 2, 9, 0, 0, 0, time.Local), DueAtHM: "09:00", Status: "scheduled", Tier: 2, TierClass: "btn-light border"},
		{Slot: "evening", Detail: "Citramox — 0.5 ml", DueAt: time.Date(2026, 10, 2, 18, 0, 0, 0, time.Local), DueAtHM: "18:00", Status: "due", Tier: 1, TierClass: "btn-warning", Applicable: true},
	}
	mg := MedGroupView{AnimalID: 1, AnimalLabel: "472/26", Series: seriesOf(slots, seriesBucketOrder)}
	require.Len(t, mg.Series, 1)

	forks := []string{
		"../templates/care_plan/_med_series.plush.html",
		"../templates/care_plan/_med_series.plush.de.html",
		"../templates/care_plan/_med_series.plush.fr.html",
		"../templates/care_plan/_med_series.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		out, err := plush.Render(string(raw), plush.NewContextWith(map[string]interface{}{
			"mg":            mg,
			"t":             func(s string) string { return s },
			"partialFeeder": carePlanPartialFeeder(t),
		}))
		require.NoError(t, err, f)

		line := strings.Index(out, `class="d-flex align-items-start plan-med-line`)
		lead := strings.Index(out, `class="plan-med-lead text-nowrap"`)
		info := strings.Index(out, "plan-detail-btn")
		label := strings.Index(out, `class="plan-med-label"`)
		btns := strings.Index(out, `class="plan-med-btns"`)
		require.True(t, line >= 0 && lead > line && info > lead, f+": the line opens with the ℹ column")
		require.True(t, label > info, f+": the drug label follows the ℹ")
		require.True(t, btns > label, f+": ONE right-aligned button group closes the line")
		require.Contains(t, out, `class="plan-med-bucket">care_plan.slot.evening<`, f,
			"the morning/noon/evening grouping is labelled (R4-2.4)")
		require.Contains(t, out, `class="plan-med-cell"`, f, "the day badge travels with its own button (R4-3.2)")
		require.NotContains(t, out, "med-series-divider", f, "the unlabelled rule is gone")
	}
}

// TestMedSeriesNoRedundantLatePill (R4-3.3, all four locales): the series
// line drops the redundant "Late" text badge — the tier colour of the slot
// button now carries the status — while the terminal glyphs (skipped ⊘,
// deferred ⏸) survive, because a terminal state is not colour-encodable.
func TestMedSeriesNoRedundantLatePill(t *testing.T) {
	now := time.Date(2026, 10, 2, 17, 39, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	at := func(d, h int) time.Time { return time.Date(2026, 10, d, h, 0, 0, 0, time.Local) }
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	late := testItem(med, 1, careplan.StatusLate)
	late.Occurrence.DueAt = at(1, 12)
	late.Applicable = false
	skip := testItem(med, 2, careplan.StatusSkipped)
	skip.Occurrence.DueAt = at(2, 9)
	defer_ := testItem(med, 3, careplan.StatusDeferred)
	defer_.Occurrence.DueAt = at(2, 20)
	plan.Items = []careplan.PlanItem{late, skip, defer_}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, "", now)
	meds := v.buildMedGroups(plan, "", false)
	require.Len(t, meds, 3, "one group per animal fixture")

	// Render every group: the late slot, the skipped slot and the deferred
	// slot each live on their own line, so each fork is checked against all
	// three.
	outs := map[string]string{}
	forks := []string{
		"../templates/care_plan/_med_series.plush.html",
		"../templates/care_plan/_med_series.plush.de.html",
		"../templates/care_plan/_med_series.plush.fr.html",
		"../templates/care_plan/_med_series.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		for _, mg := range meds {
			out, err := plush.Render(string(raw), plush.NewContextWith(map[string]interface{}{
				"mg":            mg,
				"t":             func(s string) string { return s },
				"partialFeeder": carePlanPartialFeeder(t),
			}))
			require.NoError(t, err, f)
			outs[f] += out
		}

		out := outs[f]
		require.NotContains(t, out, "care_plan.status.late", f,
			"no redundant Late pill: the slot button colour carries the status")
		require.NotContains(t, out, `badge-warning`, f, "no status-coloured pill in the series line")
		require.Contains(t, out, "⊘ 09:00", f, "the skipped terminal glyph survives")
		require.Contains(t, out, "⏸ 20:00", f, "the deferred terminal glyph survives")
		require.Contains(t, out, "btn-danger", f, "the late slot is red")
	}
}
