package actions

import (
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// TestDayPlanViewCapDefaultsAre8h pins the 2026-10-07 user ruling (day-plan
// review item 6): the seeded view caps are 8 h late / 8 h future / 60 min
// "now" window. (Supersedes the 2026-10-27 widening to 24 h — stale late
// work is no longer lost, the relative-distance heuristic folds it into a
// "N late" badge on the record.) Admins can still change or clear the caps
// per kind in the preferences UI; empty = no cap.
func TestDayPlanViewCapDefaultsAre8h(t *testing.T) {
	if preferenceDefaults.lateHours != 8 {
		t.Errorf("preferenceDefaults.lateHours = %d, want 8", preferenceDefaults.lateHours)
	}
	if preferenceDefaults.futureHours != 8 {
		t.Errorf("preferenceDefaults.futureHours = %d, want 8", preferenceDefaults.futureHours)
	}
	if preferenceDefaults.nowMinutes != 60 {
		t.Errorf("preferenceDefaults.nowMinutes = %d, want 60", preferenceDefaults.nowMinutes)
	}
}

// TestDefaultCapsBoundTheDayPlanWindow pins the window semantics of the 8 h
// defaults: work overdue by more than 8 h and work scheduled beyond 8 h
// leave the day plan (the outer bound — the relative-distance heuristic
// folds the in-between stale late entries into a badge), while work inside
// the window stays visible/actionable.
func TestDefaultCapsBoundTheDayPlanWindow(t *testing.T) {
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, time.Local)
	prefs := models.Preference{
		LateShowHours:   nulls.Int{Int: preferenceDefaults.lateHours, Valid: true},
		FutureShowHours: nulls.Int{Int: preferenceDefaults.futureHours, Valid: true},
	}
	source := testSource(careplan.KindFeeding, "caps", "source", map[string]interface{}{})
	items := []careplan.PlanItem{
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-7 * time.Hour)}, Status: careplan.StatusLate, Applicable: true},
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-9 * time.Hour)}, Status: careplan.StatusLate, Applicable: true},
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(7 * time.Hour)}, Status: careplan.StatusScheduled, Applicable: true},
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(9 * time.Hour)}, Status: careplan.StatusScheduled, Applicable: true},
	}
	got := applyPreferenceCaps(items, prefs, now)
	require.Len(t, got, 2, "work beyond the 8 h caps drops in both directions")
	require.Equal(t, items[0].Occurrence.DueAt, got[0].Occurrence.DueAt, "7 h late feeding slot keeps its Apply button")
	require.Equal(t, items[2].Occurrence.DueAt, got[1].Occurrence.DueAt, "7 h future slot stays visible")
}
