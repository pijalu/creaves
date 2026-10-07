package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// R4-7.7 (bugs.md): /care_plan?view=compact&kind=medication used to show
// EVERY slot of the schedule — including tomorrow's — so the "work" screen
// was never empty and tomorrow's buttons mixed with today's. The screen now
// scopes to the DAY: everything due up to the end of today (open or
// terminal), plus at most ONE later open slot, the nearest, and only when it
// is nearer than the pending late entry. The surviving later slot is
// bucketed "tomorrow" so it reads as its own group.

func r47Slot(day time.Time, h int, status careplan.PlanStatus) MedSlotView {
	return MedSlotView{DueAt: day.Add(time.Duration(h) * time.Hour), Status: string(status)}
}

// r47LateSlot is an ACTIONABLE late slot (Applicable, like medSlotFor
// projects an in-window late occurrence) — the shape the item-8 fold
// consumes.
func r47LateSlot(day time.Time, h int, status careplan.PlanStatus) MedSlotView {
	return MedSlotView{DueAt: day.Add(time.Duration(h) * time.Hour), Status: string(status), Applicable: true}
}

func r47Series(label string, slots ...MedSlotView) MedSeriesView {
	return MedSeriesView{Label: label, Rows: []MedSeriesRow{{Slots: slots}}}
}

func r47SlotTimes(series MedSeriesView) []time.Time {
	var out []time.Time
	for _, row := range series.Rows {
		for _, s := range row.Slots {
			out = append(out, s.DueAt)
		}
	}
	return out
}

// Today's slots stay. Item 8 (2026-10-07): the nearest later slot is now
// ALWAYS kept while a late entry is pending (co-display: "show in late the
// next upcoming action"), so tomorrow's slot survives beside the late one.
func TestScopeSeriesToTodayKeepsTomorrowBesideLate(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(16 * time.Hour) // 16:00

	series := r47Series("Amox 1 ml",
		r47Slot(day, 8, careplan.StatusApplied),                    // done this morning
		r47LateSlot(day, 12, careplan.StatusLate),                  // 4 h overdue
		r47Slot(day.AddDate(0, 0, 1), 8, careplan.StatusScheduled), // tomorrow 08:00: 16 h away
	)
	series.Rows[0].Slots[0].Done = true

	got := scopeSeriesToToday(series, 1, now)

	// item 8 co-display: the next upcoming slot (tomorrow 08:00) shows
	// beside the pending late entry; the late one is NOT stale (4 h old vs
	// 16 h to the next).
	require.Equal(t, []time.Time{day.Add(8 * time.Hour), day.Add(12 * time.Hour), day.AddDate(0, 0, 1).Add(8 * time.Hour)}, r47SlotTimes(got),
		"today's slots (done + late) plus the next upcoming slot remain")
	require.Equal(t, 0, got.StaleLateCount, "a fresh late entry keeps its toggle")
	// the co-displayed slot is its own "tomorrow" bucket row
	require.Len(t, got.Rows, 2)
	require.Equal(t, slotTomorrow, got.Rows[1].Slots[0].Slot)
	require.Equal(t, "Amox 1 ml", got.Label)
}

// Item 8's relative-distance rule: a late entry whose age exceeds the
// distance to the next occurrence is STALE — it folds into the series'
// "⏱ N" badge and the next upcoming slot carries the line. A fresher late
// entry keeps its toggle beside the next upcoming slot.
func TestScopeSeriesToTodayStaleLateFolds(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00
	tom := day.AddDate(0, 0, 1)

	tests := []struct {
		name        string
		overdue     time.Duration // age of the pending late entry
		staleCount  int
		keptSlotDue time.Time // the single actionable slot left in the rows
	}{
		{"a 20 h overdue entry folds (next is 10 h away)", 20 * time.Hour, 1, tom.Add(8 * time.Hour)},
		{"a 1 h overdue entry keeps its toggle (next is 10 h away)", 1 * time.Hour, 0, now.Add(-1 * time.Hour)},
		{"exactly at the midpoint keeps the toggle (strict rule)", 10 * time.Hour, 0, now.Add(-10 * time.Hour)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			series := r47Series("Amox",
				MedSlotView{DueAt: now.Add(-tc.overdue), Status: string(careplan.StatusLate), Applicable: true},
				MedSlotView{DueAt: tom.Add(8 * time.Hour), Status: string(careplan.StatusScheduled)}, // 10 h away
			)

			got := scopeSeriesToToday(series, 1, now)

			require.Equal(t, tc.staleCount, got.StaleLateCount, "stale fold count")
			times := r47SlotTimes(got)
			require.Len(t, times, 2-tc.staleCount, "a folded entry leaves the rows, a kept one stays")
			require.Equal(t, tc.keptSlotDue, times[0])
			if tc.staleCount > 0 {
				// the next upcoming slot is co-displayed, bucketed "tomorrow"
				require.Equal(t, slotTomorrow, got.Rows[len(got.Rows)-1].Slots[0].Slot)
				require.Contains(t, got.StaleRefsJSON, "due_at", "the fold carries its snooze ref")
			}
		})
	}
}

