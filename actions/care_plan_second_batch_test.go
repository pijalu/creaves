package actions

import (
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// Bugs.md second batch (2026-10-27 evening) template pins. Each test pins
// ONE user requirement across ALL FOUR locale forks of the touched
// partial.

// #8 tab order: Medication, Care, Feeding, Observation, Weighing,
// Cleanup — most to least important, on the care_plan chip bar AND the
// same sequence for the animal view's protocol sections.
func TestCareKindTabOrderFollowsImportance(t *testing.T) {
	require.Equal(t, []string{
		careplan.KindMedication,
		careplan.KindCare,
		careplan.KindFeeding,
		careplan.KindObservation,
		careplan.KindWeighing,
		careplan.KindCleanup,
	}, actionKinds)
}

// #8 animal view: the protocol tab (per-day groups) must follow the SAME
// importance sequence as the day-plan tabs. The medication series render
// first by construction (Group.Series before Items in the template); the
// non-medication Items were sorted ALPHABETICALLY (care, cleanup, feeding,
// observation, weighing) which put least-important cleanup SECOND. The
// day sort must follow the actionKinds rank instead.
func TestAnimalProtocolDayItemsFollowImportanceOrder(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 10, 2, 12, 0, 0, 0, time.Local)
	plan.Now = now

	srcs := map[string]careplan.PlanSource{
		careplan.KindCare:        testSource(careplan.KindCare, "c1", "Care", map[string]interface{}{"note": "wound"}),
		careplan.KindFeeding:     testSource(careplan.KindFeeding, "f1", "Feed", map[string]interface{}{"food": "x"}),
		careplan.KindObservation: testSource(careplan.KindObservation, "o1", "Obs", map[string]interface{}{"prompt": "Eating?"}),
		careplan.KindWeighing:    testSource(careplan.KindWeighing, "w1", "Weigh", map[string]interface{}{}),
		careplan.KindCleanup:     testSource(careplan.KindCleanup, "cl1", "Clean", map[string]interface{}{"note": "desinfecter"}),
	}
	// Scrambled input on purpose: the output order must come from the
	// importance rank, not from insertion or alphabetical luck.
	order := []string{careplan.KindCleanup, careplan.KindWeighing, careplan.KindObservation, careplan.KindFeeding, careplan.KindCare}
	for i, kind := range order {
		it := testItem(srcs[kind], 1, careplan.StatusDue)
		it.Occurrence.DueAt = time.Date(2026, 10, 2, 8+i, 0, 0, 0, time.Local)
		plan.Items = append(plan.Items, it)
	}

	days := animalTreatmentDays(plan, &models.Animal{ID: 1})
	require.Len(t, days, 1)
	require.Len(t, days[0].Items, 5)
	got := make([]string, 0, 5)
	for _, it := range days[0].Items {
		got = append(got, it.ActionKind)
	}
	require.Equal(t, []string{
		careplan.KindCare,
		careplan.KindFeeding,
		careplan.KindObservation,
		careplan.KindWeighing,
		careplan.KindCleanup,
	}, got, "bugs.md #8: protocol day items follow the importance order (cleanup last, not alphabetical)")
}

// #2 "Treatment should likely be obsolete as its function is taken by
// protocol" — the animal SHOW page drops the Treatment tab: the per-day
// togglable medication series it rendered are the SAME series the protocol
// tab renders (a strict superset — protocol adds the non-medication kinds
// and counts ALL open work on the day badge). The tab's UNIQUE content —
// the legacy manual-treatments journal (history, superseded annotations,
// protocol backlinks) — moves INTO the protocol tab as a collapsed history
// card, so no information is lost. The dashboard's ?med= series deep link
// and every other inbound #nav-treatment link remap to #nav-plan.
func TestAnimalShowRetiresTreatmentTabKeepsLegacyHistory(t *testing.T) {
	for _, f := range []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	} {
		src := readTemplate(t, f)
		require.NotContains(t, src, "nav-treatment", f+": the duplicate tab is gone (bugs.md #2)")
		require.Contains(t, src, `id="planLegacyJournal"`, f+": the legacy journal lives in the protocol tab")
		require.Contains(t, src, "TreatmentEntriesMap", f+": the legacy history itself is preserved")
		require.Greater(t, strings.Index(src, `id="planLegacyJournal"`), strings.Index(src, `id="animalCarePlan"`),
			 f+": the journal renders AFTER the live protocol days (history below the plan)")
		// The dashboard's ?med= deep link activates the protocol tab — that
		// is where the medication series render now.
		require.Contains(t, src, "$('#nav-plan-tab').tab('show');", f+": ?med= lands on the protocol tab")
	}
	for _, f := range []string{
		"../templates/treatments/index.plush.html",
		"../templates/treatments/index.plush.fr.html",
		"../templates/treatments/index.plush.de.html",
		"../templates/treatments/index.plush.nl.html",
	} {
		require.NotContains(t, readTemplate(t, f), "nav-treatment",
			f+": animal links point at the protocol tab that hosts the history")
	}
	for _, f := range []string{
		"../locales/care_plan.en-us.yaml",
		"../locales/care_plan.fr.yaml",
		"../locales/care_plan.de.yaml",
		"../locales/care_plan.nl.yaml",
	} {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `id: "care_plan.animal_plans.legacy_treatments"`, f+": missing the journal title key")
		i := strings.Index(raw, `id: "care_plan.animal_plans.legacy_treatments"`)
		rest := raw[i:]
		j := strings.Index(rest, "translation:")
		require.GreaterOrEqual(t, j, 0, f+": no translation line for legacy_treatments")
		line := rest[j:]
		if k := strings.Index(line, "\n"); k >= 0 {
			line = line[:k]
		}
		require.NotContains(t, line, `translation: ""`, f+": legacy_treatments must not be empty")
	}
}

