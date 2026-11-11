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
	"github.com/stretchr/testify/require"
)

// traceForks: the four locale forks of the animal page.
func traceForks() []string {
	return []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	}
}

// traceRowBlock isolates the trace table's row template, so the assertions
// count one row's markup rather than the whole page.
func traceRowBlock(t *testing.T, raw string) string {
	t.Helper()
	start := strings.Index(raw, `id="planTraceTable"`)
	require.True(t, start > 0, "the trace table must exist")
	rest := raw[start:]
	end := strings.Index(rest, "</table>")
	require.True(t, end > 0, "the trace table must be closed")
	return rest[:end]
}

// R4-7.22 / R4-7.19 — the animal Protocol tab rendered the SAME protocol twice:
//
//	Details card  : badge | name | content | schedule | actions
//	definitions   : name | kind | content | active | window | replaces | schedule
//
// Measured on animals/10312, the description appeared 3× and the schedule 2×;
// `richPlanName` also re-appended the content to the name cell, so one row
// carried it twice on its own. The caregiver had to read the same sentence
// three times to learn one thing.
//
// Removing the second table is only safe if the trace ALREADY carries every
// protocol — otherwise an idle protocol becomes unreachable (measured on
// animals/10221: TWO protocols existed only in the definitions table, one of
// them inactive). These tests pin that guarantee, so the duplication can go
// without losing the ability to edit or delete a protocol.

// TestProtocolTraceListsEveryAnimalProtocolEvenWhenIdle: an animal protocol
// that produced nothing today is still THIS animal's protocol and must remain
// listed and editable.
func TestProtocolTraceListsEveryAnimalProtocolEvenWhenIdle(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Now()

	mkAnimalPlan(t, animalID, "TraceLive-"+f.marker, "medication",
		`{"drug":"Live-`+f.marker+`","dosage":"1 ml"}`, fixedSchedule(today, 5), true)
	// Ended a week ago: produces no occurrence, but is still a protocol.
	mkAnimalPlan(t, animalID, "TraceIdle-"+f.marker, "care",
		`{"note":"ended"}`, fixedSchedule(today.AddDate(0, 0, -7), 1), true)

	trace, err := protocolTraceOf(models.DB, animalTodayPlanForTest(t, animalID), animalByID(t, animalID), true)
	require.NoError(t, err)

	names := map[string]ProtocolSourceView{}
	for _, s := range trace.Sources {
		names[s.Name] = s
	}
	_, hasIdle := names["TraceIdle-"+f.marker]
	require.True(t, hasIdle,
		"an idle animal protocol must stay listed — otherwise it cannot be edited or deleted")

	idle := names["TraceIdle-"+f.marker]
	require.Zero(t, idle.Occurrences, "it produced nothing, and must say so")
	require.True(t, idle.Editable, "it must remain editable")
}

// TestProtocolTraceStillOmitsIdleGlobalRules: the evidence guarantee is NOT
// weakened. A global rule that applies to nobody is noise, and listing every
// rule in the library would bury the ones that actually matter.
func TestProtocolTraceStillOmitsIdleGlobalRules(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Now()

	ruleWithoutMatcher(t, models.DB, "R47Idle-"+f.marker, "care",
		json.RawMessage(`{"note":"never applies"}`),
		json.RawMessage(fixedSchedule(today.AddDate(0, 0, -7), 1)))

	trace, err := protocolTraceOf(models.DB, animalTodayPlanForTest(t, animalID), animalByID(t, animalID), true)
	require.NoError(t, err)
	for _, s := range trace.Sources {
		require.NotContains(t, s.Name, "R47Idle-"+f.marker,
			"a global rule that produces nothing must not be listed")
	}
}

// TestTraceRowStatesItsTypeAndItsScope: R4-7.19 — the row must say WHAT kind
// of protocol it is ("Nourrissage") and WHOSE it is ("Spécifique", a protocol
// set at this animal's level), because the caregiver cannot otherwise tell an
// animal-specific protocol from a global rule.
func TestTraceRowStatesItsTypeAndItsScope(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)

		require.NotContains(t, raw, `id="careAnimalPlansTable"`, f+
			": the definitions table is the duplicate — it must be gone")
		require.NotContains(t, raw, "richPlanName(p.Name", f+
			": richPlanName re-appends the content the row already shows in its own column")
		require.Contains(t, raw, "src.Kind", f+
			": the row must state the protocol type (eg Nourrissage)")
		require.Contains(t, raw, `care_plan.protocol_trace.dedicated`, f+
			": the animal-level protocol must be labelled Dedicated/Spécifique")
		require.Contains(t, raw, `care_plan.protocol_trace.dedicated_hint`, f+
			": 'Dedicated' means nothing without its tooltip")
	}
}

