package actions

import (
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// TestDayPlanViewCapDefaultsAre24h pins the R9 view-cap defaults after the
// 2026-10-27 user authorization to widen the visibility filter from 8 h to
// 24 h (bugs.md second batch: with 8 h caps, e.g. 10:00 feeding slots
// vanished from the day plan after 18:00, hiding the Apply buttons the
// caregivers still need — item "feeding group=animal missing Apply
// buttons"). Admins can still change or clear the caps per kind in the
// preferences UI; empty = no cap.
func TestDayPlanViewCapDefaultsAre24h(t *testing.T) {
	if preferenceDefaults.lateHours != 24 {
		t.Errorf("preferenceDefaults.lateHours = %d, want 24", preferenceDefaults.lateHours)
	}
	if preferenceDefaults.futureHours != 24 {
		t.Errorf("preferenceDefaults.futureHours = %d, want 24", preferenceDefaults.futureHours)
	}
	if preferenceDefaults.nowMinutes != 60 {
		t.Errorf("preferenceDefaults.nowMinutes = %d, want 60", preferenceDefaults.nowMinutes)
	}
}

// TestDefaultCapsKeepTwelveHourLateWorkVisible pins the bug behind the
// reopened "feeding group=animal is missing its Apply buttons" report: a
// 10:00 slot viewed at 20:00+ is 10 h late — the previous 8 h default cap
// filtered it (and its Apply button) out of the day plan. With the 24 h
// default the same item must stay visible/actionable, while work older
// than the cap still drops.
func TestDefaultCapsKeepTwelveHourLateWorkVisible(t *testing.T) {
	now := time.Date(2026, 10, 6, 20, 0, 0, 0, time.Local)
	prefs := models.Preference{
		LateShowHours:   nulls.Int{Int: preferenceDefaults.lateHours, Valid: true},
		FutureShowHours: nulls.Int{Int: preferenceDefaults.futureHours, Valid: true},
	}
	source := testSource(careplan.KindFeeding, "caps", "source", map[string]interface{}{})
	items := []careplan.PlanItem{
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-12 * time.Hour)}, Status: careplan.StatusLate, Applicable: true},
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-25 * time.Hour)}, Status: careplan.StatusLate, Applicable: true},
		{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(20 * time.Hour)}, Status: careplan.StatusScheduled, Applicable: true},
	}
	got := applyPreferenceCaps(items, prefs, now)
	require.Len(t, got, 2, "12 h late + 20 h future stay inside the 24 h default caps")
	require.Equal(t, items[0].Occurrence.DueAt, got[0].Occurrence.DueAt, "12 h late feeding slot keeps its Apply button")
	require.Equal(t, items[2].Occurrence.DueAt, got[1].Occurrence.DueAt, "20 h future slot stays visible")
}
