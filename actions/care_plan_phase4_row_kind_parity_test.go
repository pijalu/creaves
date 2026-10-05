package actions

// Phase 4 / D4 — row-kind toggle line parity (docs/care-presentation-guideline.md
// R1/R2, §2/§3/§5). The observation/care/weighing rows now render like the
// medication series: ONE merged line per (source × animal) carrying one
// tier-coloured toggle per open occurrence, an immediate in-place colour
// flip on apply/undo, undo restoring the CURRENT tier colour, and the
// Phase-1 responsive ladder (year-number animal cell → wrap → full stack).
// Input kinds keep the apply modal per toggle.
//
// Test map (planning doc Phase 4):
//   4-T1 late observation → btn-danger toggle on a late (red) row.
//   4-T2 apply → the visible undo sibling flips to btn-success IMMEDIATELY
//        (static pin of the setToggleState pair-row recolour; the modal
//        flow for input kinds is pinned by 4-T6).
//   4-T3 undo → the visible apply sibling restores its data-tier-class.
//   4-T4 a 3×/day protocol renders ONE line with 3 toggles (slots sorted,
//        Remaining = 2).
//   4-T5 the row line walks the narrow-viewport ladder (scss pins).
//   4-T6 weighing/observation toggles keep the input modal (data-kind +
//        data-detail on every slot button; isInputKind dispatches to
//        openApply BEFORE instantApply).

