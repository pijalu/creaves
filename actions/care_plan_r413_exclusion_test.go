//go:build !sqlite
// +build !sqlite

package actions

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"creaves/locales"
	"creaves/models"

	"github.com/gobuffalo/mw-i18n/v2"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// R4-7.13 — a per-animal exception to a global care rule (§4.1).
//
// The half of the feature that mattered was MISSING, not buggy:
//
//   - care_rule_exclusions existed in the model and the day-plan engine
//     honoured it (care_plan_dayplan.go: ruleCoversAnimal turns matched=false
//     when an exclusion row exists), but NO route, NO handler and NO page
//     could ever create one. The opt-out was unreachable.
//   - the protocol trace lists the sources that "ACTUALLY produced
//     occurrences" (R4-7.22). An excluded rule produces none, so even a
//     hand-inserted row would have VANISHED from the animal page — leaving no
//     way to see it, explain it or undo it.
//
// appendExcludedRules closes the second half: a suppressed rule is listed,
// flagged, with the reason and the id needed to restore it.

// mkRuleExclusion writes a per-animal opt-out directly, for the engine/trace
// assertions that do not need the HTTP path.
func mkRuleExclusion(t *testing.T, tx *pop.Connection, ruleID uuid.UUID, animalID int, reason string) *models.CareRuleExclusion {
	t.Helper()
	e := &models.CareRuleExclusion{
		ID:       uuid.Must(uuid.NewV4()),
		RuleID:   ruleID,
		AnimalID: animalID,
		Reason:   nulls.NewString(reason),
	}
	require.NoError(t, tx.Create(e))
	return e
}

// occurrenceCountOf counts the plan items THIS animal got from a rule.
func occurrenceCountOf(plan *DayPlan, animalID int, ruleID uuid.UUID) int {
	n := 0
	for i := range plan.Items {
		it := &plan.Items[i]
		if it.Occurrence.AnimalID != animalID || it.Occurrence.Source == nil {
			continue
		}
		if src := it.Occurrence.Source; string(src.SourceType()) == "rule" && src.SourceID() == ruleID.String() {
			n++
		}
	}
	return n
}

// TestExclusionSuppressesTheRuleForThatAnimalOnly: the engine contract. The
// opt-out is PER ANIMAL — the sibling animal in the same cage keeps the rule,
// otherwise excluding one bird would silently unfeed the whole cage.
func TestExclusionSuppressesTheRuleForThatAnimalOnly(t *testing.T) {
	f := setupPlanFixture(t)
	today := time.Now()
	rule := ruleWithoutMatcher(t, models.DB, "R413Feed-"+f.marker, "feeding",
		json.RawMessage(`{"food":"R413food-`+f.marker+`"}`),
		json.RawMessage(fixedSchedule(today, 30)))

	before := animalTodayPlanForTest(t, f.animalIDs[0])
	require.NotZero(t, occurrenceCountOf(before, f.animalIDs[0], rule.ID),
		"the rule must produce occurrences before it is excluded")

	mkRuleExclusion(t, models.DB, rule.ID, f.animalIDs[0], "individual exception")

	after := animalTodayPlanForTest(t, f.animalIDs[0])
	require.Zero(t, occurrenceCountOf(after, f.animalIDs[0], rule.ID),
		"an excluded rule must produce NOTHING for the excluded animal")
	require.NotZero(t, occurrenceCountOf(after, f.animalIDs[1], rule.ID),
		"the exception is per ANIMAL: the sibling must keep the rule")
}

// TestTraceListsTheExcludedRuleAsSuppressed: the reachability guarantee. This
// is the whole point of appendExcludedRules — without it the exception is
// invisible, and an invisible exception can never be undone.
func TestTraceListsTheExcludedRuleAsSuppressed(t *testing.T) {
	f := setupPlanFixture(t)
	today := time.Now()
	rule := ruleWithoutMatcher(t, models.DB, "R413Trace-"+f.marker, "care",
		json.RawMessage(`{"note":"R413 note `+f.marker+`"}`),
		json.RawMessage(fixedSchedule(today, 30)))
	mkRuleExclusion(t, models.DB, rule.ID, f.animalIDs[0], "fractious, feed by hand")

	trace, err := protocolTraceOf(models.DB, animalTodayPlanForTest(t, f.animalIDs[0]),
		animalByID(t, f.animalIDs[0]), true)
	require.NoError(t, err)

	var row *ProtocolSourceView
	for i := range trace.Sources {
		if trace.Sources[i].SourceID == rule.ID.String() {
			row = &trace.Sources[i]
		}
	}
	require.NotNil(t, row,
		"an excluded rule produces no occurrence — it must still be LISTED, or the exception is unremovable")
	require.True(t, row.Excluded, "the row must be flagged as an exception")
	require.Equal(t, "fractious, feed by hand", row.ExclusionReason,
		"the reason is the whole point of the record")
	require.NotEmpty(t, row.ExclusionID, "the restore control needs the row id")
	require.Equal(t, "R413Trace-"+f.marker, row.Name,
		"an appended row has no engine PlanSource to project from — the name must come from the rule")
	require.Equal(t, "care", row.Kind)
	require.Contains(t, row.Content, "R413 note "+f.marker,
		"the appended row must be as informative as an occurrence-derived one")
	require.NotEmpty(t, row.Schedule, "the appended row must carry its schedule")
}

