//go:build !sqlite

package actions

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"creaves/models/careplan"
)

// R4-7.21: one unit, one number, everywhere. The summary strip used to speak
// a DIFFERENT unit from the tier section it pointed at, which is how a page
// ended up showing "Late 99+" and "Later 99+" above a feeding page whose own
// section header counted cage×diet GROUPS (162) rather than the occurrences
// (218) the apply buttons act on.
func TestFeedingTierBadgeCountsOccurrencesNotGroups(t *testing.T) {
	v := feedingView(1, 3) // one group, three animals
	fillFeedTiers(v)

	require.Len(t, v.FeedTiers[0], 1, "one group in the late tier")
	require.Equal(t, 1, v.FeedTierCount[0], "the group count is still available")
	require.Equal(t, 3, v.FeedTierOpen[0], "the BADGE counts the three occurrences, not the one group")
	require.Equal(t, BadgeCap(3), v.FeedTierOpenCap[0])
}

// A group with no open work contributes no occurrence to the badge, so the
// badge can never count work the apply buttons would not record.
func TestFeedingTierBadgeIgnoresGroupsWithNoOpenWork(t *testing.T) {
	v := feedingView(1, 0) // one group, nothing applicable
	fillFeedTiers(v)

	require.Len(t, v.FeedTiers[0], 1)
	require.Equal(t, 1, v.FeedTierCount[0])
	require.Zero(t, v.FeedTierOpen[0])
}

// Every strip pill must resolve to a section that EXISTS and must repeat that
// section's own number. The strip previously counted the workload across days
// (view.Stats) while the sections are today-scoped — "Later 83" above a
// section badged 31 — and its anchors were hardcoded #tier-late / #tier-now
// / #tier-later, which the feeding page never renders (it uses #feed-tier-N),
// so both links were dead there.
func TestSummaryStripOnlyLinksSectionsThatExist(t *testing.T) {
	for _, kind := range []string{
		careplan.KindFeeding, careplan.KindMedication,
		careplan.KindObservation, careplan.KindCare, careplan.KindCleanup,
	} {
		t.Run(kind, func(t *testing.T) {
			v := feedingView(1, 3)
			fillFeedTiers(v)
			v.Kind = kind
			fillTierHasWork(v, kind)

			for _, link := range v.TierLinks {
				idx := tierIndex(link.Key)
				wantID := "tier-" + link.Key
				if kind == careplan.KindFeeding || kind == careplan.KindCleanup {
					wantID = "feed-tier-" + fmt.Sprint(idx)
				}
				require.Equal(t, wantID, link.ID,
					"the pill must point at the section that actually renders")
				require.True(t, v.TierHasWork[idx],
					"strip shows %q but that tier renders nothing", link.Key)
				require.Equal(t, BadgeCap(link.Num), link.Cap)
			}

		})
	}
}

// A tier with nothing in it must produce no section and no pill — the page
// used to render an empty "Now" header badged "0".
func TestTierHasWorkIsFalseForAnEmptyTier(t *testing.T) {
	v := feedingView(1, 3)
	fillFeedTiers(v)
	v.Kind = careplan.KindMedication
	fillTierHasWork(v, careplan.KindMedication)

	// Under kind=medication every tier reads MedTiers, which this feeding-only
	// plan never filled — so no section and no pill, which is the whole point.
	for i := 0; i < 3; i++ {
		require.False(t, v.TierHasWork[i], "tier %d has no medication work", i)
	}
	require.Empty(t, v.TierLinks, "nothing renders, so the strip is empty")

	// The same feeding plan under kind=feeding DOES have late work, and it
	// must surface as a live pill.
	v.Kind = careplan.KindFeeding
	fillTierHasWork(v, careplan.KindFeeding)
	require.True(t, v.TierHasWork[0], "the late feeding group is real work")
	require.Len(t, v.TierLinks, 1)
	require.Equal(t, "feed-tier-0", v.TierLinks[0].ID)
}

func tierIndex(key string) int {
	switch key {
	case TierLate:
		return 0
	case TierNow:
		return 1
	default:
		return 2
	}
}

// feedingView builds a DayPlanView with `groups` feeding groups of `perGroup`
// late, applicable chips each, and the given ApplicableCount (what the apply
// button would actually record).
func feedingView(groups, applicable int) *DayPlanView {
	v := &DayPlanView{}
	for g := 0; g < groups; g++ {
		chips := make([]FeedingChip, 0, perGroupOf(applicable))
		for i := 0; i < perGroupOf(applicable); i++ {
			chips = append(chips, feedChip(8+i, 0))
		}
		v.Feedings = append(v.Feedings, FeedingGroupView{
			Cage:            "C1",
			Food:            "pellets",
			Chips:           chips,
			ApplicableCount: applicable,
		})
	}
	return v
}

func perGroupOf(applicable int) int {
	if applicable < 1 {
		return 1
	}
	return applicable
}
