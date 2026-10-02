package actions

import (
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

// Round-4 R4-4.2/4.3/4.4 tests: the feeding cards become urgency tiers
// (late → now → later) as the FIRST level, each cage × diet card carries
// ONE group status dot (the most urgent of its chips), and the apply
// count is a corner overlay so the button never changes width.

// TestFeedingGroupTierAndStatus (R4-4.4): a group exposes the status of
// its most urgent NON-superseded chip — identical cage × diet therefore
// always reads identical, whatever the individual occurrence timing is.
func TestFeedingGroupTierAndStatus(t *testing.T) {
	cases := []struct {
		name       string
		chips      []FeedingChip
		wantTier   int
		wantStatus string
	}{
		{
			name: "one late chip makes the group late",
			chips: []FeedingChip{
				{Status: "due"}, {Status: "late"},
			},
			wantTier: 0, wantStatus: "late",
		},
		{
			name: "due beats scheduled",
			chips: []FeedingChip{
				{Status: "scheduled"}, {Status: "due"},
			},
			wantTier: 1, wantStatus: "due",
		},
		{
			name: "only future chips = later tier",
			chips: []FeedingChip{
				{Status: "scheduled", Superseded: true},
				{Status: "scheduled"},
			},
			wantTier: 2, wantStatus: "scheduled",
		},
		{
			name: "superseded chips never set the group state",
			chips: []FeedingChip{
				{Status: "late", Superseded: true},
			},
			wantTier: -1, wantStatus: "",
		},
	}
	for _, c := range cases {
		tier, status := feedingGroupTier(FeedingGroupView{Chips: c.chips})
		require.Equal(t, c.wantTier, tier, c.name)
		require.Equal(t, c.wantStatus, status, c.name)
	}
}

// TestFeedTiersFirstLevel (R4-4.3): the feeding cards are distributed over
// the late / now / later tiers — the same language as medication — with
// the zone/cage sort kept inside each tier.
func TestFeedTiersFirstLevel(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now
	lateFeed := testSource(careplan.KindFeeding, "feed-late", "Feed late", map[string]interface{}{"food": "grenouilles"})
	nowFeed := testSource(careplan.KindFeeding, "feed-now", "Feed now", map[string]interface{}{"food": "poussins"})

	l := testItem(lateFeed, 1, careplan.StatusLate)
	l.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	d := testItem(nowFeed, 1, careplan.StatusDue)
	d.Occurrence.DueAt = time.Date(2026, 9, 28, 10, 15, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{l, d}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	require.Len(t, v.Feedings, 2, "flat set unchanged (nav + filters)")
	require.Len(t, v.FeedTiers[0], 1, "late feeding card")
	require.Len(t, v.FeedTiers[1], 1, "due-now feeding card")
	require.Len(t, v.FeedTiers[2], 0)
	require.Equal(t, TierLate, v.FeedTiers[0][0].TierKey)
	require.Equal(t, TierNow, v.FeedTiers[1][0].TierKey)
	require.Equal(t, "1", v.FeedTierCap[0])
	require.Equal(t, "late", v.FeedTiers[0][0].GroupStatus)
	require.Equal(t, "plan-dot-late", v.FeedTiers[0][0].GroupStatusClass)
	require.Equal(t, "plan-dot-due", v.FeedTiers[1][0].GroupStatusClass)
}

// TestFeedingSectionRender (R4-4.2/4.3/4.4, all four locales): the feeding
// section renders ONE dot per cage × diet (not per animal), no redundant
// visible status pill, and the count as a corner overlay OUTSIDE the
// apply button (so the button width is stable and N=1 still shows).
func TestFeedingSectionRender(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now
	feed := testSource(careplan.KindFeeding, "feed-1", "Feed", map[string]interface{}{"food": "grenouilles"})

	a := testItem(feed, 1, careplan.StatusLate)
	a.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	b := testItem(feed, 2, careplan.StatusDue)
	b.Occurrence.DueAt = time.Date(2026, 9, 28, 10, 15, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{a, b}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	require.Len(t, v.Feedings, 1)
	fc := v.FeedTiers[0][0]
	require.Len(t, fc.Chips, 2)

	ctx := plush.NewContextWith(map[string]interface{}{
		"view": v,
		"t":    func(s string) string { return s },
		"tbase": func(a, b, c string) string {
			return c
		},
		"dueLabel": func(a, b, c string) string { return a },
		// the index page includes JS/HTML partials this unit render does
		// not exercise; stub them so the section markup is testable alone.
		"partial": func(name string, ctx map[string]interface{}) (string, error) { return "", nil },
		"partialIf": func(name string, ctx map[string]interface{}) (string, error) { return "", nil },
	})
	forks := []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/index.plush.de.html",
		"../templates/care_plan/index.plush.fr.html",
		"../templates/care_plan/index.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		out, err := plush.Render(string(raw), ctx)
		require.NoError(t, err, f)

		require.Contains(t, out, `id="feed-tier-0"`, f, "feeding urgency sections")
		require.Contains(t, out, "plan-dot-late", f, "one group dot, most urgent chip")
		require.Equal(t, 1, strings.Count(out, `class="plan-dot plan-dot-`), f,
			"exactly ONE dot per cage × diet card")
		require.Contains(t, out, `class="badge badge-pill badge-light plan-apply-count">2<`, f,
			"count is a corner overlay badge, rendered at N=2")
		require.Contains(t, out, `class="sr-only"`, f, "status stays available to assistive tech")
		require.Contains(t, out, `aria-label="care_plan.card.apply_group (2)"`, f, "count in the a11y label")

		// The count badge must NOT be a child of the apply button.
		btn := buttonHTML(out, "plan-feeding-apply")
		require.NotEmpty(t, btn, f)
		require.NotContains(t, btn, "badge", f, "no count pill inside the button (R4-4.2)")
	}
}

// buttonHTML returns the opening HTML of the first button whose class list
// contains `class`.
func buttonHTML(html, class string) string {
	i := strings.Index(html, class)
	if i < 0 {
		return ""
	}
	start := strings.LastIndex(html[:i], "<button")
	if start < 0 {
		return ""
	}
	end := strings.Index(html[start:], ">")
	if end < 0 {
		return ""
	}
	return html[start : start+end+1]
}