// TestExclusionAPIWriteAndUndo: the HTTP contract. Create, reject a duplicate,
// scope the delete to the animal, and actually remove the row.
func TestExclusionAPIWriteAndUndo(t *testing.T) {
	f := setupPlanFixture(t)
	client, base := planAdminClient(t)
	token := planToken(t, client, base)
	today := time.Now()
	rule := ruleWithoutMatcher(t, models.DB, "R413Api-"+f.marker, "care",
		json.RawMessage(`{"note":"api"}`), json.RawMessage(fixedSchedule(today, 30)))
	path := "/animals/" + itoa(f.animalIDs[0]) + "/care_rule_exclusions"

	status, body := planDoJSON(t, client, base, "POST", path, token,
		map[string]interface{}{"rule_id": rule.ID.String(), "reason": "handled individually"})
	require.Equal(t, 201, status, "creating the exception: %s", body)

	var created models.CareRuleExclusion
	require.NoError(t, json.Unmarshal(body, &created))
	require.Equal(t, "handled individually", created.Reason.String)
	require.Equal(t, f.animalIDs[0], created.AnimalID)
	require.True(t, created.CreatedBy.Valid, "who set the exception is part of the record")

	// A second POST for the same (rule, animal) must not stack a duplicate
	// row: the trace renders one control per rule.
	status, body = planDoJSON(t, client, base, "POST", path, token,
		map[string]interface{}{"rule_id": rule.ID.String(), "reason": "again"})
	require.Equal(t, 409, status, "a duplicate exception must be refused: %s", body)
	require.Equal(t, 1, countRuleExclusions(models.DB, rule.ID, f.animalIDs[0]),
		"exactly one exclusion row must exist")

	// Another animal's exclusion must not be deletable through this animal.
	other := mkRuleExclusion(t, models.DB, rule.ID, f.animalIDs[1], "belongs to the sibling")
	status, _ = planDoJSON(t, client, base, "DELETE", path+"/"+other.ID.String(), token, nil)
	require.Equal(t, 404, status,
		"an exclusion of another animal must be a 404, never a cross-animal delete")
	require.Equal(t, 1, countRuleExclusions(models.DB, rule.ID, f.animalIDs[1]))

	status, body = planDoJSON(t, client, base, "DELETE", path+"/"+created.ID.String(), token, nil)
	require.Equal(t, 200, status, "restoring the rule: %s", body)
	require.Zero(t, countRuleExclusions(models.DB, rule.ID, f.animalIDs[0]),
		"the row must actually be gone, not just reported deleted")
}

// TestExclusionAPIIsOpenToAnyAccountThatCanEditTheAnimal: permission. The
// animal page has no admin gate (AnimalsResource.Update), so an animal's
// exception follows the animal: a plain approved account must be able to set
// and undo it, or the control would render for them and 403.
func TestExclusionAPIIsOpenToAnyAccountThatCanEditTheAnimal(t *testing.T) {
	f := setupPlanFixture(t)
	client, base := planRegularClient(t)
	token := planToken(t, client, base)
	today := time.Now()
	rule := ruleWithoutMatcher(t, models.DB, "R413Perm-"+f.marker, "care",
		json.RawMessage(`{"note":"perm"}`), json.RawMessage(fixedSchedule(today, 30)))
	path := "/animals/" + itoa(f.animalIDs[0]) + "/care_rule_exclusions"

	status, body := planDoJSON(t, client, base, "POST", path, token,
		map[string]interface{}{"rule_id": rule.ID.String(), "reason": "by the caretaker"})
	require.Equal(t, 201, status, "a non-admin must be able to exclude a rule: %s", body)

	var created models.CareRuleExclusion
	require.NoError(t, json.Unmarshal(body, &created))
	status, _ = planDoJSON(t, client, base, "DELETE", path+"/"+created.ID.String(), token, nil)
	require.Equal(t, 200, status, "and to restore it")
}

