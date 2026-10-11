package actions

// t11 regression tests (user request 2026-10-28): "Create a DONE view that
// follow exact table and keep current historique" — each ACTIVE kind tab
// renders its terminal occurrences in the kind's OWN table format (feeding =
// cage×diet groups, cleanup = cage rows, row kinds = _plan_item_line lines,
// medication = the pre-existing MedDone block), as a collapsed
// badge-success .plan-tier, while the generic Historique panel
// (id="plan-history") stays untouched.
//
// Every Done-tier occurrence renders the BUG-22 toggle pair PRE-FLIPPED
// (d-none apply side + visible .plan-unapply-btn) so undo flips it back in
// place; grouped kinds additionally render a group-level UNDO-ALL button
// (.plan-cage-undo, deliberately NOT carrying any to-do glyph class).

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// doneTierFixture builds the rich plan fixture and applies occurrences of
// EACH kind (feeding + cleanup + observation + medication), so every kind
// tab has terminal work to show. Feeding and cleanup are applied for BOTH
// fixture animals: a single-animal cage row renders NO per-animal toggle by
// design (Bug 2026-10-27 #5 — the group button covers the whole row), and
// these tests exist to pin the per-occurrence toggle pair in the Done tier.
// Observation and medication are single-animal (row/med kinds always render
// their own toggle).
func doneTierFixture(t *testing.T) (*planFixture, *http.Client, string) {
	t.Helper()
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	now := time.Now()
	soon := itemDueSoon(now)
	md := cagedRuleMatcher(t, f)

	// Feeding: cage × diet group (fixture cage only).
	feed := f.feedRule(t, models.DB, soon)
	require.NoError(t, models.DB.RawQuery(
		"UPDATE care_rules SET matcher_id = ? WHERE id = ?", md.ID, feed.ID).Exec())

	// Cleanup: cage row.
	clean := ruleWithMatcher(t, models.DB, "RCLN-DONE-"+f.marker, "cleanup",
		planRulePayload(t, "cleanup", map[string]interface{}{"caretype_id": f.defCare.String()}),
		careScheduleJSON(t, soon), md)

	// Observation: row-kind line (applied needs an answer).
	obs := ruleWithMatcher(t, models.DB, "ROBS-DONE-"+f.marker, "observation",
		planRulePayload(t, "observation", map[string]interface{}{"question": "Q-DONE-" + f.marker}),
		careScheduleJSON(t, soon), md)

	// Medication: MedDone precedent (applied slot).
	med := ruleWithMatcher(t, models.DB, "RMED-DONE-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "DoneDrug-" + f.marker, "dosage": "1 ml"}),
		careScheduleJSON(t, soon), md)

	token := planToken(t, client, baseURL)
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)

	for _, aid := range f.animalIDs {
		if it := findItemFrom(items, aid, "feeding", feed.ID); it != nil {
			code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
			require.Equal(t, http.StatusCreated, code, "apply feeding animal %d: %s", aid, raw)
		} else {
			t.Fatalf("feeding occurrence not found in day plan for animal %d", aid)
		}
		if it := findItemFrom(items, aid, "cleanup", clean.ID); it != nil {
			code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
			require.Equal(t, http.StatusCreated, code, "apply cleanup animal %d: %s", aid, raw)
		} else {
			t.Fatalf("cleanup occurrence not found in day plan for animal %d", aid)
		}
	}
	if it := findItemFrom(items, f.animalIDs[0], "observation", obs.ID); it != nil {
		ref := itemRef(it)
		ref["answer"] = "yes"
		code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, ref)
		require.Equal(t, http.StatusCreated, code, "apply observation: %s", raw)
	} else {
		t.Fatal("observation occurrence not found in day plan")
	}
	if it := findItemFrom(items, f.animalIDs[0], "medication", med.ID); it != nil {
		code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
		require.Equal(t, http.StatusCreated, code, "apply medication: %s", raw)
	} else {
		t.Fatal("medication occurrence not found in day plan")
	}

	return f, client, baseURL
}

// sectionAfter returns the HTML tail starting at the first marker, for
// ordering assertions scoped to one rendered tier.
func sectionAfter(t *testing.T, html, marker string) string {
	t.Helper()
	i := strings.Index(html, marker)
	require.NotEqual(t, -1, i, "marker %q missing from rendered page", marker)
	return html[i:]
}

