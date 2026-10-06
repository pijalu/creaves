//go:build !sqlite
// +build !sqlite

package actions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Goal 3 layout contract: width comes from each table's own content; feeding
// tier shortcuts live beside feeding tiers and collapse only that tab's tiers.
func TestGoal3LayoutAndFeedingTierNavigationAcrossLocales(t *testing.T) {
	for _, locale := range []string{"", ".fr", ".de", ".nl"} {
		index := readTemplate(t, "../templates/care_plan/index.plush"+locale+".html")
		require.Contains(t, index, `id="feedingTabContent"`, locale)
		require.Contains(t, index, `id="planSummaryStrip"`, locale)
		require.Contains(t, index, `feedingPanel.contains(link)`, locale)
		require.Contains(t, index, `scope.querySelectorAll('.plan-tier')`, locale)
		require.Contains(t, index, `.plan-tier-body table { table-layout: auto; max-width: 100%; }`, locale)
		// Bug 2026-10-06 #1: `width: max-content` on the animal cell made
		// Chrome resolve the column against the TABLE's one-line width —
		// the column blew up and the tier panel clipped the Apply buttons
		// out of view. The RULE must stay gone in every locale (comments
		// explaining the removal may mention it).
		require.NotContains(t, index, `td.plan-med-animal { width: max-content`, locale)
		require.Contains(t, index, `event.target.closest('a[href^="#"]')`, locale)
		require.NotContains(t, index, `.plan-feed-animal, .plan-care-animal { width: 1%`, locale)
		require.NotContains(t, index, `.plan-med-animal, .plan-feed-animal, .plan-care-animal, .plan-loc-col { min-width: 11rem`, locale)

		// Conditional rendering leaves exactly one strip on each active view:
		// non-feeding placement above tabs, feeding placement inside its wrapper.
		require.Equal(t, 2, strings.Count(index, `id="planSummaryStrip"`), locale)
		require.Contains(t, index, `if (view.Kind != "feeding")`, locale)
	}

	care := readTemplate(t, "../templates/care_plan/_plan_care_line.plush.html")
	require.Contains(t, care, `class="plan-med-animal plan-care-animal"`)
	med := readTemplate(t, "../templates/care_plan/_plan_med_row.plush.html")
	require.Contains(t, med, `class="mr-2 py-1 plan-med-animal"`)
}
