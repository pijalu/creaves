package actions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// R4-7.25: "instead of using a check mark for item to do — use the same as
// treatment".
//
// Measured before the change:
//
//	treatments/:id   open item -> <span class="badge badge-warning entry-status">
//	                              <i class="far fa-clock"></i> To do
//	                 done item -> <span class="badge badge-success entry-status">
//	                              <i class="fas fa-check"></i> Done
//	care_plan        open item -> <button … plan-apply-btn
//	                              title="Apply"><i class="fas fa-check"></i>
//
// So the very same treatment read "Done" on one page and "to do" on the other,
// and the ✓ is the app-wide COMPLETED glyph (dashboard "Done", todos "Fait",
// drugs/cares checkboxes). The treatment page's pair is the reference and is
// now the single language: clock = to do, check = done.
//
// The glyph lives in markup, not in Go, so this is asserted against the four
// locale forks; the rendered proof is the agent-browser pass recorded in
// tmp/browser_evidence/round7/.

// todoGlyphControls are the controls whose job is "record this item" — the
// ones that used to carry a check.
var todoGlyphControls = []string{
	"plan-apply-btn",
	"plan-feeding-one",
	"plan-feeding-apply",
	"plan-cage-apply",
}

// carePlanToDoForks: every locale fork that renders a to-do control.
var carePlanToDoForks = []string{
	"../templates/care_plan/index.plush.html",
	"../templates/care_plan/index.plush.fr.html",
	"../templates/care_plan/index.plush.de.html",
	"../templates/care_plan/index.plush.nl.html",
	"../templates/animals/show.plush.html",
	"../templates/animals/show.plush.fr.html",
	"../templates/animals/show.plush.de.html",
	"../templates/animals/show.plush.nl.html",
	// R4-7.16 moved the observation/care/weighing apply control out of the
	// three duplicated tier tables into ONE shared partial; Phase 0b evolved
	// it into `_plan_item_line`, which (like every care_plan component) is
	// forked per locale — the four copies are byte-identical by policy.
	"../templates/care_plan/_plan_item_line.plush.html",
	"../templates/care_plan/_plan_item_line.plush.fr.html",
	"../templates/care_plan/_plan_item_line.plush.de.html",
	"../templates/care_plan/_plan_item_line.plush.nl.html",
	// Phase 0b moved the feeding cage×diet table (the plan-feeding-one and
	// plan-feeding-apply controls) out of index into its own partial, still
	// in four locale forks.
	"../templates/care_plan/_plan_tier_feed_table.plush.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
}

// isReplacesKindYesNo reports whether a line's check is the animal page's "does
// this protocol replace the kind?" cell — a yes/no fact, not a to-do. It is
// the one legitimate survivor of the swap.
func isReplacesKindYesNo(line string) bool {
	return strings.Contains(line, "p.ReplacesKind")
}

// requireNoCheckOnToDoControls fails if any check mark sits on a line that also
// opens a to-do control.
func requireNoCheckOnToDoControls(t *testing.T, fork, raw string) {
	t.Helper()
	for _, line := range strings.Split(raw, "\n") {
		if !strings.Contains(line, "fas fa-check") || isReplacesKindYesNo(line) {
			continue
		}
		for _, c := range todoGlyphControls {
			require.NotContains(t, line, c,
				fork+": a check still marks a to-do control ("+c+")")
		}
	}
}