// TestExclusionAPIRejectsAnUnknownRule: a bad rule id must not create a row
// pointing at nothing.
func TestExclusionAPIRejectsAnUnknownRule(t *testing.T) {
	f := setupPlanFixture(t)
	client, base := planAdminClient(t)
	token := planToken(t, client, base)
	path := "/animals/" + itoa(f.animalIDs[0]) + "/care_rule_exclusions"

	status, _ := planDoJSON(t, client, base, "POST", path, token,
		map[string]interface{}{"rule_id": uuid.Must(uuid.NewV4()).String(), "reason": "ghost"})
	require.Equal(t, 404, status, "an exclusion may only point at a real rule")

	status, _ = planDoJSON(t, client, base, "POST", path, token,
		map[string]interface{}{"rule_id": "not-a-uuid", "reason": "ghost"})
	require.Equal(t, 422, status, "a malformed rule id must be rejected, not stored")
}

// TestTraceRowCarriesTheExceptionControlInEveryFork: the UI is only reachable
// if every locale fork renders the control on a rule row.
func TestTraceRowCarriesTheExceptionControlInEveryFork(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `partial("animals/trace_exception_ctrl.plush.html")`, f+
			": the rule row must render the exception control")
		require.Contains(t, raw, `partial("animals/trace_exception_modal.plush.html")`, f+
			": the reason modal must be rendered once per page")

		ctrl := readTemplate(t, "../templates/animals/_trace_exception_ctrl.plush.html")
		require.Contains(t, ctrl, "careRuleExcludeBtn", "the control offers the opt-out")
		require.Contains(t, ctrl, "careRuleRestoreBtn", "and the way back")
		require.Contains(t, ctrl, "src.ExclusionID", "the restore control needs the exclusion row id")
		require.Contains(t, ctrl, "src.ExclusionReason", "the reason must be shown, not hidden in a tooltip")
	}
}

// TestExceptionLocaleKeysExistInEveryFork: a key the template calls but the
// locale file lacks renders as the raw id — visible to the caregiver, not a
// silent no-op.
func TestExceptionLocaleKeysExistInEveryFork(t *testing.T) {
	ids := []string{
		"care_plan.protocol_trace.exception",
		"care_plan.protocol_trace.exception_hint",
		"care_plan.protocol_trace.exception_reason",
		"care_plan.protocol_trace.exception_reason_ph",
		"care_plan.protocol_trace.exception_save",
		"care_plan.protocol_trace.exception_error",
		"care_plan.protocol_trace.exclude",
		"care_plan.protocol_trace.exclude_intro",
		"care_plan.protocol_trace.restore",
		"care_plan.protocol_trace.restore_confirm",
	}
	for _, lang := range []string{"en-US", "fr", "de", "nl"} {
		tr, err := i18n.New(locales.FS(), "en-US")
		require.NoError(t, err, lang)
		for _, id := range ids {
			got, err := tr.TranslateWithLang(lang, id)
			require.NoError(t, err, lang+": "+id+" is missing")
			require.NotEmpty(t, got, lang+": "+id+" is empty")
			require.NotEqual(t, id, got, lang+": "+id+" fell back to its own id")
		}
	}
}

// TestExceptionScriptNeverInterpolatesATranslationRawly: R4-7.17's rule — a
// translation interpolated into a script with "<%= %>" is HTML-escaped, so the
// caregiver sees raw entities ("l&#39;exception"). Every translated string in
// the exception script must go through jsString.
func TestExceptionScriptNeverInterpolatesATranslationRawly(t *testing.T) {
	raw := readTemplate(t, "../templates/animals/_trace_exception_modal.plush.html")
	script := raw[strings.Index(raw, "<script>"):]
	require.NotContains(t, script, `"<%= t(`,
		"a translation interpolated into the script with <%= %> is HTML-escaped (R4-7.17)")
	for _, id := range []string{
		"care_plan.protocol_trace.exception_error",
		"care_plan.protocol_trace.restore_confirm",
	} {
		require.Contains(t, script, "jsString(t(\""+id+"\"))",
			id+" reaches the script through jsString, or it renders as raw HTML entities")
	}
}
