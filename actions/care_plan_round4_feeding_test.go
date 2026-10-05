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

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, "", now)
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

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, "", now)
	require.Len(t, v.Feedings, 1)
	fc := v.FeedTiers[0][0]
	require.Len(t, fc.Chips, 2)

	// Phase 0b: the feeding tier's table lives in the ONE
	// `_plan_tier_feed_table` partial (the index only passes locals); the
	// bordered panel + header live in the `_plan_tier` layout component.
	ctx := plush.NewContextWith(map[string]interface{}{
		"feedCards": v.FeedTiers[0],
		"ti":        0,
		"t":         func(s string) string { return s },
		"tbase": func(a, b, c string) string {
			return c
		},
	})
	tierCtx := plush.NewContextWith(map[string]interface{}{
		"tierKey":        "late",
		"tierId":         "feed-tier-",
		"tierIdNum":      0,
		"tierLabelKey":   "care_plan.tier.late",
		"tierBadgeClass": "badge-secondary",
		"tierCount":      "2",
		"tierExpanded":   true,
		"tierDoneCount":  0,
		"yield":          "",
		"t":              func(s string) string { return s },
	})
	forks := []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		out, err := plush.Render(string(raw), ctx)
		require.NoError(t, err, f)

		tierRaw, err := os.ReadFile(tierPartialFor(f))
		require.NoError(t, err, f)
		tierOut, err := plush.Render(string(tierRaw), tierCtx)
		require.NoError(t, err, f)
		require.Contains(t, tierOut, `id="feed-tier-0"`, f, "feeding urgency sections")
		require.Contains(t, out, "plan-dot-late", f, "one group dot, most urgent chip")
		require.Equal(t, 1, strings.Count(out, `class="plan-dot plan-dot-`), f,
			"exactly ONE dot per cage × diet card")
		require.Contains(t, out, `class="badge badge-pill badge-secondary plan-apply-count">2<`, f,
			"count is a corner overlay badge, rendered at N=2 (darker badge: R4-7.5)")
		require.Contains(t, out, `class="sr-only"`, f, "status stays available to assistive tech")
		require.Contains(t, out, `aria-label="care_plan.card.apply_group (2)"`, f, "count in the a11y label")
		// R4-7.6 asked the chip to show its expected time; R4-7.14b moved
		// that time ONTO the sub-group, because six animals fed at 08:00
		// printed 08:00 six times. The time is still on screen, and it is
		// still the same time — stated once per sub-group, not per animal.
		require.Equal(t, 2, strings.Count(out, `class="plan-time-label"`), f,
			"R4-7.14b: one time label per sub-group (08:00 and 10:15 here)")
		require.Contains(t, out, "care_plan.feeding.earliest 08:00", f,
			"R4-7.14c: the collapsed header states the earliest time")
		require.NotContains(t, out, "plan-chip-due", f,
			"R4-7.14b: no animal repeats a time the sub-group already states")
		require.Contains(t, out, "plan-feeding-one", f,
			"R4-7.5: with several animals the per-animal check stays")
		require.NotContains(t, out, "plan-apply-space", f,
			"no spacer needed when the per-animal check renders")

		// R4-7.11: no apply button wears the "done" green.
		require.NotContains(t, buttonHTML(out, "plan-feeding-apply"), "btn-success", f,
			"a green apply button reads as 'already done'")
		require.NotContains(t, buttonHTML(out, "plan-feeding-one"), "btn-success", f,
			"a green per-animal check reads as 'already done'")

		// The count badge must NOT be a child of the apply button.
		btn := buttonHTML(out, "plan-feeding-apply")
		require.NotEmpty(t, btn, f)
		require.NotContains(t, btn, "badge", f, "no count pill inside the button (R4-4.2)")
	}
}