// clockInsideToDoControls walks the markup and pairs every to-do control's
// opening tag with the glyph that closes it, so a control cannot lose the clock
// and a stray clock cannot be counted in its place. The buttons here hold
// exactly one icon child, so the glyph is whatever sits between the control's
// opening tag and its `</button>`. Returns how many controls it saw.
func clockInsideToDoControls(t *testing.T, fork, raw string) int {
	t.Helper()
	seen := 0
	for _, c := range todoGlyphControls {
		needle := " " + c
		for rest := raw; ; {
			at := strings.Index(rest, needle)
			if at < 0 {
				break
			}
			body := rest[at+len(needle):]
			// The class may end there, or continue: the animal page appends a
			// Plush conditional (`plan-apply-btn<%= if … %> plan-feeding-entry`).
			// Anything else is a longer name that merely starts the same way.
			if len(body) == 0 || (body[0] != '"' && body[0] != ' ' && body[0] != '<') {
				rest = body
				continue
			}
			tag := rest[at:]
			open := strings.Index(tag, ">")
			closing := strings.Index(tag, "</button>")
			require.True(t, open >= 0 && closing > open,
				fork+": unterminated "+c+" control")
			// Phase 4 / D4: a MERGED occurrence toggle (plan-item-slot-btn)
			// speaks the §2.3 med-parity glyph language (○ to-do · ✓ done)
			// instead of the legacy fa-clock — like `_plan_slot_toggle`, which
			// TestMedicationSlotsKeepTheirOwnPair pins separately. Skip it
			// here; TestMergedItemSlotsKeepMedParityGlyph pins that pair.
			// The needle sits inside the class attribute, so the slot marker
			// may already lie BEHIND the scan position — re-anchor on the
			// opening <button …> that owns this needle.
			tagStart := strings.LastIndex(rest[:at], "<button")
			require.True(t, tagStart >= 0, fork+": "+c+" needle outside any <button>")
			if openTag := rest[tagStart:at]; strings.Contains(openTag, "plan-item-slot-btn") {
				inner := tag[open+1 : closing]
				require.NotContains(t, inner, `fa-check`,
					fork+": a merged slot must never carry the done check")
				rest = tag[closing:]
				continue
			}
			inner := tag[open+1 : closing]
			require.Contains(t, inner, `<i class="far fa-clock"></i>`,
				fork+": "+c+" must open with the treatment page's clock")
			require.NotContains(t, inner, `fa-check`,
				fork+": "+c+" must not open with the completed check")
			seen++
			rest = tag[closing:]
		}
	}
	return seen
}

// TestNoCheckMarksAToDoItem: the regression itself, per fork.
func TestNoCheckMarksAToDoItem(t *testing.T) {
	for _, f := range carePlanToDoForks {
		raw := readTemplate(t, f)
		requireNoCheckOnToDoControls(t, f, raw)
		require.Greater(t, clockInsideToDoControls(t, f, raw), 0,
			f+": no to-do control found — the class names drifted, the assertion "+
				"is now vacuous")
	}
}

// TestEveryToDoControlCarriesTheClock: a count floor so the swap cannot
// half-apply to one locale. R4-7.16 changed the shape: the observation /
// care / weighing apply control left the three duplicated tier tables of each
// index fork and now lives in ONE shared partial, so the per-fork index count
// fell from 6 to 3. Phase 0b moved the two feeding controls out of index into
// the _plan_tier_feed_table partial, so the per-fork index count fell to 1
// (the cleanup cage-apply) while the feed-table forks carry 2 each, and the
// row-line partial became the forked `_plan_item_line` (1 clock x 4 forks).
// The total is therefore 1 clock x 4 index forks + 2 x 4 feed-table forks +
// 1 x 4 animal forks + 1 x 4 item-line forks = 20. What must never change is
// that EVERY to-do control carries the clock — that is asserted per control by
// clockInsideToDoControls in TestNoCheckMarksAToDoItem; this floor is only a
// tripwire against a whole locale losing the swap.
func TestEveryToDoControlCarriesTheClock(t *testing.T) {
	total := 0
	for _, f := range carePlanToDoForks {
		total += strings.Count(readTemplate(t, f), `<i class="far fa-clock"></i>`)
	}
	require.GreaterOrEqual(t, total, 20,
		"expected the clock 1x in each index fork, 2x in each feed-table fork, "+
			"1x in each animal fork, and 1x in each item-line fork = 20")
}

