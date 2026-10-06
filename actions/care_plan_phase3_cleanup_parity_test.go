package actions

// Phase 3 / D3 — cleanup parity with feeding/medication
// (docs/care-presentation-guideline.md R2/R3, §1/§2/§3/§4). The cleanup
// section renders the same tiered .plan-tier sections as every other kind:
// ONE line per (source × cage) — or per (source × animal) under
// group=animal — with the due time stated ONCE per time sub-group and one
// tier-coloured toggle per occurrence (med-parity ○/✓ glyphs, immediate
// in-place colour flip), the apply-cage batch kept as the group check, and
// the shared cage⇄animal grouping toggle (feeding + cleanup, §4.2).
//
// Test map (planning doc Phase 3):
//   3-T1 /care_plan?kind=cleanup renders tiered .plan-tier sections with a
//        summary-strip pill per tier (HTTP, rich fixture).
//   3-T2 a cage row states the time ONCE per sub-group and renders one
//        toggle per occurrence folded under it (viewmodel).
//   3-T3 toggle click → immediate green flip: the care-line row honours
//        the .plan-item-slot pair contract _apply_toggle's setToggleState
//        recolours in place (static pins + tier colour policy).
//   3-T4 the group check batch-applies the whole row: ChipRefsJSON carries
//        one ref per applicable occurrence, the batch button sits inside
//        the row's <tr> so flipBatchRow flips every toggle.
//   3-T5 group=cage|animal switch works and persists: one line per animal
//        (cleanup AND feeding), no batch button in animal mode, the choice
//        rides every kind/zone link (planSelfPath + index pins + HTTP).
//   3-T6 the tier header badges speak the summary strip's unit: open
//        applicable OCCURRENCES, and the strip pills are built from the
//        same counters by construction.

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// phase3Forks lists every template fork carrying the Phase-3 markup.
var phase3CareLineForks = []string{
	"../templates/care_plan/_plan_care_line.plush.html",
	"../templates/care_plan/_plan_care_line.plush.fr.html",
	"../templates/care_plan/_plan_care_line.plush.de.html",
	"../templates/care_plan/_plan_care_line.plush.nl.html",
}

var phase3TierCareTableForks = []string{
	"../templates/care_plan/_plan_tier_care_table.plush.html",
	"../templates/care_plan/_plan_tier_care_table.plush.fr.html",
	"../templates/care_plan/_plan_tier_care_table.plush.de.html",
	"../templates/care_plan/_plan_tier_care_table.plush.nl.html",
}

var phase3FeedTableForks = []string{
	"../templates/care_plan/_plan_tier_feed_table.plush.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
	"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
}

var phase3IndexForks = []string{
	"../templates/care_plan/index.plush.html",
	"../templates/care_plan/index.plush.fr.html",
	"../templates/care_plan/index.plush.de.html",
	"../templates/care_plan/index.plush.nl.html",
}

