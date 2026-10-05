package actions

import (
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// R4-7.14, the three things the caregiver reported on
// `/care_plan?kind=feeding`:
//
//  1. "the late items background color *must* match red"
//  2. "order the item based on the earliest item to execute"
//  3. "center the global checkmark in the middle"
//
// Measured before the change, live:
//
//	late tier: rowBg=rgba(0,0,0,0) inside paneBg=rgb(255,255,255)
//	          → a row in "Late" was as white as a row in "Later"; the tint
//	            existed only on the section border.
//	group dot: rgb(224,168,0) — amber, the same colour as the "due now" tier.
//	order:     first 24 due times read 16:00 | 18:00 | 17:00 | 16:00 | 17:15 …
//	group check: 10.5 px from the top of a 321 px cell, 280.5 px from the bottom.

// feedChip builds an applicable, non-superseded feeding chip due at the given
// hour — the only kind that is real work, and so the only kind that may lead
// the tier order (see firstChipDue).
func feedChip(hour, min int) FeedingChip {
	due := time.Date(2026, 10, 2, hour, min, 0, 0, time.UTC)
	return FeedingChip{
		AnimalID:   1,
		Label:      "1/26",
		Status:     "late",
		DueAt:      due,
		DueHM:      due.Format("15:04"),
		Applicable: true,
	}
}

// TestFeedingRowsAreOrderedByEarliestDue: the tier says late/now/later, but
// INSIDE a tier the rows used to arrive in cage order, so the caregiver had to
// re-sort the section by eye.
func TestFeedingRowsAreOrderedByEarliestDue(t *testing.T) {
	v := &DayPlanView{}
	v.Feedings = []FeedingGroupView{
		{Cage: "C1", Food: "pellets", Chips: []FeedingChip{feedChip(18, 0)}},
		{Cage: "C2", Food: "pellets", Chips: []FeedingChip{feedChip(9, 0), feedChip(17, 0)}},
		{Cage: "C3", Food: "hay", Chips: []FeedingChip{feedChip(8, 0)}},
		{Cage: "C4", Food: "pellets", Chips: []FeedingChip{feedChip(10, 0)}},
	}
	fillFeedTiers(v)

	require.Len(t, v.FeedTiers[0], 4, "all four groups carry late work")
	var got []string
	for _, f := range v.FeedTiers[0] {
		got = append(got, f.Cage)
	}
	require.Equal(t, []string{"C3", "C2", "C4", "C1"}, got,
		"the group whose EARLIEST occurrence is 08:00 must come before the "+
			"one whose earliest is 10:00, not be sorted on its first chip")

	// The key is the EARLIEST open occurrence, so a group whose first chip is
	// late but whose second is urgent still leads on the urgent one.
	var firsts []time.Time
	for _, f := range v.FeedTiers[0] {
		firsts = append(firsts, f.FirstDueAt)
	}
	require.True(t, sort.SliceIsSorted(firsts, func(a, b int) bool {
		return firsts[a].Before(firsts[b])
	}), "FirstDueAt must be non-decreasing down the tier")
}

// TestFeedingOrderIgnoresSupersededAndNonApplicableChips: a superseded chip is
// history and a non-applicable chip is nothing to do; neither may lead the
// order, or a group would sort by work that no longer exists.
func TestFeedingOrderIgnoresSupersededAndNonApplicableChips(t *testing.T) {
	late := feedChip(23, 0) // 23:00, but superseded
	late.Superseded = true  // history
	done := feedChip(22, 0) // 22:00, no longer applicable
	done.Applicable = false // nothing to do
	v := &DayPlanView{}
	v.Feedings = []FeedingGroupView{
		{Cage: "A", Food: "x", Chips: []FeedingChip{done, late}},
		{Cage: "B", Food: "x", Chips: []FeedingChip{feedChip(9, 0)}},
	}
	fillFeedTiers(v)
	require.Equal(t, []string{"B", "A"}, []string{v.FeedTiers[0][0].Cage, v.FeedTiers[0][1].Cage},
		"a group with no applicable chip must not jump ahead on a stale due time")
}

// TestFeedingOrderIsStableAcrossTwoFills: the screen auto-refreshes, and a
// re-sort that reshuffles equal-key rows on every refresh would make a
// caregiver's click land on a different animal.
func TestFeedingOrderIsStableAcrossTwoFills(t *testing.T) {
	build := func() *DayPlanView {
		v := &DayPlanView{}
		for _, c := range []string{"D", "B", "F", "A", "C", "E"} {
			v.Feedings = append(v.Feedings, FeedingGroupView{
				Cage: c, Food: "x", Chips: []FeedingChip{feedChip(9, 0)},
			})
		}
		fillFeedTiers(v)
		return v
	}
	first, second := build(), build()
	require.Equal(t, first.FeedTiers[0], second.FeedTiers[0],
		"identical input must produce an identical order")
	// The tie-break is the cage label, so the order is alphabetical, not the
	// input order (D, B, F, A, C, E).
	require.Equal(t, []string{"A", "B", "C", "D", "E", "F"},
		[]string{
			first.FeedTiers[0][0].Cage, first.FeedTiers[0][1].Cage,
			first.FeedTiers[0][2].Cage, first.FeedTiers[0][3].Cage,
			first.FeedTiers[0][4].Cage, first.FeedTiers[0][5].Cage,
		})
}

// TestLateTierRowsCarryTheRedTint: the regression is a CSS one — the collapse
// body was forced to solid white, which covered the tier's own tint.
func TestLateTierRowsCarryTheRedTint(t *testing.T) {
	forks := []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/index.plush.fr.html",
		"../templates/care_plan/index.plush.de.html",
		"../templates/care_plan/index.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.NotContains(t, raw, ".plan-tier-body { background: #fff; }",
			f+": the collapse body must not be forced white — it covers the tier tint")
		require.Contains(t, raw, ".plan-tier-body { background: transparent; }",
			f+": the body must inherit the tier tint")
		// The cells go transparent too: a white cell would re-cover exactly the
		// tint the body was just allowed to show.
		require.Contains(t, raw, ".plan-tier-body .table td,",
			f+": the cells must not re-cover the tint the body now shows")
		require.Contains(t, raw, ".plan-tier-body .table th { background-color: transparent; }",
			f+": every tinted cell selector must be transparent")
		// The Bootstrap hover is a grey wash that reads as dirt on red.
		require.Contains(t, raw, ".plan-tier-body .table-hover tbody tr:hover > td",
			f+": the hover must preserve the tier tint")
		// The late group dot was amber — the same colour as "due now".
		require.Contains(t, raw, ".plan-dot-late { color: #dc3545; }",
			f+": a late group dot must be red, not the amber of the due-now tier")
		require.NotContains(t, raw, ".plan-dot-late { color: #e0a800; }",
			f+": the amber late dot must be gone")
	}
}

// TestGroupCheckIsCentredInItsRow: it was top-aligned in a cell that grows with
// the animal list, so on a cage with many animals it read as belonging to the
// first animal rather than to the group.
func TestGroupCheckIsCentredInItsRow(t *testing.T) {
	css := readTemplate(t, "../assets/css/care-plan.scss")
	require.Contains(t, css, ".plan-feed-check",
		"the group check cell must have a rule that centres it")
	require.Contains(t, css, "vertical-align: middle !important;",
		"a `text-nowrap` cell must be told explicitly to centre; the default is top")

	// Phase 0b moved the feeding table into the _plan_tier_feed_table partial.
	for _, f := range []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	} {
		require.Contains(t, readTemplate(t, f), `class="text-nowrap plan-feed-check"`,
			f+": the group check cell must carry the centring class")
		// Exactly one cell: the per-animal chip cells are a different control,
		// and a second user of the class would silently re-centre it.
		require.Equal(t, 1,
			strings.Count(readTemplate(t, f), `class="text-nowrap plan-feed-check"`),
			f+": exactly one cell may carry the centring class")
	}
}