import (
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// phase4Forks lists every template fork that carries the Phase-4 markup/JS
// (the row-kind line partial and the shared toggle routine).
var phase4ItemLineForks = []string{
	"../templates/care_plan/_plan_item_line.plush.html",
	"../templates/care_plan/_plan_item_line.plush.fr.html",
	"../templates/care_plan/_plan_item_line.plush.de.html",
	"../templates/care_plan/_plan_item_line.plush.nl.html",
}

var phase4ApplyToggleForks = []string{
	"../templates/care_plan/_apply_toggle.plush.html",
	"../templates/care_plan/_apply_toggle.plush.fr.html",
	"../templates/care_plan/_apply_toggle.plush.de.html",
	"../templates/care_plan/_apply_toggle.plush.nl.html",
}

// phase4PlanItems wraps testItem with a fixed wall-clock base so the merged
// slot order is deterministic.
func phase4Item(src careplan.PlanSource, animalID int, status careplan.PlanStatus, due time.Time) careplan.PlanItem {
	it := testItem(src, animalID, status)
	it.Occurrence.DueAt = due
	return it
}

// TestPhase4LateObservationToggleIsRed (4-T1): a late observation renders
// on the late tier with a btn-danger toggle — the line's own colour comes
// from its most urgent slot.
func TestPhase4LateObservationToggleIsRed(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	obs := testSource(careplan.KindObservation, "obs-late", "Observe wound", map[string]interface{}{"question": "Q?"})
	items := []careplan.PlanItem{
		phase4Item(obs, 1, careplan.StatusLate, now.Add(-2*time.Hour)),
	}
	plan.Items = items

	v := BuildDayPlanView(plan, ViewCompact, "", "observation", now)
	require.NotEmpty(t, v.Tiers[0].Cards, "the late observation must land on the late tier")
	card := v.Tiers[0].Cards[0]
	require.Equal(t, 0, card.Tier, "line tier = most urgent slot (late)")
	require.Equal(t, "btn-danger", card.TierClass, "line colour = slotTierClass(late)")
	require.Len(t, card.Slots, 1)
	require.Equal(t, "btn-danger", card.Slots[0].TierClass,
		"§2.1: the toggle itself carries the tier colour")
	require.Equal(t, 0, card.Slots[0].Tier)
	require.True(t, card.Slots[0].LateAllowed,
		"past-due + applicable → the late-record affordance stays available")
	require.True(t, card.Slots[0].NeedsInput, "observation keeps the input modal")

	// Sanity: the OTHER tiers stay empty for this fixture.
	require.Empty(t, v.Tiers[1].Cards)
	require.Empty(t, v.Tiers[2].Cards)
}

// TestPhase4MergedLineColourFollowsMostUrgentSlot: when a merged group spans
// tiers, the line sits on the most urgent slot's tier with that slot's
// colour — even if the group's first occurrence was a later one.
func TestPhase4MergedLineColourFollowsMostUrgentSlot(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	obs := testSource(careplan.KindObservation, "obs-mix", "Observe", map[string]interface{}{"question": "Q?"})
	plan.Items = []careplan.PlanItem{
		// Scheduled FIRST on purpose: the group representative must still
		// be re-tiered by the later-seen late occurrence.
		phase4Item(obs, 1, careplan.StatusScheduled, now.Add(26*time.Hour)),
		phase4Item(obs, 1, careplan.StatusLate, now.Add(-1*time.Hour)),
		phase4Item(obs, 1, careplan.StatusDue, now.Add(30*time.Minute)),
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "observation", now)
	require.Empty(t, v.Tiers[1].Cards, "the whole group rides the late tier")
	require.Empty(t, v.Tiers[2].Cards)
	require.Len(t, v.Tiers[0].Cards, 1, "one merged line for (source × animal)")
	card := v.Tiers[0].Cards[0]
	require.Equal(t, "btn-danger", card.TierClass)
	require.Len(t, card.Slots, 3)
	// Slots are due-time ordered inside the line.
	require.True(t, !card.Slots[1].DueAt.Before(card.Slots[0].DueAt))
	require.True(t, !card.Slots[2].DueAt.Before(card.Slots[1].DueAt))
	require.Equal(t, "btn-danger", card.Slots[0].TierClass)
	require.Equal(t, "btn-warning", card.Slots[1].TierClass, "due-now slot is yellow")
	require.Equal(t, "btn-light border", card.Slots[2].TierClass, "scheduled slot is the white bordered one")
}

// TestPhase4ThreeTimesADayRendersOneLine (4-T4): a 3×/day protocol is ONE
// line with 3 toggles (§3), the fold counter shows the remaining
// occurrences, and the merge holds for both densities.
func TestPhase4ThreeTimesADayRendersOneLine(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	care := testSource(careplan.KindCare, "care-3x", "Meds bath", map[string]interface{}{"note": "bath"})
	plan.Items = []careplan.PlanItem{
		phase4Item(care, 1, careplan.StatusDue, time.Date(2026, 9, 28, 11, 0, 0, 0, time.Local)),
		phase4Item(care, 1, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
		phase4Item(care, 1, careplan.StatusDue, time.Date(2026, 9, 28, 15, 0, 0, 0, time.Local)),
		// A different animal keeps its own line — the merge key includes it.
		phase4Item(care, 2, careplan.StatusDue, time.Date(2026, 9, 28, 9, 30, 0, 0, time.Local)),
	}

	for _, view := range []string{ViewCompact, ViewDetailed} {
		v := BuildDayPlanView(plan, view, "", "care", now)
		require.Len(t, v.Tiers[1].Cards, 2, view+": one line per (source × animal)")
		var merged CardView
		for _, c := range v.Tiers[1].Cards {
			if c.AnimalID == 1 {
				merged = c
			}
		}
		require.Len(t, merged.Slots, 3, view+": 3 toggles on the merged line")
		require.Equal(t, 2, merged.Remaining, view+": the fold counter counts the hidden occurrences")
		require.Equal(t, "2", merged.RemainingCap)
		// Slots ascending by due time (09:00, 11:00, 15:00).
		require.Equal(t, "09:00", merged.Slots[0].DueHM)
		require.Equal(t, "11:00", merged.Slots[1].DueHM)
		require.Equal(t, "15:00", merged.Slots[2].DueHM)
	}
}

// TestPhase4ToggleFlipsGreenImmediately (4-T2): setToggleState's pair-row
// path paints the visible undo sibling btn-success in place on apply — no
// reload, no waiting for the auto-refresh.
func TestPhase4ToggleFlipsGreenImmediately(t *testing.T) {
	for _, fork := range phase4ApplyToggleForks {
		raw := readTemplate(t, fork)
		// The pair-row recolour block: the sibling gets un-hidden, its
		// tier classes are stripped, and apply paints it done-green.
		require.Contains(t, raw, "var tier = sib.getAttribute('data-tier-class');",
			fork+": the pair path reads the sibling's tier colour")
		require.Contains(t, raw, "sib.classList.remove('btn-danger', 'btn-warning', 'btn-light', 'border', 'btn-success');",
			fork+": the previous colour is stripped before recolouring")
		require.Contains(t, raw, "sib.classList.add('btn-success');",
			fork+": apply paints the visible sibling green IMMEDIATELY (§2.2)")
	}
}

// TestPhase4UndoRestoresTierColour (4-T3): undo re-adds the sibling's
// data-tier-class — the CURRENT tier colour, which may differ from the
// colour the button had before applying (§2.4).
func TestPhase4UndoRestoresTierColour(t *testing.T) {
	for _, fork := range phase4ApplyToggleForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw, "tier.split(/\\s+/).forEach(function (c) { if (c) { sib.classList.add(c); } });",
			fork+": undo restores the tier classes (§2.4)")
		// The restore branch must sit AFTER the green one (if/else), so an
		// undo never ends up both green and tier-coloured. Scope the check
		// to the PAIR block (the med single-button path above shares the
		// same idioms).
		pair := raw[strings.Index(raw, "var sib = applied ? pairUndo(btn) : pairApply(btn);"):]
		green := strings.Index(pair, "sib.classList.add('btn-success');")
		restore := strings.Index(pair, "tier.split(/\\s+/)")
		require.True(t, green >= 0 && restore > green,
			fork+": restore is the else-branch of the apply-green flip")
	}
	for _, fork := range phase4ItemLineForks {
		raw := readTemplate(t, fork)
		// Both buttons of the merged pair carry data-tier-class — without
		// it on the UNDO button the apply could not restore anything.
		apply := strings.Index(raw, "plan-item-slot-btn plan-apply-btn")
		undo := strings.Index(raw, "plan-item-slot-btn plan-unapply-btn d-none")
		require.True(t, apply >= 0 && undo > apply, fork+": the slot pair renders apply then undo")
		seg := raw[apply:undo]
		require.Contains(t, seg, `data-tier-class="<%= slot.TierClass %>"`,
			fork+": the apply slot carries its tier colour")
		require.Contains(t, raw[undo:], `data-tier-class="<%= slot.TierClass %>"`,
			fork+": the undo slot carries the tier colour to restore")
	}
}

