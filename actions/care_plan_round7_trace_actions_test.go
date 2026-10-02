package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// R4-7.12 (feedback item 15): "replace the '{count} occurrence(s)' with the
// edit/delete". The trace table's occurrence count was a bare number — no
// unit, no anchor, no action — next to content and schedule that already
// explain the row. It is now the row's actions.
func TestTraceTableHasActionsNotOccurrenceCount(t *testing.T) {
	forks := []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.NotContains(t, raw, "protocol_trace.occurrences", f,
			"the {count} occurrence(s) cell is gone")
		require.NotContains(t, raw, "src.Occurrences", f,
			"the row no longer renders the count")

		// an animal plan's row is edited/deleted in place, on this page
		require.Contains(t, raw, `class="btn btn-warning btn-sm carePlanEditBtn" data-id="<%= src.SourceID %>"`, f,
			"the animal plan row carries the in-place editor")
		require.Contains(t, raw, `class="btn btn-danger btn-sm carePlanDeleteBtn" data-id="<%= src.SourceID %>"`, f,
			"the animal plan row carries the delete control")

		// a global rule's row links to ITS OWN editor / destroy
		require.Contains(t, raw, "editCareRulePath({ care_rule_id: src.SourceID })", f,
			"the rule row links to the rule editor")
		require.Contains(t, raw, `"data-method": "DELETE"`, f,
			"the rule row carries a delete control")

		// the rule branch is gated on Editable — a non-admin must never get
		// a control that 403s
		require.Contains(t, raw, `src.SourceType == "animal"`, f)
		require.Contains(t, raw, `} else if (src.Editable && src.EditURL != "") {`, f,
			"the rule controls are gated on Editable")
	}
}

// The animal-plan buttons in the trace are driven by the SAME delegated JS as
// the definitions table below it — the handlers bind by class, so the trace
// rows must use exactly those classes or they would render as inert buttons.
func TestTraceEditorButtonsShareTheDelegatedHandlers(t *testing.T) {
	raw := readTemplate(t, "../templates/animals/show.plush.html")
	require.Contains(t, raw, "document.querySelectorAll('.carePlanEditBtn')",
		"the editor handler binds by class")
	require.Contains(t, raw, "document.querySelectorAll('.carePlanDeleteBtn')",
		"the delete handler binds by class")
	// and the table the handlers post to is present on the same page
	require.Contains(t, raw, `id="careAnimalPlansTable"`, "the editor's backing table is on the page")
	require.Contains(t, raw, `data-url="/animals/<%= animal.ID %>/care_animal_plans"`,
		"the editor knows where to save")
}
