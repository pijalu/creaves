package actions

import (
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// TestAnimalTreatmentDays (R3-6): the Treatment tab rebuilds on the
// care-plan engine — per-day togglable medication series over the
// history+future window. Days come newest-first, only medication
// occurrences of THIS animal land, overridden stay out, and each day's
// Group carries the `_med_series`-ready series (slot buttons togglable).
func TestAnimalTreatmentDays(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	plan.Now = now

	med := testSource(careplan.KindMedication, "med-1", "Quadro", map[string]interface{}{"drug": "Quadro", "dosage": "0.03 ml"})
	feed := testSource(careplan.KindFeeding, "feed-1", "Feed", map[string]interface{}{"food": "x"})

	// today 08:00 (applied, undoable), today 18:00 (open), tomorrow 12:00
	// (open), a feeding occurrence (excluded — not medication), another
	// animal's medication (excluded), an overridden one (excluded).
	mk := func(h, dayOff int, status careplan.PlanStatus, animalID int, src careplan.PlanSource) careplan.PlanItem {
		it := testItem(src, animalID, status)
		it.Occurrence.DueAt = time.Date(2026, 10, 2+dayOff, h, 0, 0, 0, time.Local)
		if status == careplan.StatusApplied {
			it.Application = &careplan.ApplicationView{Status: "applied"}
		}
		return it
	}
	plan.Items = []careplan.PlanItem{
		mk(8, 0, careplan.StatusApplied, 1, med),
		mk(18, 0, careplan.StatusDue, 1, med),
		mk(12, 1, careplan.StatusScheduled, 1, med),
		mk(9, 0, careplan.StatusDue, 1, feed),        // feeding — excluded
		mk(9, 0, careplan.StatusDue, 2, med),         // other animal — excluded
		mk(10, 0, careplan.StatusOverridden, 1, med), // overridden — excluded
	}

	days := animalTreatmentDays(plan, &models.Animal{ID: 1})

	require.Len(t, days, 2, "today + tomorrow only (feeding/other/overridden excluded)")
	// newest day first: tomorrow (10-03) before today (10-02)
	require.Equal(t, "2026-10-03", days[0].DateKey)
	require.False(t, days[0].Current)
	require.True(t, days[0].Future)
	require.Equal(t, "2026-10-02", days[1].DateKey)
	require.True(t, days[1].Current)
	require.False(t, days[1].Future)

	// tomorrow: one open slot → OpenCount 1, one series line
	require.Equal(t, 1, days[0].OpenCount)
	require.Len(t, days[0].Group.Series, 1)
	require.Equal(t, 1, days[0].Group.AnimalID)

	// today: 18:00 open (08:00 applied) → OpenCount 1; series carries both
	// slots and the applied one is marked done.
	require.Equal(t, 1, days[1].OpenCount)
	require.Len(t, days[1].Group.Series, 1)
	var applied, open int
	for _, row := range days[1].Group.Series[0].Rows {
		for _, s := range row.Slots {
			if s.Applied {
				applied++
			} else if !s.Done {
				open++
			}
		}
	}
	require.Equal(t, 1, applied, "the 08:00 applied slot stays visible as done")
	require.Equal(t, 1, open, "the 18:00 open slot stays togglable")
}

// TestAnimalTreatmentDaysEmpty: an animal with no medication occurrence
// in the window renders no day group (the template hides the block).
func TestAnimalTreatmentDaysEmpty(t *testing.T) {
	plan := testPlan()
	plan.Now = time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	require.Empty(t, animalTreatmentDays(plan, &models.Animal{ID: 1}))
	require.Empty(t, animalTreatmentDays(nil, &models.Animal{ID: 1}))
}

// TestTreatmentPlanWindow (R3-6): 14 days history, 5 days forward — the
// forward cap bounds open-ended protocols.
func TestTreatmentPlanWindow(t *testing.T) {
	now := time.Date(2026, 10, 2, 15, 30, 0, 0, time.Local)
	from, to := TreatmentPlanWindow(now)
	require.Equal(t, time.Date(2026, 9, 18, 0, 0, 0, 0, time.Local), from)
	require.Equal(t, time.Date(2026, 10, 7, 23, 59, 59, int(time.Second-time.Nanosecond), time.Local), to)
}

// TestTreatmentPlanWindowSurvivesEngineClamp (R3-6 regression): the §6.1
// window cap inside BuildDayPlan must NOT truncate the R3-6 window — it
// bounds the forward reach (max(from, now)+14d), not the total span, so a
// past-spanning caller keeps its history AND its future horizon.
func TestTreatmentPlanWindowSurvivesEngineClamp(t *testing.T) {
	now := time.Date(2026, 10, 2, 15, 30, 0, 0, time.Local)
	from, to := TreatmentPlanWindow(now) // [09-18, 10-07] — 19 days > 14d cap
	maxTo := maxTime(from, now).Add(planWindowMaxDays * 24 * time.Hour)
	require.False(t, to.After(maxTo), "R3-6 window end %s must fit the engine forward cap %s", to, maxTo)
	require.True(t, to.After(now.Add(4*24*time.Hour)), "the +5d forward reach must survive (clamping to from+14d cut it at 10-02)")
}
