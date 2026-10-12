package actions

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// BUG-29-C (user scope extension 2026-10-12): "for the cleanup — the 'LATE'
// should only show based on the distance with 'now' — eg: next cleanup is
// in less than 2 hours so it's odd to show them late and should be likely
// set in 'plus tard'. You may show the item in the 'later' but with a
// 'X late' to show the missing elements."
//
// The item-8 fold already HIDES the stale late occurrences behind the row's
// "⏱ N" badge, but fillCareTiers/careRowTier still tiered the row by its
// MOST URGENT open occurrence — INCLUDING the folded stale ones — so the
// row stayed in "En retard" with no visible late toggle. The tier and the
// sort key must follow the most urgent NON-stale open occurrence instead;
// the badge (StaleCount/StaleRefsJSON) stays on the row.

// bug29StaleCleanup builds one (source × cage) cleanup group whose only
// late occurrence is stale: 08:00 late (still applicable), 16:00
// scheduled, now 13:00 (age 5 h > 3 h to the successor ⇒ stale).
func bug29StaleCleanup(t *testing.T) *DayPlan {
	t.Helper()
	plan := testPlan()
	now := time.Date(2026, 10, 12, 13, 0, 0, 0, time.Local)
	plan.Now = now
	src := testSource(careplan.KindCleanup, "cln-b29", "Cage scrub", map[string]interface{}{"note": "scrub"})

	late := testItem(src, 1, careplan.StatusLate)
	late.Occurrence.DueAt = time.Date(2026, 10, 12, 8, 0, 0, 0, time.Local)
	late.NextDue = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	next := testItem(src, 1, careplan.StatusScheduled)
	next.Occurrence.DueAt = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{late, next}
	return plan
}

// TestStaleLateCleanupRowTiersAsLater: the row lands in "Plus tard" — not
// "En retard" — and keeps its "⏱ 1" fold badge; the leading sort time is
// the successor's.
func TestStaleLateCleanupRowTiersAsLater(t *testing.T) {
	plan := bug29StaleCleanup(t)
	cares := careViewsOf(plan, "/care_plan", "")
	require.Len(t, cares, 1)
	cv := cares[0]

	require.Equal(t, 1, cv.StaleCount, "the missed occurrence stays behind the ⏱ badge")
	require.Contains(t, cv.StaleRefsJSON, "T08:00", "the badge snooze/record-late refs target the missed occurrence")
	require.Equal(t, 1, cv.ApplicableCount, "the successor stays actionable")

	tier, status := careRowTier(cv)
	require.Equal(t, 2, tier, "user ruling: an all-stale-late row tiers as Later, not Late")
	require.Equal(t, string(careplan.StatusScheduled), status)
	require.Equal(t, time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local), firstCareDue(cv),
		"the tier sort follows the first NON-stale actionable occurrence")
}

// TestStaleLateCleanupRowWithFreshLateStaysLate: a row that ALSO has a
// non-stale late occurrence still tiers Late — the fold only re-tiers rows
// whose late work is ENTIRELY stale.
func TestStaleLateCleanupRowWithFreshLateStaysLate(t *testing.T) {
	plan := bug29StaleCleanup(t)
	src := plan.Items[0].Occurrence.Source
	fresh := testItem(src, 2, careplan.StatusLate)
	fresh.Occurrence.DueAt = time.Date(2026, 10, 12, 11, 30, 0, 0, time.Local) // age 1 h 30 — not stale
	fresh.NextDue = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	plan.Items = append(plan.Items, fresh)

	cares := careViewsOf(plan, "/care_plan", "")
	require.Len(t, cares, 1)
	cv := cares[0]

	tier, status := careRowTier(cv)
	require.Equal(t, 0, tier, "a fresh (non-stale) late occurrence keeps the row in Late")
	require.Equal(t, string(careplan.StatusLate), status)
	require.Equal(t, fresh.Occurrence.DueAt, firstCareDue(cv),
		"the sort leads with the fresh late occurrence")
	require.Equal(t, 1, cv.StaleCount, "the stale 08:00 occurrence stays folded")
}

// TestStaleLateCleanupNoSuccessorKeepsLateTier: end-of-schedule — a lone
// late is never stale (no open sibling), so the row tiers Late as before.
func TestStaleLateCleanupNoSuccessorKeepsLateTier(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 10, 12, 13, 0, 0, 0, time.Local)
	plan.Now = now
	src := testSource(careplan.KindCleanup, "cln-b29", "Cage scrub", map[string]interface{}{"note": "scrub"})
	lone := testItem(src, 1, careplan.StatusLate)
	lone.Occurrence.DueAt = time.Date(2026, 10, 12, 8, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{lone}

	cares := careViewsOf(plan, "/care_plan", "")
	require.Len(t, cares, 1)
	cv := cares[0]
	require.Equal(t, 0, cv.StaleCount, "no open sibling → no stale fold")
	tier, status := careRowTier(cv)
	require.Equal(t, 0, tier)
	require.Equal(t, string(careplan.StatusLate), status)
	require.Equal(t, lone.Occurrence.DueAt, firstCareDue(cv))
}