// phase3CleanupPlan builds a day plan with one cleanup source: animals 1+2
// (cage C1, zone Z1) share the 09:00 occurrence, animal 1 owes a second one
// at 11:00, and animal 3 (cage C9, zone Z2) has its own 09:00 row.
func phase3CleanupPlan(now time.Time) *DayPlan {
	plan := testPlan()
	plan.Now = now
	cln := testSource(careplan.KindCleanup, "cln-1", "Cage scrub", map[string]interface{}{"note": "scrub"})
	plan.Items = []careplan.PlanItem{
		phase3ItemAt(cln, 1, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
		phase3ItemAt(cln, 2, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
		phase3ItemAt(cln, 1, careplan.StatusDue, time.Date(2026, 9, 28, 11, 0, 0, 0, time.Local)),
		phase3ItemAt(cln, 3, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
	}
	return plan
}

// phase3ItemAt wraps testItem with an explicit due time.
func phase3ItemAt(src careplan.PlanSource, animalID int, status careplan.PlanStatus, due time.Time) careplan.PlanItem {
	it := testItem(src, animalID, status)
	it.Occurrence.DueAt = due
	return it
}

// careRowOf returns the cleanup row of one cage, or fails.
func careRowOf(t *testing.T, rows []CareView, cage string) CareView {
	t.Helper()
	for _, cv := range rows {
		if cv.Cage == cage {
			return cv
		}
	}
	t.Fatalf("no cleanup row for cage %s", cage)
	return CareView{}
}

// ---------------------------------------------------------------------------
// 3-T1 — tiered sections on the rendered page (HTTP)
// ---------------------------------------------------------------------------

// TestPhase3CleanupRendersTieredSections (3-T1): /care_plan?kind=cleanup
// renders the shared .plan-tier sections (care-tier-N ids), a summary-strip
// pill jumping to each, and the rich fixture's cage row with the time stated
// once and one toggle per occurrence.
func TestPhase3CleanupRendersTieredSections(t *testing.T) {
	_, client, baseURL := planFixtureRich(t)

	raw := fetchPlanHTML(t, client, baseURL, "?kind=cleanup")

	// §1: the cleanup section is a tiered .plan-tier section, not the
	// legacy flat table. The fixture's cleanup rule is due soon → Now tier.
	require.Contains(t, raw, `id="care-tier-1"`,
		"the Now cleanup tier renders as a .plan-tier section")
	require.Contains(t, raw, `href="#care-tier-1"`,
		"the summary strip carries a pill jumping to the cleanup tier (R4-7.21)")
	require.Contains(t, raw, "plan-tier",
		"the shared tier panel markup wraps the cleanup section")

	// §3: ONE cage row for the fixture's 2-animal cage — the time sub-group
	// label once, one ○ toggle per occurrence (maskVolatile turns every
	// HH:MM into HHMM, so "○ HHMM" is one rendered apply toggle).
	// B10-7: the scheduled tomorrow/day-after occurrences (2 animals × 2
	// days) render too — the daily rule yields 2 due-now + 4 scheduled.
	require.Contains(t, raw, "plan-cage-row plan-care-row",
		"cleanup rows render through the shared care-line partial")
	require.Contains(t, raw, "plan-time-group")
	require.Contains(t, raw, "plan-time-label")
	require.Equal(t, 6, strings.Count(raw, "○ HHMM"),
		"one ○ toggle per occurrence: 2 due-now + 4 scheduled within the horizon")

	// The group check (batch apply-cage) survives the parity rework.
	require.Contains(t, raw, "plan-cage-apply",
		"the apply-cage batch button stays on the cage row")
	require.Contains(t, raw, `plan-apply-count">6`,
		"the corner pill counts the row's applicable occurrences")
}

// ---------------------------------------------------------------------------
// 3-T2 — time label once + one toggle per occurrence (viewmodel)
// ---------------------------------------------------------------------------

// TestPhase3CageRowStatesTimeOncePerOccurrenceGroup (3-T2): a cage row folds
// its open occurrences into due-time sub-groups — one label per sub-group,
// one slot (toggle) per occurrence, each slot carrying its OWN animal ref.
func TestPhase3CageRowStatesTimeOncePerOccurrenceGroup(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	plan := phase3CleanupPlan(now)

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)

	// One row per (source × cage): C1 (3 occurrences) and C9 (1).
	require.Len(t, v.Cares, 2, "one cleanup row per (source × cage)")
	row := careRowOf(t, v.Cares, "C1")
	require.Equal(t, "Z1", row.Zone, "the row carries its zone for tbase()")
	require.Equal(t, "Cage scrub", row.SourceName)
	require.Equal(t, 3, row.ApplicableCount, "all 3 occurrences are open applicable work")
	require.Equal(t, 3, row.Count)

	// The tier stamping happens on the CareTiers copies (fillCareTiers):
	// the row lands in the Now tier with the ONE group dot.
	stamped := careRowOf(t, v.CareTiers[1], "C1")
	require.Equal(t, 1, stamped.Tier, "the row lands in the Now tier")
	require.Equal(t, "btn-warning", stamped.TierClass)
	require.Equal(t, "due", stamped.GroupStatus, "R4-4.4: ONE group dot — the most urgent status")
	require.Equal(t, "plan-dot-due", stamped.GroupStatusClass)

	// §3.2: TWO time sub-groups (09:00 with the 2 shared occurrences, then
	// 11:00) — the label is stated ONCE per group, never per animal.
	require.Len(t, row.TimeGroups, 2, "occurrences fold into due-time sub-groups")
	tg := row.TimeGroups[0]
	require.Equal(t, "09:00", tg.Label, "the first sub-group states its time once")
	require.Equal(t, 2, tg.Count)
	require.Equal(t, 2, tg.Applicable)
	require.Len(t, tg.Slots, 2, "one toggle per occurrence under the shared label")
	require.NotEqual(t, tg.Slots[0].AnimalID, tg.Slots[1].AnimalID,
		"each slot carries its OWN animal ref (the apply key)")
	require.ElementsMatch(t, []int{1, 2}, []int{tg.Slots[0].AnimalID, tg.Slots[1].AnimalID})

	second := row.TimeGroups[1]
	require.Equal(t, "11:00", second.Label)
	require.Len(t, second.Slots, 1)
	require.Equal(t, 1, second.Slots[0].AnimalID)

	// The slots speak the ONE tier policy (§2): due-now → btn-warning, and
	// the row's tier follows its most urgent open occurrence.
	for _, grp := range row.TimeGroups {
		for _, s := range grp.Slots {
			require.Equal(t, "btn-warning", s.TierClass, "due-now occurrence toggle is yellow (§2.1)")
		}
	}

	// The C9 row stays a plain single-occurrence row.
	c9 := careRowOf(t, v.Cares, "C9")
	require.Len(t, c9.TimeGroups, 1)
	require.Len(t, c9.TimeGroups[0].Slots, 1)
	require.Equal(t, 3, c9.TimeGroups[0].Slots[0].AnimalID)
}

// ---------------------------------------------------------------------------
// 3-T3 — immediate colour flip: the slot-pair contract (static pins)
// ---------------------------------------------------------------------------

// TestPhase3CleanupTogglePairContract (3-T3): the care-line row renders each
// occurrence as a .plan-item-slot apply/undo PAIR with data-tier-class on
// BOTH buttons — the exact parent contract _apply_toggle's setToggleState
// needs to flip the colour in place (§2.2) and restore the CURRENT tier
// colour on undo (§2.4). The flip implementation itself is pinned by the
// Phase-4 tests (shared _apply_toggle forks).
func TestPhase3CleanupTogglePairContract(t *testing.T) {
	for _, fork := range phase3CareLineForks {
		raw := readTemplate(t, fork)

		// §2.3 med-parity glyphs: ○ to-do / ✓ done on the toggle pair.
		require.Contains(t, raw, `title="<%= t("care_plan.apply.action") %>">○ <%= slot.DueHM %></button>`,
			fork+": the apply toggle speaks the ○ to-do glyph")
		require.Contains(t, raw, `title="<%= t("care_plan.action.undo") %>">✓ <%= slot.DueHM %></button>`,
			fork+": the undo toggle speaks the ✓ done glyph")

		// The pair contract: one .plan-item-slot parent per occurrence,
		// apply visible + undo d-none, BOTH carrying data-tier-class.
		require.Contains(t, raw, `<span class="plan-item-slot">`,
			fork+": each occurrence gets the shared slot parent setToggleState queries")
		apply := strings.Index(raw, "plan-item-slot-btn plan-apply-btn")
		undo := strings.Index(raw, "plan-item-slot-btn plan-unapply-btn d-none")
		require.True(t, apply >= 0 && undo > apply, fork+": apply renders before its undo sibling")
		require.Contains(t, raw[apply:undo], `data-tier-class="<%= slot.TierClass %>"`,
			fork+": the apply toggle carries its tier colour")
		require.Contains(t, raw[undo:], `data-tier-class="<%= slot.TierClass %>"`,
			fork+": the undo toggle carries the tier colour to restore")

		// §2.1: the colour comes from the ONE slotTierClass policy, and the
		// apply ref keys on the OCCURRENCE (its own animal + due time).
		require.Contains(t, raw, `class="btn <%= slot.TierClass %> btn-sm plan-item-btn plan-item-slot-btn plan-apply-btn"`)
		require.Contains(t, raw, `data-animal-id="<%= slot.AnimalID %>"`,
			fork+": the apply key is the occurrence's own animal, not the cage row's")
		require.Contains(t, raw, `data-due-at="<%= slot.DueAtRFC %>"`)
		require.Contains(t, raw, `data-kind="cleanup"`,
			fork+": cleanup is not an input kind — instantApply flips it immediately")
	}
}

// TestPhase3LateCleanupToggleIsRed: the tier colour policy reaches the
// cleanup toggles — a late occurrence renders a btn-danger toggle (§2.1).
func TestPhase3LateCleanupToggleIsRed(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	cln := testSource(careplan.KindCleanup, "cln-late", "Cage scrub", nil)
	plan.Items = []careplan.PlanItem{
		phase3ItemAt(cln, 1, careplan.StatusLate, now.Add(-2*time.Hour)),
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)
	require.NotEmpty(t, v.CareTiers[0], "the late cleanup row lands on the late tier")
	row := v.CareTiers[0][0]
	require.Equal(t, 0, row.Tier)
	require.Equal(t, "btn-danger", row.TierClass)
	require.Equal(t, 1, row.LateCount, "the late occurrence is counted")
	slot := row.TimeGroups[0].Slots[0]
	require.Equal(t, "btn-danger", slot.TierClass, "§2.1: the toggle itself carries the tier colour")
}

// ---------------------------------------------------------------------------
// 3-T4 — the group check batch-applies the whole row
// ---------------------------------------------------------------------------

// TestPhase3BatchApplyCageFlipsTheWholeRow (3-T4): the cage row's batch refs
// carry ONE entry per applicable occurrence (each with its own animal_id),
// and the batch button sits inside the row's <tr> so the shared
// flipBatchRow flips every toggle of the row without a reload.
func TestPhase3BatchApplyCageFlipsTheWholeRow(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	plan := phase3CleanupPlan(now)

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)
	row := careRowOf(t, v.Cares, "C1")

	// Viewmodel: the data-items payload is one ref per applicable
	// occurrence, keyed (source_type, source_id, animal_id, due_at).
	var refs []map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(row.ChipRefsJSON), &refs))
	require.Len(t, refs, 3, "one batch ref per applicable occurrence")
	animals := map[float64]int{}
	for _, r := range refs {
		require.Equal(t, row.SourceType, r["source_type"])
		require.Equal(t, row.SourceID, r["source_id"])
		require.Contains(t, r, "due_at")
		animals[r["animal_id"].(float64)]++
	}
	require.Equal(t, map[float64]int{1: 2, 2: 1}, animals,
		"animal 1 owes 2 occurrences, animal 2 one — the batch covers them all")

	// Static pins: the batch button is INSIDE the row's <tr> (flipBatchRow
	// scopes the flip to btn.closest('tr')) and carries the payload.
	for _, fork := range phase3CareLineForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw, `<tr class="plan-cage-row plan-care-row"`,
			fork+": the cleanup row stays a <tr> — flipBatchRow's scope")
		batch := strings.Index(raw, "plan-cage-apply")
		require.True(t, batch >= 0, fork+": the apply-cage group check renders")
		seg := raw[batch:]
		require.Contains(t, seg, `data-items="<%= ccard.ChipRefsJSON %>"`,
			fork+": the batch button carries the per-occurrence refs")
		require.Contains(t, raw, `plan-apply-count"><%= ccard.ApplicableCount %>`,
			fork+": the corner pill counts the applicable occurrences (R4-4.2)")
	}
	for _, fork := range phase3IndexForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw, `row.querySelectorAll('.plan-feeding-one:not(.d-none), .plan-apply-btn:not(.d-none)')`,
			fork+": flipBatchRow flips every visible apply toggle inside the row — cleanup toggles included")
		require.Contains(t, raw, `var row = btn.closest('tr');`,
			fork+": the batch flip is scoped to the button's own row")
	}
}

