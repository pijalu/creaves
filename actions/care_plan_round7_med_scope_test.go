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

// Today's slots stay; tomorrow's are dropped unless they beat the late one.
func TestScopeSeriesToTodayDropsTomorrow(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(16 * time.Hour) // 16:00

	series := r47Series("Amox 1 ml",
		r47Slot(day, 8, careplan.StatusApplied),                    // done this morning
		r47Slot(day, 12, careplan.StatusLate),                      // 4 h overdue
		r47Slot(day.AddDate(0, 0, 1), 8, careplan.StatusScheduled), // tomorrow 08:00: 16 h away
	)
	series.Rows[0].Slots[0].Done = true

	got := scopeSeriesToToday(series, now)

	// tomorrow's slot is NOT nearer (16 h away) than the late one (4 h
	// overdue), so it does not survive.
	require.Equal(t, []time.Time{day.Add(8 * time.Hour), day.Add(12 * time.Hour)}, r47SlotTimes(got),
		"only today's slots (done + late) remain")
	require.Len(t, got.Rows, 1)
	require.Equal(t, "Amox 1 ml", got.Label)
}

// The rule the caregiver asked for: show the next entry only "if the duration
// from now to the entry is shorter than the current one" — the pending late
// entry's age. A slot overdue by 20 h loses to one 10 h out (the next entry
// is genuinely close, worth showing); a slot overdue by 1 h does not.
func TestScopeSeriesToTodayKeepsNearerNext(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00
	tom := day.AddDate(0, 0, 1)

	tests := []struct {
		name     string
		overdue  time.Duration // age of the pending late entry
		keepNext bool
	}{
		{"next is nearer than the late entry is old", 20 * time.Hour, true},
		{"late entry is fresher than the next one", 1 * time.Hour, false},
		{"exactly equal keeps nothing extra", 10 * time.Hour, false}, // next is 10 h away
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			series := r47Series("Amox",
				MedSlotView{DueAt: now.Add(-tc.overdue), Status: string(careplan.StatusLate)},
				MedSlotView{DueAt: tom.Add(8 * time.Hour), Status: string(careplan.StatusScheduled)}, // 10 h away
			)

			got := scopeSeriesToToday(series, now)

			times := r47SlotTimes(got)
			if tc.keepNext {
				require.Len(t, times, 2, "the nearer next entry is kept")
				require.Equal(t, tom.Add(8*time.Hour), times[1])
				// it is bucketed as its own group, like morning/noon/evening
				require.Equal(t, slotTomorrow, got.Rows[len(got.Rows)-1].Slots[0].Slot)
			} else {
				require.Len(t, times, 1, "only the pending late entry remains")
			}
		})
	}
}

// The perfect case the user described: once the day's work is done, the
// compact medication screen is EMPTY in the evening — no leftover tomorrow.
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

	got := scopeSeriesToToday(series, now)

	require.Empty(t, got.Rows, "nothing open today: the series is dropped entirely")
	require.Equal(t, "Amox", got.Label, "the label survives so the caller can skip it")
}

// At most ONE later slot survives, not the whole rest of the schedule.
func TestScopeSeriesToTodayKeepsAtMostOneLaterSlot(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00
	series := r47Series("Amox",
		MedSlotView{DueAt: day.Add(2 * time.Hour), Status: string(careplan.StatusLate)}, // 20 h overdue
		r47Slot(day.AddDate(0, 0, 1), 6, careplan.StatusScheduled),
		r47Slot(day.AddDate(0, 0, 1), 8, careplan.StatusScheduled),
		r47Slot(day.AddDate(0, 0, 2), 8, careplan.StatusScheduled),
	)

	got := scopeSeriesToToday(series, now)

	times := r47SlotTimes(got)
	require.Len(t, times, 2, "today's late entry + the single nearest later slot")
	require.Equal(t, day.AddDate(0, 0, 1).Add(6*time.Hour), times[1],
		"the NEAREST later slot wins, not the first in the list")
}

// fillMedTiers drops a series that scoping emptied — the caller chain must
// not render an empty medication line.
func TestFillMedTiersDropsScopedOutSeries(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := []MedGroupView{{
		AnimalID: 1, AnimalLabel: "Fox-1", AnimalLink: "/animals/1",
		Series: []MedSeriesView{
			{Key: "a", Label: "All done", Rows: []MedSeriesRow{{Slots: []MedSlotView{
				{DueAt: day.Add(8 * time.Hour), Status: string(careplan.StatusApplied), Done: true},
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
				"an all-done series renders no line in the evening")
		}
	}
	require.Len(t, v.MedTiers[0], 1)
	require.Equal(t, "Pending", v.MedTiers[0][0].Series[0].Label)
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
