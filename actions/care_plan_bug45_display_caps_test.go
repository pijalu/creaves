package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// BUG-45: applyDisplayCaps folds each tier's overflow into the Capped*
// counts, keeping the EARLIEST occurrences visible; 0 = uncapped; stale
// slots are exempt (already folded behind the stale badge). The caps are
// purely presentational — Slots trimming never touches scheduling/apply.
func TestApplyDisplayCaps(t *testing.T) {
	day := time.Date(2026, 11, 13, 0, 0, 0, 0, time.Local)
	slot := func(h, tier int) ItemSlotView {
		return ItemSlotView{DueAt: day.Add(time.Duration(h) * time.Hour), Tier: tier}
	}

	// Uncapped: untouched, no counts.
	cv := CardView{Slots: []ItemSlotView{slot(8, 0), slot(9, 1), slot(10, 2)}}
	applyDisplayCaps(&cv)
	require.Len(t, cv.Slots, 3)
	require.Zero(t, cv.CappedLateCount)
	require.Zero(t, cv.CappedNowCount)
	require.Zero(t, cv.CappedLaterCount)

	// Per-tier caps: each tier keeps its earliest N, overflow folds.
	cv = CardView{
		DisplayMaxLate:   1,
		DisplayMaxNow:    2,
		DisplayMaxFuture: 1,
		Slots: []ItemSlotView{
			slot(6, 0), slot(7, 0), // late ×2 → keep 1, cap 1
			slot(9, 1), slot(10, 1), slot(11, 1), // now ×3 → keep 2, cap 1
			slot(13, 2), slot(14, 2), slot(15, 2), slot(16, 2), // later ×4 → keep 1, cap 3
		},
	}
	applyDisplayCaps(&cv)
	require.Len(t, cv.Slots, 4)
	require.Equal(t, 1, cv.CappedLateCount)
	require.Equal(t, 1, cv.CappedNowCount)
	require.Equal(t, 3, cv.CappedLaterCount)
	// Kept: earliest per tier.
	require.Equal(t, day.Add(6*time.Hour), cv.Slots[0].DueAt)
	require.Equal(t, day.Add(9*time.Hour), cv.Slots[1].DueAt)
	require.Equal(t, day.Add(10*time.Hour), cv.Slots[2].DueAt)
	require.Equal(t, day.Add(13*time.Hour), cv.Slots[3].DueAt)

	// Stale slots stay (exempt from the cap bookkeeping).
	cv = CardView{
		DisplayMaxNow: 1,
		Slots: []ItemSlotView{
			{DueAt: day.Add(5 * time.Hour), Tier: 0, Stale: true},
			slot(9, 1), slot(10, 1),
		},
	}
	applyDisplayCaps(&cv)
	require.Len(t, cv.Slots, 2) // stale + 1 kept now slot
	require.True(t, cv.Slots[0].Stale)
	require.Equal(t, 1, cv.CappedNowCount)

	// Cap above the tier size: no fold, slots slice untouched.
	cv = CardView{DisplayMaxNow: 5, Slots: []ItemSlotView{slot(9, 1), slot(10, 1)}}
	applyDisplayCaps(&cv)
	require.Len(t, cv.Slots, 2)
	require.Zero(t, cv.CappedNowCount)
}