// R9-2 supersedes R4-7.7's "empty in the evening": a fully-applied series
// no longer disappears — it keeps its today's done record so the new Done
// tier can render it. Only a series with NOTHING due today leaves.
func TestScopeSeriesToTodayEmptyInEvening(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00

	series := r47Series("Amox",
		r47Slot(day, 8, careplan.StatusApplied),
		r47Slot(day, 20, careplan.StatusApplied),
		r47Slot(day.AddDate(0, 0, 1), 8, careplan.StatusScheduled),
	)
	series.Rows[0].Slots[0].Done = true
	series.Rows[0].Slots[1].Done = true

	got := scopeSeriesToToday(series, 1, now)

	// R9-2: the day's two applied occurrences stay visible (the Done tier
	// renders them); tomorrow's scheduled slot is dropped.
	times := r47SlotTimes(got)
	require.Equal(t, []time.Time{day.Add(8 * time.Hour), day.Add(20 * time.Hour)}, times,
		"today's done record survives for the Done tier; tomorrow leaves")
	require.Equal(t, "Amox", got.Label)
}

// At most ONE later slot survives, not the whole rest of the schedule.
// Item 8: a 20 h overdue entry with the next occurrence 8 h out is stale —
// it folds into the badge and the nearest later slot carries the line.
func TestScopeSeriesToTodayKeepsAtMostOneLaterSlot(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00
	series := r47Series("Amox",
		MedSlotView{DueAt: day.Add(2 * time.Hour), Status: string(careplan.StatusLate), Applicable: true}, // 20 h overdue
		r47Slot(day.AddDate(0, 0, 1), 6, careplan.StatusScheduled),
		r47Slot(day.AddDate(0, 0, 1), 8, careplan.StatusScheduled),
		r47Slot(day.AddDate(0, 0, 2), 8, careplan.StatusScheduled),
	)

	got := scopeSeriesToToday(series, 1, now)

	times := r47SlotTimes(got)
	require.Len(t, times, 1, "the stale late folds; the single nearest later slot carries the line")
	require.Equal(t, day.AddDate(0, 0, 1).Add(6*time.Hour), times[0],
		"the NEAREST later slot wins, not the first in the list")
	require.Equal(t, 1, got.StaleLateCount)
}

// fillMedTiers drops a series that scoping emptied — the caller chain must
// not render an empty medication line. R9-2: a fully-applied series is NOT
// dropped anymore — it moves to MedDone; only a series with nothing due
// today leaves the screen. Here "All done" was applied today, so it lands
// in MedDone and stays OUT of the open tiers.
func TestFillMedTiersDropsScopedOutSeries(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := []MedGroupView{{
		AnimalID: 1, AnimalLabel: "Fox-1", AnimalLink: "/animals/1",
		Series: []MedSeriesView{
			{Key: "a", Label: "All done", Rows: []MedSeriesRow{{Slots: []MedSlotView{
				{DueAt: day.Add(8 * time.Hour), Status: string(careplan.StatusApplied), Done: true, Applied: true},
			}}}},
			{Key: "b", Label: "Pending", Rows: []MedSeriesRow{{Slots: []MedSlotView{
				{DueAt: day.Add(20 * time.Hour), Status: string(careplan.StatusLate)},
			}}}},
		},
	}}

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(22*time.Hour)) // 22:00 — the day's work is done

	for _, tier := range v.MedTiers {
		for _, line := range tier {
			require.NotEqual(t, "All done", line.Series[0].Label,
				"an all-done series renders no line in the OPEN tiers")
		}
	}
	require.Len(t, v.MedTiers[0], 1)
	require.Equal(t, "Pending", v.MedTiers[0][0].Series[0].Label)
	// R9-2: the fully-applied series is in the Done section, not dropped.
	require.Len(t, v.MedDone, 1)
	require.Equal(t, "All done", v.MedDone[0].Series[0].Label)
}

