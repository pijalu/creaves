package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// Phase 4 (UX-4, bugs.md U3/U6): fast-action links (animal / source /
// fulfillment), zone tabs + kind chips already gated, skip/defer modal
// contract and cage batch apply.

// ---------------------------------------------------------------------------
// Link helpers (view model, no DB)
// ---------------------------------------------------------------------------

func TestCardLinks(t *testing.T) {
	const back = "/care_plan"
	require.Equal(t, "/animals/42?back=%2Fcare_plan#nav-plan", cardAnimalLink(42, back))
	require.Empty(t, cardAnimalLink(0, back))

	// Rule source → rule show; animal-plan source → the animal's Plan tab.
	require.Equal(t, "/care_rules/abc-123?back=%2Fcare_plan",
		cardSourceLink(string(careplan.SourceRule), "abc-123", 42, back))
	require.Equal(t, "/animals/42?back=%2Fcare_plan#nav-plan",
		cardSourceLink(string(careplan.SourceAnimal), "plan-1", 42, back))

	// Fulfillment links by type; deleted/none/none-id never link (§10-CP1).
	require.Equal(t, "/cares/fid-1?back=%2Fcare_plan",
		cardFulfillmentLink(models.ApplicationFulfillmentCare, "fid-1", false, back))
	require.Equal(t, "/treatments/fid-2?back=%2Fcare_plan",
		cardFulfillmentLink(models.ApplicationFulfillmentTreatment, "fid-2", false, back))
	require.Empty(t, cardFulfillmentLink(models.ApplicationFulfillmentCare, "fid-1", true, back))
	require.Empty(t, cardFulfillmentLink(models.ApplicationFulfillmentCare, planFulfillmentNone, false, back))
	require.Empty(t, cardFulfillmentLink("unknown", "fid", false, back))
}

// TestBuildDayPlanViewLinks: cardFor wires the three link types from the
// plan row + application; done cards carry the fulfillment link.
func TestBuildDayPlanViewLinks(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindObservation, "src-obs", "Obs", nil)
	it := testItem(src, 1, careplan.StatusApplied)
	it.Application = &careplan.ApplicationView{
		Status:          "applied",
		FulfillmentType: models.ApplicationFulfillmentCare,
		FulfillmentID:   "fid-9",
	}
	plan.Items = []careplan.PlanItem{it}

	v := BuildDayPlanView(plan, ViewDetailed, "", "", time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	require.Len(t, v.Tiers[3].Cards, 1, "applied item lands in the done tier")
	card := v.Tiers[3].Cards[0]
	require.Equal(t, "/animals/1?back=%2Fcare_plan%3Fview%3Ddetailed#nav-plan", card.AnimalLink)
	require.Equal(t, "/care_rules/src-obs?back=%2Fcare_plan%3Fview%3Ddetailed", card.SourceLink)
	require.Equal(t, "/cares/fid-9?back=%2Fcare_plan%3Fview%3Ddetailed", card.FulfillmentLink)
}

// TestBuildDayPlanViewCareCards (bugs.md U6): cleanup items leave the
// compact tiers — one cage card carries the per-cage batch refs.
func TestBuildDayPlanViewCareCards(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindCleanup, "src-clean", "Clean (conversion)", map[string]interface{}{"note": "désinfecter"})
	plan.Items = []careplan.PlanItem{
		testItem(src, 1, careplan.StatusDue),
		testItem(src, 2, careplan.StatusDue),
		testItem(src, 3, careplan.StatusApplied), // done: off the work screen in compact
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "", time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	require.Empty(t, v.Tiers[1].Cards, "cleanup work leaves the per-animal tiers in compact")
	require.Empty(t, v.Tiers[3].Cards, "applied cleanup work has no synthetic done card in compact")
	require.Len(t, v.Cares, 1)
	cc := v.Cares[0]
	require.Equal(t, "C1", cc.Cage)
	require.Equal(t, "Z1", cc.Zone)
	require.Equal(t, "Clean", cc.SourceName, "conversion markers stripped")
	require.Equal(t, "/care_rules/src-clean?back=%2Fcare_plan%3Fview%3Dcompact", cc.SourceLink)
	require.Equal(t, 2, cc.ApplicableCount)
	var refs []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(cc.ChipRefsJSON), &refs))
	require.Len(t, refs, 2)
	for _, r := range refs {
		require.Equal(t, "rule", r["source_type"])
		require.Equal(t, "src-clean", r["source_id"])
		require.NotEmpty(t, r["due_at"])
	}

	// Zone filter narrows the card list; counters stay unfiltered.
	vz := BuildDayPlanView(plan, ViewCompact, "Z2", "", time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local))
	require.Empty(t, vz.Cares)
}

