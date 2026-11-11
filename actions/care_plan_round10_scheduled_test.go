package actions

// B10-7 — scheduled occurrences within the future horizon render as open
// work. The per-kind view caps (applyPreferenceCaps) already drop scheduled
// items beyond `future_show_hours` BEFORE the view model; the grouped-view
// builders (cleanup careItemCounts, feeding feedingViewOf) must therefore
// NOT re-drop every scheduled occurrence — a 09:00 cleanup rule with the
// default 60-min lookahead stayed invisible all morning while the summary
// strip counted it in "Later".

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// TestB10_7ScheduledCleanupRenders: a scheduled (future, beyond the
// lookahead) cleanup occurrence yields a CareView row with an applicable
// slot and a batch ref — the tab must not go empty while the work is
// planned within the horizon.
func TestB10_7ScheduledCleanupRenders(t *testing.T) {
	now := time.Date(2026, 10, 6, 7, 0, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	cln := testSource(careplan.KindCleanup, "cln-b10", "Cage scrub", map[string]interface{}{"note": "scrub"})
	plan.Items = []careplan.PlanItem{
		// 09:00 cleanup viewed at 07:00: beyond the default 60-min
		// lookahead → StatusScheduled, still applicable.
		phase3ItemAt(cln, 1, careplan.StatusScheduled, now.Add(2*time.Hour)),
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)
	require.Len(t, v.Cares, 1, "the scheduled cleanup renders as an open row")
	row := v.Cares[0]
	require.Equal(t, 1, row.ApplicableCount)
	require.Len(t, row.TimeGroups, 1)
	slot := row.TimeGroups[0].Slots[0]
	require.Equal(t, string(careplan.StatusScheduled), slot.Status)
	require.True(t, slot.Applicable, "scheduled within the window is actionable")
	require.Equal(t, "btn-light border", slot.TierClass, "scheduled slot keeps the future tier colour")

	// The batch ref rides on the row so the cage group-check covers it.
	require.Contains(t, row.ChipRefsJSON, "cln-b10")

	// And the row lands in the LATER tier (its most urgent occurrence).
	stamped := careRowOf(t, v.CareTiers[2], row.Cage)
	require.Equal(t, 2, stamped.Tier)
}

// TestB10_7ScheduledFeedingChipRenders: same contract for the feeding
// grouped view — a scheduled feeding chip within the horizon renders.
func TestB10_7ScheduledFeedingChipRenders(t *testing.T) {
	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	feed := testSource(careplan.KindFeeding, "feed-b10", "Feed", map[string]interface{}{"food": "Croquettes"})
	plan.Items = []careplan.PlanItem{
		// 16:00 feeding viewed at 10:00 → scheduled, applicable.
		phase3ItemAt(feed, 1, careplan.StatusScheduled, now.Add(6*time.Hour)),
	}

	rows := feedingViewsOf(plan, "/care_plan?kind=feeding", "")
	require.Len(t, rows, 1, "the scheduled feeding chip renders")
	require.Len(t, rows[0].Chips, 1)
	require.Equal(t, string(careplan.StatusScheduled), rows[0].Chips[0].Status)
	require.True(t, rows[0].Chips[0].Applicable)
	require.Equal(t, 1, rows[0].ApplicableCount, "the batch ref counter includes it")
}