// TestDoneTierFeedingRendersCageDietGroup: the feeding tab renders its
// terminal occurrences as a cage×diet group in the SAME table partial as
// open work: group-level undo-all (.plan-cage-undo) + pre-flipped chip pair
// (d-none .plan-feeding-one + visible .plan-unapply-btn).
func TestDoneTierFeedingRendersCageDietGroup(t *testing.T) {
	f, client, baseURL := doneTierFixture(t)
	defer f.cleanup()

	html := fetchPlanHTML(t, client, baseURL, "?kind=feeding")

	require.Contains(t, html, `id="feed-tier-done"`,
		"feeding tab must render the collapsed Done tier")
	done := sectionAfter(t, html, `id="feed-tier-done"`)

	require.Contains(t, done, "plan-cage-undo",
		"Done feeding group must render the group-level undo-all button")
	require.Contains(t, done, "plan-feeding-one d-none",
		"applied chip must keep the d-none apply half of the pair (BUG-22 contract)")
	require.Contains(t, done, `plan-unapply-btn`,
		"applied chip must render the visible ✓ undo half of the pair")

	// Pair order inside the Done tier: apply half first, undo half second.
	iApply := strings.Index(done, "plan-feeding-one d-none")
	iUndo := strings.Index(done, "plan-unapply-btn")
	require.Less(t, iApply, iUndo,
		"the d-none apply half must precede its visible unapply sibling")

	// The group undo-all button carries NO to-do glyph class — the glyph
	// policy tests pin this at template level (care_plan_round7_todo_glyph_test.go).
	require.NotContains(t, done, "plan-cage-undo plan-feeding-apply",
		"group undo-all must not mix with the to-do apply class")

	// The generic Historique panel survives untouched (user directive).
	require.Contains(t, html, `id="plan-history"`,
		"the generic Historique panel must remain on the page")
}

// TestDoneTierCleanupRendersCageRows: the cleanup tab renders terminal
// occurrences as cage rows with applied slots rendered as pre-flipped pairs.
func TestDoneTierCleanupRendersCageRows(t *testing.T) {
	f, client, baseURL := doneTierFixture(t)
	defer f.cleanup()

	html := fetchPlanHTML(t, client, baseURL, "?kind=cleanup")

	require.Contains(t, html, `id="care-tier-done"`,
		"cleanup tab must render the collapsed Done tier")
	done := sectionAfter(t, html, `id="care-tier-done"`)

	require.Contains(t, done, "plan-cage-undo",
		"Done cleanup cage row must render the group-level undo-all button")
	require.Contains(t, done, "plan-apply-btn d-none",
		"applied slot must keep the d-none apply half of the pair")
	require.Contains(t, done, "plan-unapply-btn",
		"applied slot must render the visible ✓ undo half of the pair")
	require.Contains(t, done, `data-kind="cleanup"`,
		"cleanup undo buttons carry data-kind=cleanup for the unapply POST")

	require.Contains(t, html, `id="plan-history"`,
		"the generic Historique panel must remain on the page")
}

// TestDoneTierRowKindRendersLines: a row-kind tab (observation) renders
// terminal occurrences as _plan_item_line lines with the real undo toggle
// AND the separate view-record link (R4-7.11b sibling, never a replacement).
func TestDoneTierRowKindRendersLines(t *testing.T) {
	f, client, baseURL := doneTierFixture(t)
	defer f.cleanup()

	html := fetchPlanHTML(t, client, baseURL, "?kind=observation")

	require.Contains(t, html, `id="tier-done"`,
		"row-kind tab must render the collapsed Done tier")
	done := sectionAfter(t, html, `id="tier-done"`)

	require.Contains(t, done, "plan-apply-btn d-none",
		"applied line must keep the d-none apply half of the pair")
	require.Contains(t, done, "plan-unapply-btn",
		"applied line must render the visible ✓ undo half of the pair")
	require.Contains(t, done, "plan-item-view",
		"the view-record link survives as a SIBLING of the toggle (BUG-22)")

	require.Contains(t, html, `id="plan-history"`,
		"the generic Historique panel must remain on the page")
}

// TestDoneTierMedicationKeepsMedDone: the medication tab keeps its
// pre-existing MedDone series block (tierId tier-done) — fillDoneViews is a
// deliberate no-op for medication so no duplicate Done tier appears.
func TestDoneTierMedicationKeepsMedDone(t *testing.T) {
	f, client, baseURL := doneTierFixture(t)
	defer f.cleanup()

	html := fetchPlanHTML(t, client, baseURL, "?kind=medication")

	require.Contains(t, html, `id="tier-done"`,
		"medication tab keeps the MedDone terminal block")
	require.NotContains(t, html, `id="feed-tier-done"`,
		"medication tab must not leak the feeding Done tier")
	require.NotContains(t, html, `id="care-tier-done"`,
		"medication tab must not leak the cleanup Done tier")

	require.Contains(t, html, `id="plan-history"`,
		"the generic Historique panel must remain on the page")
}

// TestDoneTierNotOnOtherKindTabs: the feeding/cleanup Done tiers are scoped
// to their own kind tab (fillDoneViews + stage-3 filters).
func TestDoneTierNotOnOtherKindTabs(t *testing.T) {
	f, client, baseURL := doneTierFixture(t)
	defer f.cleanup()

	obs := fetchPlanHTML(t, client, baseURL, "?kind=observation")
	require.NotContains(t, obs, `id="feed-tier-done"`,
		"observation tab must not render the feeding Done tier")
	require.NotContains(t, obs, `id="care-tier-done"`,
		"observation tab must not render the cleanup Done tier")

	feed := fetchPlanHTML(t, client, baseURL, "?kind=feeding")
	require.NotContains(t, feed, `id="care-tier-done"`,
		"feeding tab must not render the cleanup Done tier")

	care := fetchPlanHTML(t, client, baseURL, "?kind=cleanup")
	require.NotContains(t, care, `id="feed-tier-done"`,
		"cleanup tab must not render the feeding Done tier")
}