// ---------------------------------------------------------------------------
// applicationViewOf: stored row → engine view (bugs.md U3 additive fields)
// ---------------------------------------------------------------------------

func TestApplicationViewOfFulfillment(t *testing.T) {
	a := &models.CarePlanApplication{
		Status:          models.ApplicationStatusApplied,
		FulfillmentType: models.ApplicationFulfillmentCare,
		FulfillmentID:   "fid-1",
	}
	v := applicationViewOf(a)
	require.Equal(t, models.ApplicationFulfillmentCare, v.FulfillmentType)
	require.Equal(t, "fid-1", v.FulfillmentID)

	// The §4.5 "none" placeholder never surfaces as a link target.
	a.FulfillmentID = planFulfillmentNone
	v = applicationViewOf(a)
	require.Empty(t, v.FulfillmentType)
	require.Empty(t, v.FulfillmentID)
}

// ---------------------------------------------------------------------------
// HTTP: JSON fulfillment fields + HTML link/modal/batch rendering
// ---------------------------------------------------------------------------

// TestPlanJSONFulfillmentFields: applying an item surfaces fulfillment_*
// in the JSON read model (additive, backward compatible) while open items
// omit the keys entirely.
func TestPlanJSONFulfillmentFields(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	require.NotEmpty(t, items)
	it := findItemByAnimal(items, f.animalIDs[0], "feeding")
	require.NotNil(t, it)
	_, hasType := it["fulfillment_type"]
	_, hasID := it["fulfillment_id"]
	require.False(t, hasType, "open item omits fulfillment_type")
	require.False(t, hasID, "open item omits fulfillment_id")

	// Apply → the JSON row now carries the fulfillment link fields.
	st, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
	require.Equal(t, http.StatusCreated, st, "apply: %s", raw)

	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	items = planItemsOf(t, body)
	done := findItemByAnimal(items, f.animalIDs[0], "feeding")
	require.NotNil(t, done)
	require.Equal(t, models.ApplicationFulfillmentCare, done["fulfillment_type"])
	require.NotEmpty(t, done["fulfillment_id"])
}

// TestCarePlanDayPlanHTMLLinks (bugs.md U3/U6): the work screen renders
// every fast action — linked animal labels and source names with back=,
// zone tabs with hash-memory markup, the shared skip/defer modal, and the
// care cage card with its apply_batch button.
func TestCarePlanDayPlanHTMLLinks(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	// One feeding rule (feeding card) + one cleanup rule (care cage card),
	// both due inside the window.
	f.feedRule(t, models.DB, itemDueSoon(time.Now()))
	payload := planRulePayload(t, "cleanup", map[string]interface{}{"note": "nettoyer"})
	ruleWithoutMatcher(t, models.DB, "CPCL-"+f.marker, "cleanup", payload, careScheduleJSON(t, itemDueSoon(time.Now())))

	req, err := http.NewRequest("GET", baseURL+"/care_plan", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "html: %.300s", raw)
	html := string(raw)

	// Care cage card (bugs.md U6): one card, batch button with item refs.
	require.Contains(t, html, "plan-cage-card", "cleanup renders as a cage card")
	require.Contains(t, html, "plan-cage-apply", "cage card carries the apply_batch button")
	require.Contains(t, html, "/care_plan/apply_batch", "batch button posts to apply_batch")

	// Zone tabs: badges + hash-memory anchors (z- prefixed).
	require.Contains(t, html, `id="planZoneTabs"`)
	require.Contains(t, html, "#z-", "zone tab hash memory")
	// Kind chips with counts.
	require.Contains(t, html, `id="planKindChips"`)

	// Skip/defer fast actions + shared modal markup.
	require.Contains(t, html, "plan-skip-btn")
	require.Contains(t, html, "plan-defer-btn")
	require.Contains(t, html, `id="planSkipDeferModal"`)
	require.Contains(t, html, "deferred_until", "modal posts deferred_until for defer")

	// Apply one feeding item, then re-render: the done tier links the
	// fulfillment record with back=/care_plan (bugs.md U3).
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	it := findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "feeding")
	require.NotNil(t, it)
	st, raw2 := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
	require.Equal(t, http.StatusCreated, st, "apply: %s", raw2)

	resp2, err := client.Do(req)
	require.NoError(t, err)
	defer resp2.Body.Close()
	raw, _ = io.ReadAll(resp2.Body)
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	html = string(raw)
	require.Contains(t, html, "back=%2Fcare_plan", "fast-action links carry an escaped back=/care_plan")
}

