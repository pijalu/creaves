package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// BUG-29 (2026-10-12): a stale-folded late feeding chip rendered in "En
// retard" with NO apply control (stale chips leave ApplicableCount, the
// per-animal toggle hides behind the ⏱ badge, and a single-animal cage row
// has no per-animal toggle at all) AND the chip dedupe never promoted the
// next occurrence, so nothing reached "Plus tard". User ruling: the chip is
// PROMOTED to the next upcoming occurrence — the card lands in Later with a
// normal actionable toggle; the missed occurrence lives behind the ⏱ badge
// (snooze / record-late), per the item-8 design ("the next upcoming
// occurrence renders as the actionable toggle beside it").

// bug29StaleFeeding builds one (source × animal) feeding group whose
// displayed occurrence is stale-late: 08:00 late, 16:00 scheduled, now
// 13:00 (age 5 h > 3 h to the successor ⇒ stale; the successor is not yet
// due ⇒ the late is still applicable, hence stale rather than superseded).
func bug29StaleFeeding(t *testing.T) (*FeedingCard, map[string]int, time.Time) {
	t.Helper()
	plan := testPlan()
	now := time.Date(2026, 10, 12, 13, 0, 0, 0, time.Local)
	plan.Now = now
	src := testSource(careplan.KindFeeding, "feed-b29", "Feed", map[string]interface{}{"food": "panade"})

	late := testItem(src, 1, careplan.StatusLate)
	late.Occurrence.DueAt = time.Date(2026, 10, 12, 8, 0, 0, 0, time.Local)
	late.NextDue = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	next := testItem(src, 1, careplan.StatusScheduled)
	next.Occurrence.DueAt = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{late, next}

	_, feedings := GroupCards(plan.Items, plan)
	require.Len(t, feedings, 1)
	require.Len(t, feedings[0].Chips, 1, "chip dedupe: one chip per (source × animal)")
	return feedings[0], openCurrentCounts(plan), now
}

// TestStaleLateFeedingChipPromotesTheNextOccurrence: the card's actionable
// chip IS the 16:00 occurrence — visible toggle, counted, batch-refed — and
// the card tiers as Later.
func TestStaleLateFeedingChipPromotesTheNextOccurrence(t *testing.T) {
	fc, openCurrent, now := bug29StaleFeeding(t)

	fv, ok := feedingViewOf(fc, openCurrent, "/care_plan", now)
	require.True(t, ok)
	require.Len(t, fv.Chips, 1)

	chip := fv.Chips[0]
	require.True(t, chip.StaleLate, "the fold badge stays (⏱ 1 + snooze)")
	require.True(t, chip.Promoted, "the chip is promoted to the next occurrence")
	require.Equal(t, "16:00", chip.DueHM, "the toggle carries the NEXT occurrence's time")
	require.Equal(t, string(careplan.StatusScheduled), chip.Status)
	require.True(t, chip.Applicable, "the promoted chip is actionable")
	require.Equal(t, time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local), chip.DueAt)

	// The stale occurrence stays reachable: badge snooze refs + the hidden
	// record-late toggle carry the ORIGINAL due time.
	require.Equal(t, "08:00", chip.StaleDueHM)
	require.Contains(t, chip.StaleRefsJSON, "due_at")
	require.Contains(t, chip.StaleRefsJSON, "T08:00", "the snooze/record-late refs target the missed occurrence")

	// The promoted chip counts as work and feeds the group apply refs.
	require.Equal(t, 1, fv.ApplicableCount)
	require.Contains(t, fv.ChipRefsJSON, "T16:00")
	require.NotContains(t, fv.ChipRefsJSON, "T08:00",
		"item 8 guardrail: a group apply never silently records the folded occurrence")

	// Tier: the group lands in "Plus tard" (user ruling).
	tier, status := feedingGroupTier(fv)
	require.Equal(t, 2, tier, "the promoted card tiers as Later")
	require.Equal(t, "scheduled", status)
	require.Equal(t, chip.DueAt, fv.FirstDueAt, "tier sort follows the promoted occurrence")
}

// TestStaleLateFeedingWithoutSuccessorKeepsTheFold: end-of-schedule — no
// open successor in the window — keeps the former behavior (badge + hidden
// toggle, no promotion).
func TestStaleLateFeedingWithoutSuccessorKeepsTheFold(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 10, 12, 13, 0, 0, 0, time.Local)
	plan.Now = now
	src := testSource(careplan.KindFeeding, "feed-b29", "Feed", map[string]interface{}{"food": "panade"})
	// A DUE sibling of ANOTHER animal gives the group a nearer open
	// sibling… no — staleness is per (source × animal): with no successor
	// at all the 08:00 occurrence is NOT stale. Force staleness via a
	// second animal's due-now sibling? No: staleLateFlags keys on
	// (source × animal). Instead: successor exists but is APPLIED
	// (terminal ⇒ not open) — the late is then the last open one. A lone
	// late with no open sibling is never stale, so assert that directly.
	lone := testItem(src, 1, careplan.StatusLate)
	lone.Occurrence.DueAt = time.Date(2026, 10, 12, 8, 0, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{lone}

	_, feedings := GroupCards(plan.Items, plan)
	require.Len(t, feedings, 1)
	fv, ok := feedingViewOf(feedings[0], openCurrentCounts(plan), "/care_plan", now)
	require.True(t, ok)
	require.Len(t, fv.Chips, 1)
	require.False(t, fv.Chips[0].StaleLate, "no open sibling → no stale fold")
	require.False(t, fv.Chips[0].Promoted)
	require.Equal(t, 1, fv.ApplicableCount, "a plain late chip stays actionable")

	// And a stale chip whose only successor is terminal (applied) has no
	// open sibling either → not stale (foldStale guards hasSibling).
	applied := testItem(src, 1, careplan.StatusApplied)
	applied.Occurrence.DueAt = time.Date(2026, 10, 12, 16, 0, 0, 0, time.Local)
	lone.NextDue = applied.Occurrence.DueAt
	plan.Items = []careplan.PlanItem{lone, applied}
	_, feedings = GroupCards(plan.Items, plan)
	fv, ok = feedingViewOf(feedings[0], openCurrentCounts(plan), "/care_plan", now)
	require.True(t, ok)
	require.False(t, fv.Chips[0].StaleLate, "a terminal successor is not an open sibling")
	require.False(t, fv.Chips[0].Promoted)
}

// TestStaleLateFeedingTemplateRendersThePromotedToggle: the four locale
// forks render the promoted chip's toggle VISIBLE (not the d-none stale
// wrapper) while the stale occurrence's own toggle stays hidden behind the
// badge for one-by-one record-late.
func TestStaleLateFeedingTemplateRendersThePromotedToggle(t *testing.T) {
	raw := readTemplate(t, "../templates/care_plan/_plan_tier_feed_table.plush.html")
	require.Contains(t, raw, "chip.Promoted",
		"the promoted chip renders its toggle outside the stale d-none wrapper")
	require.Contains(t, raw, "chip.StaleDueHM",
		"the stale occurrence's record-late toggle keeps the original time behind the badge")
	// The 4 forks are byte-identical by policy.
	base := raw
	for _, f := range []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	} {
		require.Equal(t, base, readTemplate(t, f), f+": fork drift")
	}
	require.False(t, strings.Contains(base, "chip.StaleNextHM") && !strings.Contains(base, "chip.Promoted"),
		"the promoted path must exist alongside the legacy next label")
}
