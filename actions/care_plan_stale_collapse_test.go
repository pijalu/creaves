package actions

import (
	"fmt"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// Item 8 follow-up (2026-10-07): the "⏱ N late" fold badge becomes a
// COLLAPSIBLE — the folded occurrences' toggles render hidden behind it and
// the badge reveals them again, so the caregiver can still record each late
// occurrence one by one (⏸ snooze keeps deferring them all at once).
//
// Two render strategies, one contract:
//   - medication: the folded MedSlotViews leave Rows (the actionable line)
//     and ride along in MedSeriesView.StaleSlots — `_med_series` renders
//     them hidden beside the badge;
//   - row kinds: the stale ItemSlotViews STAY in CardView.Slots flagged
//     Stale — the normal slot loop hides them (plan-stale-slot d-none);
//   - care views / feeding chips kept their slots too (they were never
//     dropped — the templates skipped them; they now render them hidden).
//
// The JS handler (.plan-stale-toggle → toggle .plan-stale-slot within
// .plan-stale-scope) lives in `_apply_toggle`, delegated like every other
// binding.

func TestScopeSeriesKeepsStaleSlotsForTheCollapsible(t *testing.T) {
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)
	now := day.Add(22 * time.Hour) // 22:00
	tom := day.AddDate(0, 0, 1)

	series := r47Series("Amox",
		r47LateSlot(now.Add(-20*time.Hour), 0, careplan.StatusLate), // stale: 20 h old, next is 10 h away
		r47Slot(tom.Add(8*time.Hour), 0, careplan.StatusScheduled),
	)

	got := scopeSeriesToToday(series, 1, now)

	require.Equal(t, 1, got.StaleLateCount)
	require.Len(t, got.StaleSlots, 1, "the folded slot rides along for the badge's collapsible")
	require.Equal(t, now.Add(-20*time.Hour), got.StaleSlots[0].DueAt, "the stale slot keeps its due time (its toggle re-renders)")
	require.True(t, got.StaleSlots[0].Applicable, "the folded slot is the actionable shape — its toggle works when revealed")
	require.NotContains(t, r47SlotTimes(got), now.Add(-20*time.Hour),
		"the folded slot stays OUT of Rows — the actionable line shows the next upcoming action only")
	require.NotEmpty(t, got.StaleRefsJSON, "the snooze ref survives")
}

// TestStaleCollapsibleTemplates: every locale fork carries the SAME wiring
// (the forks are byte-identical structural partials, pinned here so a fork
// edit can never silently drop the collapsible).
func TestStaleCollapsibleTemplates(t *testing.T) {
	families := []string{
		"care_plan/_apply_toggle.plush%s.html",
		"care_plan/_med_series.plush%s.html",
		"care_plan/_plan_care_line.plush%s.html",
		"care_plan/_plan_item_line.plush%s.html",
		"care_plan/_plan_tier_feed_table.plush%s.html",
	}
	for _, fam := range families {
		for _, lang := range []string{"", ".de", ".fr", ".nl"} {
			raw := readTemplate(t, "../templates/"+fmt.Sprintf(fam, lang))

			switch fam {
			case "care_plan/_apply_toggle.plush%s.html":
				require.Contains(t, raw, ".plan-stale-toggle", lang+": the collapse handler is bound")
				require.Contains(t, raw, "plan-stale-scope", lang+": the handler scopes to the record")
				require.Contains(t, raw, "offsetParent === null", lang+": the bucket-fit pass skips hidden cells")
			case "care_plan/_med_series.plush%s.html":
				require.Contains(t, raw, "plan-stale-scope", lang+": the series line is the collapsible's scope")
				require.Contains(t, raw, "series.StaleSlots", lang+": the folded med slots render (hidden)")
				require.Contains(t, raw, "plan-stale-slot d-none", lang+": folded med toggles start hidden")
			case "care_plan/_plan_item_line.plush%s.html":
				require.Contains(t, raw, "plan-stale-scope", lang+": the record is the collapsible's scope")
				require.Contains(t, raw, `if (slot.Stale) { %> plan-stale-slot d-none`, lang+": folded row-kind toggles start hidden")
			case "care_plan/_plan_care_line.plush%s.html":
				require.Contains(t, raw, "plan-care-slots plan-stale-scope", lang+": the row's td is the scope")
				require.Contains(t, raw, "plan-animal-row d-flex flex-wrap align-items-center py-1 plan-stale-scope", lang+": per-animal lines are scopes too")
				require.Contains(t, raw, "plan-stale-slot d-none", lang+": folded care toggles start hidden")
			case "care_plan/_plan_tier_feed_table.plush%s.html":
				require.Contains(t, raw, "plan-animal-row d-flex justify-content-between align-items-center py-1 plan-stale-scope", lang+": the chip row is the collapsible's scope")
				require.Contains(t, raw, "plan-stale-slot d-none", lang+": the stale chip's toggle renders hidden")
			}
		}
	}
}

// TestStaleBadgeIsACollapsibleToggle: the badge partial is a BUTTON with an
// aria-expanded state and a turning caret — not a dead pill.
func TestStaleBadgeIsACollapsibleToggle(t *testing.T) {
	raw := readTemplate(t, "../templates/care_plan/_plan_stale_badge.plush.html")

	require.Contains(t, raw, `class="plan-stale-late plan-stale-toggle"`, "the badge is the collapsible's toggle")
	require.Contains(t, raw, `type="button"`, "it is a real button (keyboard operable)")
	require.Contains(t, raw, `aria-expanded="false"`, "the collapsed state is announced")
	require.Contains(t, raw, "plan-stale-caret", "the caret signals expandability")
	require.Contains(t, raw, `title="<%= t("care_plan.late.stale_hint") %>"`, "the localized hint stays on the badge")
	require.Contains(t, raw, "plan-snooze-btn", "the snooze stays beside the badge")
}

// TestStaleCollapsibleStylesheet: the caret turns when the collapsible is
// open, the badge reads as pressable, and the animal cell's tint follows the
// ROW STATE (light green when done, light red when late, light yellow when
// due now — the neutral grey is the no-state fallback).
func TestStaleCollapsibleStylesheet(t *testing.T) {
	raw := readTemplate(t, "../assets/css/care-plan.scss")

	require.Contains(t, raw, ".plan-stale-toggle[aria-expanded=\"true\"] .plan-stale-caret", "the caret rotates when open")
	require.Contains(t, raw, "cursor: pointer;", "the badge reads as pressable")

	base := ruleBody(t, raw, ".plan-med-animal {")
	require.Contains(t, base, "background-color: #f8f9fa;", "no-state cells are neutral grey — never late red")

	late := ruleBody(t, raw, ".plan-tier-late .plan-med-animal,\ndiv.plan-med-row.plan-item-late .plan-med-animal,\ndiv.plan-med-row.plan-item-missing .plan-med-animal {")
	require.Contains(t, late, "background-color: #fdf2f3;", "late rows keep the historical pink cell")

	done := ruleBody(t, raw, ".plan-tier-done .plan-med-animal,\ndiv.plan-med-row.plan-item-applied .plan-med-animal {")
	require.Contains(t, done, "background-color: #d4edda;", "done rows read light green — the ✓ family")

	nowTier := ruleBody(t, raw, ".plan-tier-now .plan-med-animal,\ndiv.plan-med-row.plan-item-due .plan-med-animal {")
	require.Contains(t, nowTier, "background-color: #fff9e6;", "due-now rows read light yellow")
}