// ---------------------------------------------------------------------------
// 3-T5 — group=cage|animal switch + persistence
// ---------------------------------------------------------------------------

// TestPhase3GroupAnimalOneLinePerAnimal (3-T5, viewmodel): under
// group=animal the cleanup section renders ONE line per (source × animal)
// with the tinted-animal identity and the same time-grouped toggles; the
// feeding section does the same per (animal × diet).
func TestPhase3GroupAnimalOneLinePerAnimal(t *testing.T) {
	now := time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	plan := phase3CleanupPlan(now)

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "animal", now)
	require.Equal(t, "animal", v.Group)

	// 3 occurrences of animal 1 fold into ONE line (2 time sub-groups);
	// animals 2 and 3 get their own lines.
	require.Len(t, v.Cares, 3, "one cleanup line per (source × animal)")
	byAnimal := map[int]CareView{}
	for _, cv := range v.Cares {
		require.NotEmpty(t, cv.AnimalYear, "§5 level 1: the animal cell shows the year number")
		require.NotEmpty(t, cv.AnimalLabel, "the full label stays for the cell title")
		require.NotEmpty(t, cv.AnimalLink, "the animal cell links to the animal plan tab")
		byAnimal[cv.AnimalID] = cv
	}
	a1 := byAnimal[1]
	require.Equal(t, "C1", a1.Cage, "the cage is demoted to a caption, not dropped")
	require.Len(t, a1.TimeGroups, 2, "animal 1 keeps its two time sub-groups")
	require.Len(t, a1.TimeGroups[0].Slots, 1, "one toggle per occurrence — never shared across animals")
	require.Equal(t, 1, a1.TimeGroups[0].Slots[0].AnimalID)
	require.Equal(t, 2, a1.ApplicableCount)
	require.Empty(t, a1.ChipRefsJSON, "§4.3: no batch payload in animal mode (batch is cage-scoped)")
	require.Equal(t, "11/26", a1.AnimalYear)

	// Feeding follows the same toggle (§4.2): one line per (animal × diet)
	// with the per-occurrence toggle preserved.
	fplan := testPlan()
	fplan.Now = now
	feed := testSource(careplan.KindFeeding, "feed-1", "Feed", map[string]interface{}{"food": "Croquettes"})
	fplan.Items = []careplan.PlanItem{
		phase3ItemAt(feed, 1, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
		phase3ItemAt(feed, 2, careplan.StatusDue, time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)),
	}
	fv := BuildDayPlanView(fplan, ViewCompact, "", "feeding", "animal", now)
	require.Len(t, fv.Feedings, 2, "one feeding line per (animal × diet)")
	for _, fg := range fv.Feedings {
		require.Len(t, fg.Chips, 1, "a single-animal line carries exactly one chip")
		require.Equal(t, fg.AnimalID, fg.Chips[0].AnimalID)
		require.NotEmpty(t, fg.AnimalYear)
		require.Empty(t, fg.ChipRefsJSON, "no batch apply-group in animal mode")
		require.False(t, fg.Collapsible, "a one-chip line has nothing to collapse")
		require.Equal(t, 1, fg.ApplicableCount, "the toggle stays live on the animal line")
	}
	// Cage mode keeps the merged row — the grouping is a VIEW preference.
	cv := BuildDayPlanView(fplan, ViewCompact, "", "feeding", "", now)
	require.Len(t, cv.Feedings, 1, "cage mode keeps one (cage × diet) row")
	require.Len(t, cv.Feedings[0].Chips, 2)
}

