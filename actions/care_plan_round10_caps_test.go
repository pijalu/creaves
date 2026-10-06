package actions

import (
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

func TestPreferenceCapsApplyAcrossAllPlanKinds(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	prefs := models.Preference{LateShowHours: nulls.Int{Int: 8, Valid: true}, FutureShowHours: nulls.Int{Int: 8, Valid: true}}
	for _, kind := range []string{careplan.KindFeeding, careplan.KindMedication, careplan.KindCare, careplan.KindCleanup, careplan.KindWeighing, careplan.KindObservation} {
		t.Run(kind, func(t *testing.T) {
			source := testSource(kind, "caps", "source", map[string]interface{}{})
			items := []careplan.PlanItem{
				{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-8 * time.Hour)}, Status: careplan.StatusLate},
				{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(8 * time.Hour)}, Status: careplan.StatusScheduled},
			}
			require.Empty(t, applyPreferenceCaps(items, prefs, now))
		})
	}
}

func TestPreferenceCapsRemoveYesterdayActionBeforeProjection(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	source := testSource(careplan.KindCare, "yesterday", "Care", map[string]interface{}{"note": "care"})
	item := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: time.Date(2026, 10, 5, 4, 0, 0, 0, time.Local)}, Status: careplan.StatusLate, Applicable: true}
	prefs := models.Preference{LateShowHours: nulls.Int{Int: 8, Valid: true}}
	plan := testPlan()
	plan.Now = now
	plan.Items = applyPreferenceCaps([]careplan.PlanItem{item}, prefs, now)
	require.Empty(t, plan.Items, "over-cap yesterday occurrence cannot reach badge/action projection")
	view := BuildDayPlanView(plan, ViewCompact, "", careplan.KindCare, "", now)
	require.Empty(t, view.Cares, "no actionable row remains")
	require.Empty(t, view.Tiers[0].Cards)
}

func TestPreferenceCapsExcludeAtAndBeyondLateAndFutureBoundaries(t *testing.T) {
	now := time.Date(2026, 10, 6, 12, 0, 0, 0, time.Local)
	source := testSource(careplan.KindMedication, "caps", "Medication", map[string]interface{}{})
	item := func(status careplan.PlanStatus, due time.Time) careplan.PlanItem {
		return careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: due}, Status: status}
	}
	items := []careplan.PlanItem{
		item(careplan.StatusLate, now.Add(-9*time.Hour)),
		item(careplan.StatusLate, now.Add(-8*time.Hour)),
		item(careplan.StatusLate, now.Add(-7*time.Hour-59*time.Minute)),
		item(careplan.StatusMissing, now.Add(-9*time.Hour)),
		item(careplan.StatusScheduled, now.Add(9*time.Hour)),
		item(careplan.StatusScheduled, now.Add(8*time.Hour)),
		item(careplan.StatusScheduled, now.Add(7*time.Hour+59*time.Minute)),
		item(careplan.StatusApplied, now.Add(-24*time.Hour)),
	}
	prefs := models.Preference{LateShowHours: nulls.Int{Int: 8, Valid: true}, FutureShowHours: nulls.Int{Int: 8, Valid: true}}
	got := applyPreferenceCaps(items, prefs, now)
	require.Len(t, got, 3)
	require.Equal(t, items[2].Occurrence.DueAt, got[0].Occurrence.DueAt, "just-inside late cap remains")
	require.Equal(t, items[6].Occurrence.DueAt, got[1].Occurrence.DueAt, "just-inside future cap remains")
	require.Equal(t, items[7].Occurrence.DueAt, got[2].Occurrence.DueAt, "terminal items remain for History")
	require.Equal(t, careplan.StatusApplied, got[2].Status, "terminal status is unchanged")
}