// TestTreatmentPageKeepsTheReferencePair: the treatment page is the page the
// user named, so it must keep saying clock = to do, check = done. If someone
// later "tidies" the treatment badge, the two pages drift apart again and this
// is the test that says so.
func TestTreatmentPageKeepsTheReferencePair(t *testing.T) {
	for _, f := range []string{
		"../templates/treatments/show.plush.html",
		"../templates/treatments/show.plush.fr.html",
		"../templates/treatments/show.plush.de.html",
		"../templates/treatments/show.plush.nl.html",
	} {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `<i class="far fa-clock"></i>`,
			f+": the pending badge must keep the clock (the to-do reference)")
		require.Contains(t, raw, `<i class="fas fa-check"></i>`,
			f+": the done badge must keep the check (the done reference)")
		// And the toggle: pending shows ✓ ("mark as done"), done shows ○.
		require.Contains(t, raw, `entry.Status == "done"`,
			f+": the entry toggle must keep its done/to-do split")
	}
}

// TestMedicationSlotsKeepTheirOwnPair: the medication slot buttons already
// spoke this language — open `○`, applied `✓` in a green button. Pinned so the
// glyph work never "simplifies" them into a check for an open slot.
// Phase 0b: the slot button is the `_plan_slot_toggle` component (still x4).
func TestMedicationSlotsKeepTheirOwnPair(t *testing.T) {
	for _, f := range []string{
		"../templates/care_plan/_plan_slot_toggle.plush.html",
		"../templates/care_plan/_plan_slot_toggle.plush.fr.html",
		"../templates/care_plan/_plan_slot_toggle.plush.de.html",
		"../templates/care_plan/_plan_slot_toggle.plush.nl.html",
	} {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `>○ <%= slot.DueAtHM %>`,
			f+": an open medication slot must stay an empty circle")
		require.Contains(t, raw, `>✓ <%= slot.DueAtHM %>`,
			f+": an applied medication slot must keep the check")
	}
}

// TestMergedItemSlotsKeepMedParityGlyph (Phase 4 / D4, §2.3): a MERGED
// occurrence toggle (plan-item-slot-btn) speaks the medication slot language —
// open `○`, applied `✓` — never the legacy fa-clock and never a green to-do.
// Pinned across the four item-line forks.
func TestMergedItemSlotsKeepMedParityGlyph(t *testing.T) {
	for _, f := range []string{
		"../templates/care_plan/_plan_item_line.plush.html",
		"../templates/care_plan/_plan_item_line.plush.fr.html",
		"../templates/care_plan/_plan_item_line.plush.de.html",
		"../templates/care_plan/_plan_item_line.plush.nl.html",
	} {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `plan-item-slot-btn plan-apply-btn`,
			f+": the merged open toggle is a plan-item-slot-btn apply control")
		require.Contains(t, raw, `>○ <%= slot.DueHM %>`,
			f+": the open merged toggle stays an empty circle (med parity)")
		require.Contains(t, raw, `>✓ <%= slot.DueHM %>`,
			f+": the applied merged toggle keeps the check (med parity)")
	}
}

// TestToDoGlyphDoesNotReflowTheButtonGroup: the clock (11.56 px) is narrower
// than the check it replaced (12.45 px). R4-4.2 exists precisely so a control
// never changes size, so the width is pinned in CSS rather than left to the
// glyph's metrics.
func TestToDoGlyphDoesNotReflowTheButtonGroup(t *testing.T) {
	css := readTemplate(t, "../assets/css/care-plan.scss")
	for _, c := range todoGlyphControls {
		require.Contains(t, css, "."+c,
			"the to-do control ."+c+" must be in the width-pinned rule")
	}
	require.Contains(t, css, "min-width: 2.1rem",
		"the to-do controls must keep the footprint of the check they replaced")
}

// TestToDoControlIsNeverGreen: green is reserved for "applied". An apply
// control that turned green would read as done before it is done — which is
// the bug R4-7.25 is fixing, in colour instead of in a glyph.
func TestToDoControlIsNeverGreen(t *testing.T) {
	for _, f := range carePlanToDoForks {
		for _, line := range strings.Split(readTemplate(t, f), "\n") {
			for _, c := range todoGlyphControls {
				if !strings.Contains(line, c) {
					continue
				}
				require.NotContains(t, line, "btn-success",
					f+": a to-do control ("+c+") must not be green")
			}
		}
	}
}