// TestTraceContentIsRenderedOncePerRow: the anti-duplication guard. One
// description, one place. This is the defect the caregiver reported.
//
// Count the RENDER (`<%= src.Content %>`), not every mention: the guard
// `<%= if (src.Content != "") { %>` is a second mention of the field on the
// same line but shows nothing. Counting the bare substring reports a
// duplication that does not exist — and would have hidden a real one.
func TestTraceContentIsRenderedOncePerRow(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)
		row := traceRowBlock(t, raw)
		require.Equal(t, 1, strings.Count(row, "<%= src.Content %>"),
			f+": the description must be rendered exactly once in the row")
		require.Equal(t, 0, strings.Count(row, "richPlanName"),
			f+": the name cell must not re-append the description")
		require.Equal(t, 0, strings.Count(row, "planContentLabel"),
			f+": the payload content must not be rendered a second way")
	}
}

// TestTraceLocaleKeysExistInEveryFork: a key the template calls but the
// locale file lacks renders as the raw id ("care_plan.protocol_trace.dedicated")
// — visible to the caregiver, not a silent no-op. The real i18n loader is used
// so a malformed YAML file fails here too.
func TestTraceLocaleKeysExistInEveryFork(t *testing.T) {
	for _, lang := range []string{"en-US", "fr", "de", "nl"} {
		tr, err := i18n.New(locales.FS(), "en-US")
		require.NoError(t, err, lang)
		for _, id := range []string{
			"care_plan.protocol_trace.dedicated",
			"care_plan.protocol_trace.dedicated_hint",
			"care_plan.protocol_trace.rule_source_hint",
		} {
			got, err := tr.TranslateWithLang(lang, id)
			require.NoError(t, err, lang+": "+id+" is missing")
			require.NotEmpty(t, got, lang+": "+id+" is empty")
			require.NotEqual(t, id, got, lang+": "+id+" fell back to its own id")
		}
	}
}

// TestTraceRowStillCarriesTheRemovedColumns: the definitions table is gone, so
// its unique information must live in the trace row. Status and the ISO
// window had no other home; losing them would leave an inactive protocol
// looking live, and the caregiver unable to tell when a protocol runs.
func TestTraceRowStillCarriesTheRemovedColumns(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)
		row := traceRowBlock(t, raw)
		require.Contains(t, row, "src.Active", f+
			": the trace row must state whether the protocol is active")
		require.Contains(t, row, "planExpired(animal, src.Schedule)", f+
			": an ended protocol must read as expired")
		require.Contains(t, row, "planWindow(animal, src.Schedule)", f+
			": the ISO window is only in this row now")
	}
}

// TestPlanDetailsCarriesTheCRUDAnchorNotTheTable: the delegated editor reads
// its url from #planDetails. It must NOT read it from #planTraceTable: that
// table renders only when the trace is non-empty, so an animal with no
// protocol at all would silently lose the "New protocol" button.
func TestPlanDetailsCarriesTheCRUDAnchorNotTheTable(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `id="planDetails" class="collapse" aria-labelledby="planDetailsHead"`+
			"\n             data-animal-id=", f+
			": #planDetails is always rendered and must carry the CRUD url")
		require.Contains(t, raw, "document.getElementById('planDetails')", f+
			": the editor reads its url from #planDetails")
		require.NotContains(t, raw, "getElementById('planTraceTable')", f+
			": the conditionally-rendered trace table must not gate the editor")
	}
}

// TestCarePlanEditorStillHasItsBackingTable: the create/edit/delete buttons
// are delegated by a script that reads the table id, so removing the table
// means re-pointing it — and a dead selector would silently break editing.
func TestCarePlanEditorStillHasItsBackingTable(t *testing.T) {
	for _, f := range traceForks() {
		raw := readTemplate(t, f)
		require.Contains(t, raw, "data-animal-id=",
			f+": the protocol list still carries the animal id the editor posts to")
		require.Contains(t, raw, `data-url="/animals/`,
			f+": the protocol list still carries the CRUD url")
		require.NotContains(t, raw, `getElementById('careAnimalPlansTable')`,
			f+": the delegated editor script must not point at the removed table")
	}
}