// TestPhase3GroupChoicePersists (3-T5, persistence): the group choice rides
// the self path and every kind/zone link, and the toggle renders only on
// the two grouped kinds with the right active state.
func TestPhase3GroupChoicePersists(t *testing.T) {
	require.Contains(t, planSelfPath(ViewCompact, "", "cleanup", "animal", ""),
		"group=animal", "the self path carries the grouping preference")
	require.NotContains(t, planSelfPath(ViewCompact, "", "cleanup", "", ""),
		"group=", "the default cage grouping stays out of the URL")

	for _, fork := range phase3IndexForks {
		raw := readTemplate(t, fork)
		// The toggle only renders for the grouped kinds (§4.2).
		require.Contains(t, raw, `<%= if (view.Kind == "feeding" || view.Kind == "cleanup") { %>`,
			fork+": the grouping toggle is scoped to feeding + cleanup")
		require.Contains(t, raw, "plan-group-toggle")
		// Both options link the CURRENT kind/zone with the explicit group,
		// and the active state follows view.Group.
		require.Contains(t, raw, `&group=cage"`, fork+": the cage option keeps kind/zone context")
		require.Contains(t, raw, `&group=animal"`, fork+": the animal option keeps kind/zone context")
		require.Contains(t, raw, `<%= if (view.Group != "animal") { %> active<% } %>`,
			fork+": cage is active by default")
		require.Contains(t, raw, `<%= if (view.Group == "animal") { %> active<% } %>`)
		// The kind tabs thread the choice (R4 persistence: switching kind
		// must not silently reset the grouping).
		require.Contains(t, raw, `<%= if (view.Group != "") { %>&group=<%= view.Group %><% } %>`,
			fork+": the kind tabs persist the grouping preference")
	}

	// Locale keys exist in all four locales.
	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		require.Contains(t, raw, `id: "care_plan.group.label"`, lang+": group toggle label")
		require.Contains(t, raw, `id: "care_plan.group.cage"`, lang+": cage option")
		require.Contains(t, raw, `id: "care_plan.group.animal"`, lang+": animal option")
	}
}