// The "tomorrow" bucket label must exist in EVERY locale — a missing key
// renders the raw key to the caregiver.
func TestTomorrowSlotLabelInAllLocales(t *testing.T) {
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		require.Contains(t, raw, `- id: "care_plan.slot.tomorrow"`, lang)
	}
	// and it must not collide with an empty translation
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		i := indexOf(raw, `- id: "care_plan.slot.tomorrow"`)
		require.GreaterOrEqual(t, i, 0, lang)
		rest := raw[i:]
		j := indexOf(rest, "\n- id:")
		require.Greater(t, j, 0, lang)
		require.Contains(t, rest[:j], "translation:", lang+" — tomorrow has no translation")
	}
}

func indexOf(s, sub string) int {
	return strings.Index(s, sub)
}

// R9 next-in-future rule (user review) + item 8 (2026-10-07): past 12:00
// the day plan showed the same treatment TWICE — yesterday's slot as LATE
// and today's as due now. Once the next occurrence is due (next_in = 0)
// every pending late folds into the series' "⏱ N" badge — the due slot
// carries the line, and the badge keeps the missed occurrence visible with
// its snooze ref. Done slots (the day's record) always survive.
func TestScopeSeriesDropsPastWhenDueNow(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	yest := day.AddDate(0, 0, -1)
	now := day.Add(13 * time.Hour) // 13:00 — past the 12:00 slot

	series := r47Series("Citramox 0.02 ml",
		r47LateSlot(yest, 12, careplan.StatusLate), // yesterday 12:00, missed
		r47Slot(day, 12, careplan.StatusDue),   // today 12:00, due NOW
		r47Slot(day.AddDate(0, 0, 1), 12, careplan.StatusScheduled),
	)

	got := scopeSeriesToToday(series, 7, now)
	times := r47SlotTimes(got)
	require.NotContains(t, times, yest.Add(12*time.Hour),
		"the stale past slot must fold once today's slot is due")
	require.Contains(t, times, day.Add(12*time.Hour),
		"the due-now slot stays")
	require.Equal(t, 1, got.StaleLateCount, "the missed occurrence is folded, not deleted")
	require.Contains(t, got.StaleRefsJSON, `"animal_id":7`, "the fold carries its snooze ref")

	// The done record of today survives the fold (applied 08:00).
	withDone := r47Series("Amox 1 ml",
		r47LateSlot(yest, 12, careplan.StatusLate),
		r47Slot(day, 8, careplan.StatusApplied),
		r47Slot(day, 12, careplan.StatusDue),
	)
	withDone.Rows[0].Slots[1].Done = true
	got2 := scopeSeriesToToday(withDone, 7, now)
	times2 := r47SlotTimes(got2)
	require.NotContains(t, times2, yest.Add(12*time.Hour))
	require.Contains(t, times2, day.Add(8*time.Hour), "the done slot is the record")
	require.Equal(t, 1, got2.StaleLateCount)
}

// The counterpart: with NO slot due now and the next occurrence still far,
// a FRESH late entry stays actionable beside the upcoming one. Item 8: the
// next upcoming is co-displayed even when farther than the late entry is old.
func TestScopeSeriesKeepsFreshPastBesideFutureNext(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	yest := day.AddDate(0, 0, -1)
	now := day.Add(7 * time.Hour) // 07:00 — before today's 08:00 slot

	series := r47Series("Panacur 0.48 g",
		r47LateSlot(yest, 20, careplan.StatusLate), // yesterday 20:00, missed (11 h ago)
		r47Slot(day, 8, careplan.StatusScheduled), // today 08:00 — 1 h away
	)

	got := scopeSeriesToToday(series, 1, now)
	// 11 h overdue vs 1 h to the next: past the missed/next midpoint →
	// the late entry folds into the badge; the imminent next slot carries
	// the line (this is the granularity-aware behavior a 30-min cadence
	// needs).
	require.Equal(t, []time.Time{day.Add(8 * time.Hour)}, r47SlotTimes(got))
	require.Equal(t, 1, got.StaleLateCount, "the missed occurrence is folded with its snooze ref")
}

// One-shot group: a late entry with NO future sibling never folds — the
// absolute /preferences late cap stays the only bound.
func TestScopeSeriesKeepsLateWithoutFutureSibling(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour)

	series := r47Series("Vaccin unique",
		MedSlotView{DueAt: day.Add(9 * time.Hour), Status: string(careplan.StatusLate), Applicable: true}, // 13 h overdue, no next
	)

	got := scopeSeriesToToday(series, 1, now)
	require.Equal(t, []time.Time{day.Add(9 * time.Hour)}, r47SlotTimes(got),
		"without a next occurrence the late entry keeps its toggle")
	require.Equal(t, 0, got.StaleLateCount)
}
