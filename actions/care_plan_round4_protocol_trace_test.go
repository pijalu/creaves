package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// Round-4 R4-6 tests: the applicable-protocol trace on the animal page
// lists the sources that ACTUALLY produced occurrences for that animal —
// its own plans AND the global rules that apply — and the global-rule edit
// link is admin-only (a non-admin never sees a dead link).

// TestProtocolTraceListsOnlyApplicableSources (unit): the trace is built
// from the engine pass, one row per source that produced an occurrence for
// THIS animal, animal plans first, with the admin gate on rule edit links.
func TestProtocolTraceListsOnlyApplicableSources(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Now()

	// The animal's own medication protocol — contributes one occurrence.
	mkAnimalPlan(t, animalID, "TracePlan-"+f.marker, "medication",
		`{"drug":"TraceDrug-`+f.marker+`","dosage":"1 ml"}`, fixedSchedule(today, 5), true)

	// A global feeding rule that matches every in-care animal — applies.
	usedRule := ruleWithoutMatcher(t, models.DB, "TraceRule-"+f.marker, "feeding",
		json.RawMessage(`{"food":"grenouilles"}`), json.RawMessage(fixedSchedule(today, 5)))
	// A global rule whose course ended before today — it contributes NO
	// occurrence for this animal, so it must NOT appear in the trace.
	unused := fixedSchedule(today.AddDate(0, 0, -7), 1)
	ruleWithoutMatcher(t, models.DB, "TraceUnused-"+f.marker, "care",
		json.RawMessage(`{"note":"never applies"}`), json.RawMessage(unused))

	trace, err := protocolTraceOf(models.DB, animalTodayPlanForTest(t, animalID), animalByID(t, animalID), true)
	require.NoError(t, err)

	byName := map[string]ProtocolSourceView{}
	for _, s := range trace.Sources {
		byName[s.Name] = s
		require.Positive(t, s.Occurrences)
	}
	require.Contains(t, byName, "TracePlan-"+f.marker, "the animal's own protocol is traced")
	require.Contains(t, byName, "TraceRule-"+f.marker, "the matching global rule is traced")
	require.NotContains(t, byName, "TraceUnused-"+f.marker, "a rule that produced nothing never appears")

	require.True(t, byName["TracePlan-"+f.marker].Editable, "animal plans are editable")
	require.Equal(t, fmt.Sprintf("/animals/%d#nav-plan", animalID), byName["TracePlan-"+f.marker].EditURL)
	require.Equal(t, string(careplanSourceAnimal), byName["TracePlan-"+f.marker].SourceType)

	rule := byName["TraceRule-"+f.marker]
	require.True(t, rule.Editable, "admins may edit a global rule from the trace")
	require.Equal(t, "/care_rules/"+usedRule.ID.String(), rule.EditURL)
	require.NotEmpty(t, rule.Schedule, "the trace carries the raw schedule (humanised in the template)")
	require.Equal(t, "grenouilles", rule.Content)

	// Ordering: animal plans first, then rules.
	require.Equal(t, string(careplanSourceAnimal), trace.Sources[0].SourceType)

	// Non-admin: the rule is still listed, without an edit link.
	asUser, err := protocolTraceOf(models.DB, animalTodayPlanForTest(t, animalID), animalByID(t, animalID), false)
	require.NoError(t, err)
	for _, s := range asUser.Sources {
		if s.SourceType == string(careplanSourceAnimal) {
			require.True(t, s.Editable, "animal plans stay editable for every user")
			continue
		}
		require.False(t, s.Editable, "global rules show no edit link to a non-admin")
	}
}

// TestAnimalProtocolTraceAllLocales renders the trace card in every locale
// fork and asserts the admin/non-admin edit-link rule end to end.
func TestAnimalProtocolTraceAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	today := time.Now()

	mkAnimalPlan(t, animalID, "TracePlan-"+f.marker, "medication",
		`{"drug":"TraceDrug-`+f.marker+`","dosage":"1 ml"}`, fixedSchedule(today, 5), true)
	rule := ruleWithoutMatcher(t, models.DB, "TraceRule-"+f.marker, "feeding",
		json.RawMessage(`{"food":"grenouilles"}`), json.RawMessage(fixedSchedule(today, 5)))

	admin, baseURL := planAdminClient(t)
	user, userURL := planRegularClient(t)

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		html := animalShowHTMLAs(t, admin, baseURL, animalID, lang)
		require.Contains(t, html, `id="planTraceTable"`, lang+": the trace table renders")
		require.Contains(t, html, "TracePlan-"+f.marker, lang+": the animal protocol is listed")
		require.Contains(t, html, "TraceRule-"+f.marker, lang+": the applicable rule is listed")
		require.Contains(t, html, "/care_rules/"+rule.ID.String(), lang+": admin sees the rule edit link")
		require.Contains(t, html, "plan-trace-animal", lang+": animal sources are marked")
		require.Contains(t, html, "plan-trace-rule", lang+": rule sources are marked")
	}

	// A non-admin sees the same list but never a rule edit link.
	userHTML := animalShowHTMLAs(t, user, userURL, animalID, "en-US")
	require.Contains(t, userHTML, `id="planTraceTable"`, "the trace renders for a regular user")
	// Scope to the trace TABLE element: the day rows below it (and the
	// protocol definitions table it now shares a card with) legitimately
	// link to a source protocol for every user.
	traceTable := traceTableHTML(userHTML)
	require.Contains(t, traceTable, "TraceRule-"+f.marker, "the rule is still visible")
	require.NotContains(t, traceTable, "/care_rules/"+rule.ID.String(),
		"a non-admin never gets a dead link to a global rule")
}

// traceTableHTML slices exactly the traceability table out of a rendered
// animal page — from its id up to its closing </table>.
func traceTableHTML(page string) string {
	i := strings.Index(page, `id="planTraceTable"`)
	if i < 0 {
		return ""
	}
	j := strings.Index(page[i:], "</table>")
	if j < 0 {
		return ""
	}
	return page[i : i+j]
}

// animalShowHTMLAs fetches one animal page with an explicit client and
// language cookie.
func animalShowHTMLAs(t *testing.T, client *http.Client, baseURL string, animalID int, lang string) string {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+fmt.Sprintf("/animals/%d", animalID), nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "animal show rendered")
	return string(raw)
}

// animalByID loads the animal row the trace is projected for.
func animalByID(t *testing.T, id int) *models.Animal {
	t.Helper()
	a := &models.Animal{}
	require.NoError(t, models.DB.Find(a, id))
	return a
}

// animalTodayPlanForTest runs the same engine pass the animal page uses.
func animalTodayPlanForTest(t *testing.T, animalID int) *DayPlan {
	t.Helper()
	plan, err := animalTodayPlan(models.DB)
	require.NoError(t, err)
	return plan
}

// careplanSourceAnimal mirrors careplan.SourceAnimal for the assertions
// without importing the engine package name twice.
const careplanSourceAnimal = "animal"