// ---------------------------------------------------------------------------
// R5-2d (D-b): back-link propagation, sanitization, labels
// ---------------------------------------------------------------------------

// TestPlanSelfPathBack: an incoming back target is carried in the work
// screen's self URL — sanitized to same-origin paths; anything else is
// dropped (fallback = the plain care_plan self URL).
func TestPlanSelfPathBack(t *testing.T) {
	require.Equal(t, "/care_plan?view=compact", planSelfPath(ViewCompact, "", "", ""))
	require.Equal(t, "/care_plan?view=compact", planSelfPath(ViewCompact, "", "", "//evil.com"))
	require.Equal(t, "/care_plan?view=compact", planSelfPath(ViewCompact, "", "", "javascript:alert(1)"))
	require.Equal(t, "/care_plan?view=compact", planSelfPath(ViewCompact, "", "", "/\\evil.example"))

	require.Equal(t, "/care_plan?back=%2F&view=compact", planSelfPath(ViewCompact, "", "", "/"))
	require.Equal(t, "/care_plan?back=%2Fdashboard&view=compact", planSelfPath(ViewCompact, "", "", "/dashboard"))
}

// TestBuildDayPlanViewBackChain: the dashboard origin (back=/) survives the
// hop through the work screen — self URL AND card links carry it, so the
// animal page's back button chains to the dashboard, not just the plan.
func TestBuildDayPlanViewBackChain(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindObservation, "src-obs", "Obs", nil)
	it := testItem(src, 1, careplan.StatusDue)
	plan.Items = []careplan.PlanItem{it}
	now := time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)

	v := BuildDayPlanView(plan, ViewCompact, "", "", now, "/")
	require.Equal(t, "/care_plan?back=%2F&view=compact", v.SelfPath)
	require.Equal(t, "/animals/1?back=%2Fcare_plan%3Fback%3D%252F%26view%3Dcompact#nav-plan",
		v.Tiers[1].Cards[0].AnimalLink)

	// No incoming back → cards fall back to the plain self URL.
	v = BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.Equal(t, "/care_plan?view=compact", v.SelfPath)
	require.Equal(t, "/animals/1?back=%2Fcare_plan%3Fview%3Dcompact#nav-plan",
		v.Tiers[1].Cards[0].AnimalLink)
}

// TestLandingBackLabelKey: the U16 back label names the FINAL destination —
// "/" and /dashboard* directly, or through a care_plan self URL carrying
// back=/ (the R5-2d chain).
func TestLandingBackLabelKey(t *testing.T) {
	require.Equal(t, "animals.back.to_dashboard", landingBackLabelKey("/"))
	require.Equal(t, "animals.back.to_dashboard", landingBackLabelKey("/dashboard/"))
	require.Equal(t, "animals.back.to_dashboard", landingBackLabelKey("/care_plan?back=%2F&view=compact"))
	require.Equal(t, "animals.back.to_dashboard", landingBackLabelKey("/care_plan?back=%2Fdashboard&view=compact"))
	require.Equal(t, "animals.back.to_day_plan", landingBackLabelKey("/care_plan?view=compact"))
	require.Equal(t, "animals.back.to_day_plan", landingBackLabelKey("/care_plan"))
	require.Equal(t, "animals.back.to_care_schedule", landingBackLabelKey("/reports/care_schedule?from=2026-01-01"))
	require.Equal(t, "animals.back.to_in_care", landingBackLabelKey("/animals/5"))
	require.Equal(t, "animals.back.to_in_care", landingBackLabelKey("//evil.com"))
}

// TestUnwrapBackChain (D-b): a back target carrying its own ?back=...
// collapses to the embedded origin — the work screen's self URL chains the
// dashboard through, and back buttons must land there directly so the U16
// label and the actual landing never diverge.
func TestUnwrapBackChain(t *testing.T) {
	require.Equal(t, "/", unwrapBackChain("/care_plan?back=%2F&view=compact"))
	require.Equal(t, "/", unwrapBackChain("/care_plan?view=compact&back=%2F"))
	require.Equal(t, "/dashboard/", unwrapBackChain("/care_plan?back=%2Fdashboard%2F&view=compact"))
	// No embedded back → unchanged.
	require.Equal(t, "/care_plan?view=compact", unwrapBackChain("/care_plan?view=compact"))
	require.Equal(t, "/", unwrapBackChain("/"))
	// Only the first hop is collapsed for plain targets.
	require.Equal(t, "/animals/5", unwrapBackChain("/animals/5"))
}
