//go:build !sqlite

package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"creaves/models"
	"creaves/models/careplan"
)

// R4-7.23: the Treatment tab card body renders the medication series alone,
// so every day it shows must HAVE a series, and its badge must count only the
// medication slots. Before this, an animal whose protocols are all
// feeding/care got one day card per day — each badged with the open work of
// every kind, each with an empty body (measured on animals/10312: 7 cards,
// badge 3, zero rows). The badge must count what is under it.
func TestMedicationOnlyDaysDropsDaysWithNoSeries(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	plan.Now = now

	med := testSource(careplan.KindMedication, "med-1", "Quadro", map[string]interface{}{"drug": "Quadro", "dosage": "0.03 ml"})
	obs := testSource(careplan.KindObservation, "obs-1", "Observe", map[string]interface{}{"prompt": "Eating?"})
	care := testSource(careplan.KindCare, "care-1", "Care", map[string]interface{}{"note": "Check wound"})

	mk := func(h, dayOff int, status careplan.PlanStatus, src careplan.PlanSource) careplan.PlanItem {
		it := testItem(src, 1, status)
		it.Occurrence.DueAt = time.Date(2026, 10, 2+dayOff, h, 0, 0, 0, time.Local)
		return it
	}
	plan.Items = []careplan.PlanItem{
		mk(8, 0, careplan.StatusDue, med),         // today: one medication slot
		mk(9, 0, careplan.StatusDue, obs),         // today: one observation item
		mk(10, 1, careplan.StatusScheduled, care), // tomorrow: care only, no series
	}

	days := animalTreatmentDays(plan, &models.Animal{ID: 1})
	require.Len(t, days, 2, "both days still reach the data: the Protocol tab lists them")

	// Today carries medication: kept, and the badge counts the SLOT only.
	today := days[1]
	require.Equal(t, "2026-10-02", today.DateKey)
	require.Len(t, today.Group.Series, 1)
	require.Equal(t, 2, today.OpenCount, "the day still knows all its open work (Protocol tab)")
	require.Equal(t, 1, today.MedOpenCount, "the Treatment-tab badge counts the medication slot only")

	// Tomorrow is care-only: dropped from the Treatment tab…
	tomorrow := days[0]
	require.Equal(t, "2026-10-03", tomorrow.DateKey)
	require.Empty(t, tomorrow.Group.Series)
	require.Len(t, tomorrow.Items, 1)

	medDays := medicationOnlyDays(days)
	require.Len(t, medDays, 1, "only the day with a series can be shown in the Treatment tab")
	require.Equal(t, "2026-10-02", medDays[0].DateKey)
}

// An animal with NO medication at all must produce no Treatment-tab card
// rather than a stack of empty ones.
func TestMedicationOnlyDaysIsEmptyWhenNothingIsMedication(t *testing.T) {
	plan := testPlan()
	plan.Now = time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)

	obs := testSource(careplan.KindObservation, "obs-1", "Observe", map[string]interface{}{"prompt": "Eating?"})
	care := testSource(careplan.KindCare, "care-1", "Care", map[string]interface{}{"note": "Check wound"})

	mk := func(h, dayOff int, src careplan.PlanSource) careplan.PlanItem {
		it := testItem(src, 1, careplan.StatusDue)
		it.Occurrence.DueAt = time.Date(2026, 10, 2+dayOff, h, 0, 0, 0, time.Local)
		return it
	}
	plan.Items = []careplan.PlanItem{
		mk(8, 0, obs),
		mk(9, 0, care),
		mk(8, 1, obs),
	}

	days := animalTreatmentDays(plan, &models.Animal{ID: 1})
	require.Len(t, days, 2, "the Protocol tab still has both days")
	// today carries obs+care (2 open), tomorrow obs only (1 open): OpenCount
	// tracks the real work; MedOpenCount is zero because there is no slot.
	want := map[string]int{"2026-10-02": 2, "2026-10-03": 1}
	for _, d := range days {
		require.Empty(t, d.Group.Series)
		require.Zero(t, d.MedOpenCount, "no medication slot means nothing for the badge to count")
		require.Equal(t, want[d.DateKey], d.OpenCount)
	}
	require.Empty(t, medicationOnlyDays(days), "no empty cards in the Treatment tab")
}

// A medication day whose slots are ALL applied shows no badge — an applied
// day is history, not open work.
func TestMedicationOnlyDaysBadgeZeroWhenAllSlotsApplied(t *testing.T) {
	plan := testPlan()
	plan.Now = time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)

	med := testSource(careplan.KindMedication, "med-1", "Quadro", map[string]interface{}{"drug": "Quadro", "dosage": "0.03 ml"})
	it := testItem(med, 1, careplan.StatusApplied)
	it.Occurrence.DueAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{it}

	days := animalTreatmentDays(plan, &models.Animal{ID: 1})
	require.Len(t, days, 1)
	require.Len(t, days[0].Group.Series, 1, "the series still renders (history is visible)")
	require.Zero(t, days[0].MedOpenCount, "nothing open — no badge")
	require.Len(t, medicationOnlyDays(days), 1, "an all-applied day stays visible, just unbadged")
}
