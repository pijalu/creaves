package actions

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Phase 5 / D5: the animal page's #nav-plan non-medication rows render the
// SAME shared line component as the care-plan day plan (guideline §7) —
// ONE merged line per (source × animal) requirement, no animal cell (the
// page context carries the identity, §4.3), the kind badge kept (the
// per-day list mixes kinds), tier-coloured toggles with the immediate
// in-place flip (§2), the missing-occurrence pill and lock kept (R4-7.2),
// and the R8-2 `?src=` deep link untouched.

// phase5AnimalForks are the four locale forks of the animal page.
var phase5AnimalForks = []string{
	"../templates/animals/show.plush.html",
	"../templates/animals/show.plush.fr.html",
	"../templates/animals/show.plush.de.html",
	"../templates/animals/show.plush.nl.html",
}

// phase5LineForks are the four byte-identical forks of the shared line.
var phase5LineForks = []string{
	"../templates/care_plan/_plan_item_line.plush.html",
	"../templates/care_plan/_plan_item_line.plush.fr.html",
	"../templates/care_plan/_plan_item_line.plush.de.html",
	"../templates/care_plan/_plan_item_line.plush.nl.html",
}

// 5-T1: every non-medication item row of the animal Protocol tab delegates
// to the shared `_plan_item_line` component with NO animal cell
// (showAnimal: false — the identity is the page context) and the kind
// badge kept (showKind: true — the per-day list mixes kinds); the feeding
// rows keep the prefilled ration modal (feedingEntry: true, R4-2.2). The
// partial itself must gate the animal cell on the flag.
func TestPhase5AnimalRowsUseSharedLineNoAnimalCell(t *testing.T) {
	for _, f := range phase5AnimalForks {
		raw := readTemplate(t, f)
		require.Contains(t, raw,
			`partial("care_plan/plan_item_line.plush.html", {card: item, showAnimal: false, showKind: true, feedingEntry: true})`,
			f, "the animal page renders the shared line — no animal cell, kind badge kept")
		require.NotContains(t, raw, `showAnimal: true`, f,
			"the animal page never asks for the animal cell (identity is the page context)")
	}
	// The partial honours the flag: the animal cell renders only when asked.
	for _, f := range phase5LineForks {
		require.Contains(t, readTemplate(t, f), `<%= if (showAnimal) { %>`, f,
			"the animal cell is conditional — the animal page omits it")
	}
}

// 5-T2: the medication block is untouched — the shared `_med_series`
// component renders the per-day medication occurrences and the med toggle
// partial is still included. Phase 5 changes ONLY the non-medication rows.
func TestPhase5MedicationBlockUnchanged(t *testing.T) {
	for _, f := range phase5AnimalForks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `partial("care_plan/med_series.plush.html")`, f,
			"the medication rows keep the shared med series component")
		require.Contains(t, raw, `partial("care_plan/plan_med_toggle.plush.html")`, f,
			"the medication apply/undo implementation is unchanged")
	}
}

// 5-T3: the immediate in-place flip (§2) reaches the animal page through
// the shared apply implementation — the page includes the delegated
// `_apply_toggle` JS (pair swap via setToggleState, no reload), and the
// line component emits the plan-apply-btn controls it drives.
func TestPhase5ApplyFlipViaSharedToggle(t *testing.T) {
	for _, f := range phase5AnimalForks {
		require.Contains(t, readTemplate(t, f),
			`partial("care_plan/apply_toggle.plush.html")`, f,
			"the animal page shares ONE apply/undo implementation with the day plan")
	}
	for _, f := range phase5LineForks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, "plan-apply-btn", f,
			"the line emits the apply control the shared toggle flips in place")
		require.Contains(t, raw, "plan-unapply-btn", f,
			"the line emits the undo control (flip is reversible, §2.4)")
	}
}