// TestFeedingSingleAnimalGroupOnly pins R4-7.5: a cage with ONE animal shows
// only the group check — no per-animal check, no counter pill — and the row
// keeps its footprint through the invisible spacer, so the table's columns
// stay aligned whether or not the per-animal check is present.
func TestFeedingSingleAnimalGroupOnly(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now
	feed := testSource(careplan.KindFeeding, "feed-1", "Feed", map[string]interface{}{"food": "grenouilles"})

	a := testItem(feed, 1, careplan.StatusDue)
	a.Occurrence.DueAt = time.Date(2026, 9, 28, 10, 15, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{a}

	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, "", now)
	require.Len(t, v.Feedings, 1)
	require.Len(t, v.FeedTiers[1][0].Chips, 1, "the lone due occurrence sits in the 'now' tier")

	// Phase 0b: the feeding table markup lives in the ONE
	// `_plan_tier_feed_table` partial; the index only passes locals.
	ctx := plush.NewContextWith(map[string]interface{}{
		"feedCards": v.FeedTiers[1],
		"ti":        1,
		"t":         func(s string) string { return s },
		"tbase":     func(a, b, c string) string { return c },
	})
	forks := []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		out, err := plush.Render(string(raw), ctx)
		require.NoError(t, err, f)

		// Scope to the feeding table: the page's batch JS legitimately
		// names the per-animal class it flips.
		tier := out[:strings.Index(out, "</table>")]

		require.Contains(t, tier, "plan-feeding-apply", f, "the GROUP check stays")
		require.NotContains(t, tier, "plan-feeding-one", f,
			"a lone animal has no per-animal check (R4-7.5)")
		require.Contains(t, tier, `<span class="plan-apply-space" aria-hidden="true"></span>`, f,
			"the spacer keeps the row footprint — and the columns — aligned")
		require.NotContains(t, tier, "plan-apply-count", f,
			"a counter of 1 is noise, not information (R4-7.5)")
		require.Contains(t, out, `aria-label="care_plan.card.apply_group (1)"`, f,
			"the count still reaches assistive tech through the aria-label")
	}
}

// tierPartialFor maps a forked body-partial path to the matching fork of the
// `_plan_tier` layout component (same locale suffix).
func tierPartialFor(bodyFork string) string {
	return strings.Replace(bodyFork, "_plan_tier_feed_table", "_plan_tier", 1)
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

// TestAutoRefreshVisibilityFloor (R4-4.1): the auto-refresh may not wipe
// a change the caregiver just made — every action records the time in the
// SHARED apply partial and the refresh defers itself while the page is
// younger than the 30 s floor.
func TestAutoRefreshVisibilityFloor(t *testing.T) {
	partials := []string{
		"../templates/care_plan/_apply_toggle.plush.html",
		"../templates/care_plan/_apply_toggle.plush.de.html",
		"../templates/care_plan/_apply_toggle.plush.fr.html",
		"../templates/care_plan/_apply_toggle.plush.nl.html",
	}
	for _, f := range partials {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		s := string(raw)
		require.Contains(t, s, "ACTION_VISIBILITY_MS = 30000", f, "30 s visibility floor")
		require.Contains(t, s, "window.planMarkAction = markAction", f, "shared hook")
		require.Contains(t, s, "window.planActionAge", f, "the refresh guard can read the action age")
		for _, fn := range []string{"markApplied", "markOpen", "instantApply", "instantUnapply"} {
			i := strings.Index(s, "function "+fn+"(")
			require.Positive(t, i, f+" declares "+fn)
			require.Contains(t, s[i:i+400], "markAction()", f+": "+fn+" records the action time")
		}
	}

	pages := []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/index.plush.de.html",
		"../templates/care_plan/index.plush.fr.html",
		"../templates/care_plan/index.plush.nl.html",
	}
	for _, f := range pages {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		s := string(raw)
		require.Contains(t, s, `partial("care_plan/apply_toggle.plush.html")`, f, "one shared apply implementation")
		require.Contains(t, s, "window.planActionAge() < 30000", f, "the reload waits for the floor")
	}

	toggles := []string{
		"../templates/care_plan/_plan_med_toggle.plush.html",
		"../templates/care_plan/_plan_med_toggle.plush.de.html",
		"../templates/care_plan/_plan_med_toggle.plush.fr.html",
		"../templates/care_plan/_plan_med_toggle.plush.nl.html",
	}
	for _, f := range toggles {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		require.Contains(t, string(raw), "window.planMarkAction()", f, "slot flips hold the refresh off")
	}
}

// TestApplyToggleSharedByBothPages (R4-2.2): ONE apply implementation —
// the day plan and the animal tabs include the same partial, so a feeding
// or observation row is recorded the same way on both.
func TestApplyToggleSharedByBothPages(t *testing.T) {
	forks := []string{
		"../templates/care_plan/_apply_toggle.plush.html",
		"../templates/care_plan/_apply_toggle.plush.de.html",
		"../templates/care_plan/_apply_toggle.plush.fr.html",
		"../templates/care_plan/_apply_toggle.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		s := string(raw)
		require.Contains(t, s, "plan-feeding-entry", f, "feeding entries open the prefilled modal")
		require.Contains(t, s, "closest('.plan-apply-btn, .plan-feeding-one')", f, "delegated apply binding")
		require.Contains(t, s, "closest('.plan-unapply-btn')", f, "delegated undo binding")
	}
	for _, f := range []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	} {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		s := string(raw)
		require.Contains(t, s, `partial("care_plan/apply_toggle.plush.html")`, f, "animal tabs use the shared apply")
		require.Contains(t, s, "plan-apply-btn", f, "animal item rows are actionable")
		require.Contains(t, s, "plan-feeding-entry", f, "feeding rows open the prefilled entry")
	}
}