// TestPhase3GroupAnimalRendersHTTP (3-T5, HTTP): the rendered pages honour
// the switch — animal cells + per-occurrence toggles, no batch button, and
// the animal option active.
func TestPhase3GroupAnimalRendersHTTP(t *testing.T) {
	_, client, baseURL := planFixtureRich(t)

	cleanup := fetchPlanHTML(t, client, baseURL, "?kind=cleanup&group=animal")
	require.Contains(t, cleanup, "plan-care-animal",
		"cleanup animal mode renders the tinted animal cell")
	require.NotContains(t, cleanup, ` plan-cage-apply"`,
		"§4.3: no batch group-check button in animal mode (the JS selector string stays)")
	// One line per animal: the fixture cage holds 2 animals → 2 ○ toggles,
	// each on its own row (plan-care-row appears once per line).
	require.Equal(t, 2, strings.Count(cleanup, "plan-care-animal"),
		"one animal cell per (source × animal) line")
	require.Contains(t, cleanup, `&group=animal"`,
		"the grouping rides the rendered links (persistence)")

	feeding := fetchPlanHTML(t, client, baseURL, "?kind=feeding&group=animal")
	require.Contains(t, feeding, "plan-feed-animal",
		"feeding animal mode renders the tinted animal cell (§4.2)")
	require.NotContains(t, feeding, ` plan-feeding-apply"`,
		"§4.3: no batch apply-group button in animal mode (the JS selector string stays)")
	require.Contains(t, feeding, "plan-feeding-one",
		"the per-occurrence toggle survives on the single-animal feeding line")
}