// #6 /care_matchers needs filter/search: the matcher library is a plain
// table today — with 18+ named matchers finding one means reading every
// row. A client-side filter over name + description + expression (the
// three visible columns) with a visible hit counter, in all four locales.
func TestCareMatchersIndexHasFilterSearch(t *testing.T) {
	for _, f := range []string{
		"../templates/care_matchers/index.plush.html",
		"../templates/care_matchers/index.plush.fr.html",
		"../templates/care_matchers/index.plush.de.html",
		"../templates/care_matchers/index.plush.nl.html",
	} {
		src := readTemplate(t, f)
		require.Contains(t, src, `id="matcherFilter"`, f+": the search input exists")
		require.Contains(t, src, `id="matcherTable"`, f+": the table is the filter target")
		require.Contains(t, src, `id="matcherFilterCount"`, f+": the hit counter exists")
		require.Contains(t, src, `t("care_plan.matchers.filter_placeholder")`, f+": the placeholder is localized")
		require.Contains(t, src, `addEventListener('input', apply)`, f+": the filter reacts to typing")
		require.Contains(t, src, "textContent.toLowerCase()", f+": the match is case-insensitive over the row text")
	}
	for _, f := range []string{
		"../locales/care_plan.en-us.yaml",
		"../locales/care_plan.fr.yaml",
		"../locales/care_plan.de.yaml",
		"../locales/care_plan.nl.yaml",
	} {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `id: "care_plan.matchers.filter_placeholder"`, f+": missing the filter placeholder key")
		i := strings.Index(raw, `id: "care_plan.matchers.filter_placeholder"`)
		rest := raw[i:]
		j := strings.Index(rest, "translation:")
		require.GreaterOrEqual(t, j, 0, f+": no translation line for filter_placeholder")
		line := rest[j:]
		if k := strings.Index(line, "\n"); k >= 0 {
			line = line[:k]
		}
		require.NotContains(t, line, `translation: ""`, f+": filter_placeholder must not be empty")
	}
}

// #3 feeding/cage: "no need to show the time on top of the animal — the
// button has the time" — the .plan-time-label sub-group header is gone
// from the feed table (the collapsed group header keeps its earliest-time
// hint), and "for multiple animals allow some space between the animal
// and its associated button" — the button span carries a left margin.
func TestFeedTableDropsTimeLabelKeepsChipSpacing(t *testing.T) {
	for _, f := range []string{
		"../templates/care_plan/_plan_tier_feed_table.plush.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.fr.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.de.html",
		"../templates/care_plan/_plan_tier_feed_table.plush.nl.html",
	} {
		src := readTemplate(t, f)
		require.NotContains(t, src, "plan-time-label",
			f+": the time above the animals is redundant — the ○ HH:MM button carries the time")
		// Cage mode keeps the fixed ml-2 gap after the chip; animal mode
		// right-aligns the toggle (ml-auto) — the chip is sr-only there, so
		// justify-content-between would otherwise park it at the cell's left
		// (2026-10-08, user report).
		require.Contains(t, src, `<span class="d-flex align-items-center<%= if (group == "animal") { %> ml-auto<% } else { %> ml-2<% } %>">`,
			f+": the per-animal buttons need spacing from the chip (cage) and right alignment (animal)")
		require.Contains(t, src, "plan-feed-animal-cell",
			f+": animal mode centres the toggle cell vertically (2026-10-08)")
	}
}

// #7 media upload (e.g. /animals/10351#nav-media): "input long enough to
// show the hint", "upload should allow comment/details on media" and "user
// should be allowed to edit it" — the file picker is wide enough for its
// placeholder hint, the upload form carries a comment field, every gallery
// card offers an owner/admin comment edit form, and non-editors still see
// the comment text.
func TestAnimalShowMediaPaneHasCommentUploadAndEdit(t *testing.T) {
	for _, f := range []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	} {
		src := readTemplate(t, f)
		require.NotContains(t, src, "max-width:360px",
			f+": the file input must be wide enough to show its hint")
		require.Contains(t, src, `t("attachments.upload.choose_file")`,
			f+": the picker hint label is localized (data-placeholder + text)")
		require.Contains(t, src, `name="comment"`,
			f+": the upload form accepts an optional comment")
		require.Contains(t, src, `t("attachments.upload.comment_placeholder")`,
			f+": the comment field is explained in the user's language")
		require.Contains(t, src, `action="/attachments/<%= a.ID %>/comment"`,
			f+": gallery cards offer a comment edit form")
		require.Contains(t, src, `value="<%= a.Comment.String %>"`,
			f+": the edit form is prefilled with the current comment")
		require.Contains(t, src, `} else if (a.Comment.Valid) {`,
			f+": non-editors still see the comment text")
	}
	for _, f := range []string{
		"../locales/attachments.en-us.yaml",
		"../locales/attachments.fr.yaml",
		"../locales/attachments.de.yaml",
		"../locales/attachments.nl.yaml",
	} {
		raw := readTemplate(t, f)
		for _, key := range []string{
			"attachments.upload.choose_file",
			"attachments.upload.comment_placeholder",
			"attachments.comment.save",
			"attachments.comment.updated",
			"attachments.comment.too_long",
		} {
			require.Contains(t, raw, `id: "`+key+`"`, f+": missing key "+key)
			i := strings.Index(raw, `id: "`+key+`"`)
			rest := raw[i:]
			j := strings.Index(rest, "translation:")
			require.GreaterOrEqual(t, j, 0, f+": no translation line for "+key)
			line := rest[j:]
			if k := strings.Index(line, "\n"); k >= 0 {
				line = line[:k]
			}
			require.NotContains(t, line, `translation: ""`, f+": "+key+" must not be empty")
		}
	}
}
