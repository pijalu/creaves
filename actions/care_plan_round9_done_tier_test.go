package actions

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// R9-2 (bugs.md): a medication series is "Done" ONLY when EVERY repeated
// occurrence is applied. A fully-applied series no longer disappears —
// it lands in a new MedDone section (same line layout, green ✓ toggles).
// A partially-applied series stays in late/now/later by its NEXT OPEN
// slot, and its applied slots stay visible (green toggles) alongside.

func r92Slot(day time.Time, h int, status careplan.PlanStatus) MedSlotView {
	return MedSlotView{
		DueAt:   day.Add(time.Duration(h) * time.Hour),
		Status:  string(status),
		Done:    status == careplan.StatusApplied || status == careplan.StatusSkipped || status == careplan.StatusDeferred,
		Applied: status == careplan.StatusApplied,
	}
}

func r92Series(label string, slots ...MedSlotView) MedSeriesView {
	return MedSeriesView{Key: label, Label: label, Rows: []MedSeriesRow{{Slots: slots}}}
}

func r92Group(series ...MedSeriesView) []MedGroupView {
	return []MedGroupView{{
		AnimalID: 1, AnimalLabel: "Fox-1", AnimalLink: "/animals/1",
		Series: series,
	}}
}

// 100%-applied series → MedDone, not dropped.
func TestFillMedTiersFullyAppliedSeriesGoesToDone(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := r92Group(
		r92Series("Amox 1 ml",
			r92Slot(day, 8, careplan.StatusApplied),
			r92Slot(day, 12, careplan.StatusApplied),
			r92Slot(day, 18, careplan.StatusApplied),
		),
	)

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(19*time.Hour)) // 19:00 — all three applied

	require.Len(t, v.MedDone, 1, "the fully-applied series lands in MedDone")
	require.Equal(t, "Amox 1 ml", v.MedDone[0].Series[0].Label)
	for i := 0; i < 3; i++ {
		require.Empty(t, v.MedTiers[i], "no open tier holds a fully-applied series")
	}
	// Applied slots stay visible (the green ✓ toggles).
	slots := flatSlots(v.MedDone[0].Series[0])
	require.Len(t, slots, 3, "all three applied occurrences stay visible")
	for _, s := range slots {
		require.True(t, s.Applied, "every slot of the done series is applied")
	}
}

// Worked example from bugs.md (animal 1234, 08:00/12:00/18:00): after
// applying 08:00 the series is NOT done — it stays by its next open slot
// (12:00), and the applied 08:00 toggle stays visible.
func TestFillMedTiersPartialSeriesStaysOpenWithAppliedVisible(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := r92Group(
		r92Series("Amox 1 ml",
			r92Slot(day, 8, careplan.StatusApplied), // applied
			r92Slot(day, 12, careplan.StatusDue),    // next open
			r92Slot(day, 18, careplan.StatusScheduled),
		),
	)

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(12*time.Hour)) // 12:00

	require.Empty(t, v.MedDone, "a partially-applied series is NOT done")
	require.Len(t, v.MedTiers[1], 1, "the series sits in NOW by its 12:00 open slot")
	slots := flatSlots(v.MedTiers[1][0].Series[0])
	require.Len(t, slots, 3, "applied + open occurrences all stay on the line")
	require.True(t, slots[0].Applied, "the 08:00 applied toggle stays visible (green)")
	require.False(t, slots[1].Done, "12:00 is still open")
	require.False(t, slots[2].Done, "18:00 is still open")
}

// Undo of the last applied occurrence pulls the series OUT of Done and
// back into its correct open tier (the viewmodel rebuild does it — the
// in-place toggle marks the button open, the refresh re-tiers).
func TestFillMedTiersUndoneSeriesLeavesDone(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := r92Group(
		r92Series("Amox 1 ml",
			r92Slot(day, 8, careplan.StatusApplied),
			r92Slot(day, 12, careplan.StatusApplied),
			r92Slot(day, 18, careplan.StatusLate), // undone — back to open (late)
		),
	)

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(19*time.Hour)) // 19:00

	require.Empty(t, v.MedDone, "an undone series is no longer fully applied")
	require.Len(t, v.MedTiers[0], 1, "the series returns to LATE (18:00 late open slot)")
}

// A series whose ONLY open slots are FUTURE days (today's occurrences all
// applied) is Done FOR TODAY: it joins MedDone showing today's applied
// record, and does not reappear in the open tiers. This is the real-world
// shape of a multi-day protocol partway through — not 100%-terminal overall,
// but nothing left to do today.
func TestFillMedTiersDoneForTodayWithFutureOpenSlots(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	tom := day.AddDate(0, 0, 1)
	groups := r92Group(
		r92Series("Baycox",
			r92Slot(day, 12, careplan.StatusApplied),                    // today, applied
			r92Slot(tom, 12, careplan.StatusScheduled),                  // tomorrow, open
			r92Slot(tom.AddDate(0, 0, 1), 12, careplan.StatusScheduled), // +2d, open
		),
	)

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(13*time.Hour)) // 13:00 today

	require.Len(t, v.MedDone, 1, "today all-applied + future open → Done for today")
	require.Equal(t, "Baycox", v.MedDone[0].Series[0].Label)
	for i := 0; i < 3; i++ {
		require.Empty(t, v.MedTiers[i], "no open tier holds a done-for-today series")
	}
	// Only today's applied occurrence shows — the future slots wait for their day.
	slots := flatSlots(v.MedDone[0].Series[0])
	require.Len(t, slots, 1, "the Done line shows today's record only")
	require.True(t, slots[0].Applied)
}

// The Done badge counts the fully-done SERIES it lists.
func TestFillMedTiersDoneCountTracksSeries(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	groups := r92Group(
		r92Series("A", r92Slot(day, 8, careplan.StatusApplied)),
		r92Series("B", r92Slot(day, 9, careplan.StatusApplied), r92Slot(day, 10, careplan.StatusApplied)),
		r92Series("C", r92Slot(day, 11, careplan.StatusLate)), // open
	)

	v := &DayPlanView{}
	v.fillMedTiers(groups, day.Add(12*time.Hour))

	require.Equal(t, 2, v.MedDoneCount, "two fully-applied series")
	require.Equal(t, "2", v.MedDoneCountCap)
	require.Len(t, v.MedDone, 2)
}