// 5-T4a: server-side, repeats of one requirement merge into ONE line with
// one slot per open occurrence (§3.1) — the merged line's own tier is its
// most urgent slot's; terminal rows stand alone.
func TestPhase5MergeDayItemsMergesRepeats(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	mk := func(sourceID string, h int, status, tier string, tierNo int) CardView {
		return CardView{
			SourceType: "rule", SourceID: sourceID,
			ActionKind: "feeding", Detail: "grenouilles",
			Status:    status,
			DueAt:     time.Date(2026, 10, 2, h, 0, 0, 0, time.Local),
			Applicable: true,
			Tier:      tierNo,
			TierClass: tier,
			AnimalID:  7,
		}
	}
	items := []CardView{
		mk("s1", 8, "late", "btn-danger", 0),
		mk("s1", 10, "due", "btn-warning", 1),
		mk("s1", 20, "scheduled", "btn-outline-secondary", 3),
		mk("s2", 9, "due", "btn-warning", 1),     // another requirement — its own line
		mk("s1", 7, "applied", "btn-success", 4), // terminal — stands alone
	}
	got := mergeDayItems(items, now)

	// s1's three OPEN occurrences merge into one line; s2 is its own line;
	// the applied s1 occurrence is a standalone terminal row ⇒ 3 lines.
	require.Len(t, got, 3, "repeats merge per requirement; terminal rows stand alone")

	var merged *CardView
	standalone := 0
	for i := range got {
		if len(got[i].Slots) == 3 {
			merged = &got[i]
		} else {
			standalone++
		}
	}
	require.NotNil(t, merged, "the three open s1 occurrences merged into ONE line")
	require.Equal(t, 2, standalone, "the singleton s2 line and the terminal s1 row stand alone")
	require.Len(t, merged.Slots, 3, "one toggle per occurrence")
	// Slots are due-time ordered (08:00, 10:00, 20:00).
	require.True(t, merged.Slots[0].DueAt.Before(merged.Slots[1].DueAt))
	require.True(t, merged.Slots[1].DueAt.Before(merged.Slots[2].DueAt))
	// The line's tier is its MOST URGENT slot's (the late 08:00 one).
	require.Equal(t, 0, merged.Tier, "the representative re-tiers to the most urgent slot")
	require.Equal(t, "btn-danger", merged.TierClass)
	require.Equal(t, "late", merged.Status)
	require.Equal(t, 2, merged.Remaining, "the fold counts the other occurrences")
	require.True(t, merged.Applicable, "the merged line is actionable while any slot is")
}

// 5-T4b: an open occurrence outside its apply window is NOT actionable —
// its slot keeps Applicable=false so the shared line renders it locked
// (🔒, §10-A1) instead of as a dead button; the missing pill stays on the
// page (R4-7.2). The lock itself is pinned in markup.
func TestPhase5OutOfWindowLocksNotDeadButton(t *testing.T) {
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	far := CardView{
		SourceType: "rule", SourceID: "s9",
		ActionKind: "observation", Detail: "comportement",
		Status:     "scheduled",
		DueAt:      now.Add(48 * time.Hour), // tomorrow+ — outside the apply window
		Applicable: false,
		Tier:       3, TierClass: "btn-outline-secondary",
		AnimalID: 7,
	}
	got := mergeDayItems([]CardView{far}, now)
	require.Len(t, got, 1)
	require.Len(t, got[0].Slots, 1, "even a singleton open occurrence renders via a slot")
	require.False(t, got[0].Slots[0].Applicable, "the out-of-window slot is not actionable")
	require.False(t, got[0].Applicable, "the line itself is not actionable")

	// Markup: the lock glyph + localized out-of-window label, and the pill.
	for _, f := range phase5LineForks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, "plan-item-locked", f,
			"a locked occurrence shows the 🔒 lock, never a dead action")
		require.Contains(t, raw, `t("care_plan.status.out_of_window")`, f,
			"the lock carries the localized hors-délai label")
	}
	for _, f := range phase5AnimalForks {
		require.Contains(t, readTemplate(t, f),
			`t("care_plan.animal_plans.missing", day.MissingCount)`, f,
			"the missed-occurrences pill is kept (R4-7.2)")
	}
}

// 5-T5 (R8-2 regression): the `?src=<source-id>` deep link still opens the
// collapsed Details card and highlights the matching traceability row —
// Phase 5 rewrote the day list, not the trace table or its JS.
func TestPhase5SrcDeepLinkStillOpensDetails(t *testing.T) {
	for _, f := range phase5AnimalForks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `new URLSearchParams(window.location.search).get('src')`, f,
			"the deep link reads the src parameter")
		require.Contains(t, raw, `document.querySelector('#planDetails tr[data-source-id="' + src + '"]')`, f,
			"the link locates the traceability row for that source")
		require.Contains(t, raw, `$('#planDetails').collapse('show')`, f,
			"the Details card opens")
		require.Contains(t, raw, `row.classList.add('plan-trace-highlight')`, f,
			"the matching row is highlighted")
	}
}