// ---------------------------------------------------------------------------
// 3-T6 — tier badges speak the summary strip's unit
// ---------------------------------------------------------------------------

// TestPhase3TierBadgesSpeakSummaryStripUnit (3-T6): the cleanup tier header
// badge counts OPEN APPLICABLE OCCURRENCES (not rows) — the unit the
// summary strip speaks — and the strip pills are built from the SAME
// counters, so header and strip can never disagree (§1.4).
func TestPhase3TierBadgesSpeakSummaryStripUnit(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	cln := testSource(careplan.KindCleanup, "cln-t", "Cage scrub", nil)
	plan.Items = []careplan.PlanItem{
		// C1: a LATE row holding 2 occurrences (one late, one due) — the
		// row lands on its most urgent tier and the badge counts BOTH
		// occurrences (R4-1.3: one unit, one number, everywhere).
		phase3ItemAt(cln, 1, careplan.StatusLate, now.Add(-2*time.Hour)),
		phase3ItemAt(cln, 2, careplan.StatusDue, now.Add(30*time.Minute)),
		// C9: a separate due-now row.
		phase3ItemAt(cln, 3, careplan.StatusDue, now.Add(45*time.Minute)),
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)

	// Occurrence unit: the C1 row (tier 0) contributes TWO, the C9 row
	// (tier 1) one — rows would read 1/1, occurrences read 2/1.
	require.Equal(t, 2, v.CareTierOpen[0], "the late tier badge counts occurrences, not rows")
	require.Equal(t, 1, v.CareTierOpen[1])
	require.Equal(t, BadgeCap(2), v.CareTierOpenCap[0])
	require.Equal(t, 0, v.CareTierOpen[2])

	// The summary strip is built from the SAME counters by construction:
	// one pill per tier that has work, pointing at the real section id.
	require.Len(t, v.TierLinks, 2)
	require.Equal(t, "care-tier-0", v.TierLinks[0].ID)
	require.Equal(t, v.CareTierOpen[0], v.TierLinks[0].Num)
	require.Equal(t, v.CareTierOpenCap[0], v.TierLinks[0].Cap)
	require.Equal(t, "care-tier-1", v.TierLinks[1].ID)
	require.Equal(t, v.CareTierOpen[1], v.TierLinks[1].Num)
	require.Equal(t, v.CareTierOpenCap[1], v.TierLinks[1].Cap)

	// The header badge equals the sum of its rows' applicable occurrences.
	for ti := 0; ti < 3; ti++ {
		sum := 0
		for _, cv := range v.CareTiers[ti] {
			sum += cv.ApplicableCount
		}
		require.Equal(t, v.CareTierOpen[ti], sum, "tier %d badge == sum of row occurrences", ti)
	}
}

// TestPhase3CleanupTierTableContract: the tier body partial loops the
// care-line partial with the full data map the row needs (ccard, ti, ri,
// group) — the grouping choice must reach every row.
func TestPhase3CleanupTierTableContract(t *testing.T) {
	for _, fork := range phase3TierCareTableForks {
		raw := readTemplate(t, fork)
		require.Contains(t, raw,
			`<%= partial("care_plan/plan_care_line.plush.html", {ccard: ccard, ti: ti, ri: ri, group: group}) %>`,
			fork+": every cleanup row renders through the shared line partial with the grouping")
	}
}