// TestPhase4RowLineWalksTheLadder (4-T5): the row-kind line shares the
// Phase-1 responsive ladder — year-number animal cell, wrap, then the
// narrow full stack. (The full ladder matrix is pinned in
// TestPhase1LadderCSS; here the row-line-specific cells.)
func TestPhase4RowLineWalksTheLadder(t *testing.T) {
	css := readTemplate(t, "../assets/css/care-plan.scss")

	// Level 3: the row-kind line stacks fully — animal, label, toggles
	// each take the full width inside the narrow media query, and the
	// toggle group aligns left (§5, no clipped toggle).
	media := css[strings.Index(css, "@media (max-width: 767.98px)"):]
	require.Contains(t, media, ".plan-item-line .plan-med-animal {\n    flex: 1 1 100%;",
		"the animal cell stacks first on the row line")
	require.Contains(t, media, ".plan-item-line .plan-med-label {\n    flex: 1 1 100%;",
		"the label wraps full-width (no ellipsis)")
	require.Contains(t, media, ".plan-item-line .plan-med-btns {\n    flex: 1 1 100%;",
		"the toggle group drops under the label")
	require.Contains(t, media, "justify-content: flex-start;",
		"the stacked toggles align left, fully visible")

	// Level 1 hook: the animal cell renders the compact year number, the
	// full label rides on the title attribute.
	for _, fork := range phase4ItemLineForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw, `title="<%= card.AnimalLabel %>"`,
			fork+": the animal cell keeps the full label on hover")
		require.Contains(t, raw, "<%= card.AnimalYear %>",
			fork+": the animal cell renders the year number (§5 level 1)")
	}
}

// TestPhase4WeighingKeepsTheInputModal (4-T6): every merged slot toggle
// carries data-kind + data-detail (per occurrence), and the delegated
// click handler routes input kinds to the modal BEFORE any instant apply.
func TestPhase4WeighingKeepsTheInputModal(t *testing.T) {
	for _, fork := range phase4ItemLineForks {
		raw := readTemplate(t, fork)
		// The applicable-slot apply button must carry the modal payload.
		apply := strings.Index(raw, "plan-item-slot-btn plan-apply-btn")
		require.True(t, apply >= 0, fork+": merged slot toggle missing")
		seg := raw[apply : apply+800]
		require.Contains(t, seg, `data-kind="<%= card.ActionKind %>"`,
			fork+": isInputKind() keys on data-kind — weighing/observation open the modal")
		require.Contains(t, seg, `data-detail="<%= card.Detail %>"`,
			fork+": the modal prefill reads data-detail per occurrence")
		require.Contains(t, seg, `data-due-at="<%= slot.DueAtRFC %>"`,
			fork+": the apply ref points at THIS occurrence, not the line")
	}
	for _, fork := range phase4ApplyToggleForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw, "if (kind === 'weighing' || kind === 'observation') { return true; }",
			fork+": weighing and observation stay input kinds")
		dispatch := strings.Index(raw, "if (isInputKind(btn)) { openApply(btn); return; }")
		instant := strings.Index(raw, "instantApply(btn);")
		require.True(t, dispatch >= 0 && instant > dispatch,
			fork+": the modal dispatch runs before (and instead of) the instant apply")
	}
}

// TestPhase4WeighingSlotFlagsInput: the viewmodel marks weighing slots as
// input kinds (the template keeps the modal affordance for them).
func TestPhase4WeighingSlotFlagsInput(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	weigh := testSource(careplan.KindWeighing, "weigh-1", "Weigh", map[string]interface{}{})
	plan.Items = []careplan.PlanItem{
		phase4Item(weigh, 1, careplan.StatusDue, now.Add(30*time.Minute)),
	}
	v := BuildDayPlanView(plan, ViewCompact, "", "weighing", now)
	require.Len(t, v.Tiers[1].Cards, 1)
	slot := v.Tiers[1].Cards[0].Slots[0]
	require.True(t, slot.NeedsInput, "weighing toggles keep the modal input")
	require.Equal(t, "btn-warning", slot.TierClass, "due-now weighing toggle is yellow")
}
