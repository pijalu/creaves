package actions

// Phase 2 / defect D2 — /care_rules/{id} HTML show page.
//
// 2-T1 browser GET → 200 HTML, all rule fields + back link honouring
//     ?back=/care_plan?kind=cleanup (the source-link round-trip).
// 2-T2 Accept: application/json → the same payload as before the change
//     (API contract preserved).
// 2-T3 click-through: the care_plan work screen (cleanup AND medication
//     kinds) renders the rule source link; following it yields the show
//     page.
// 2-T4 non-admin gate: a regular user gets 403 on the HTML show page.
// 2-T5 4-locale sweep: localized title/labels, no missing-key marker.
// 2-T6 invalid ?back= (open-redirect attempt) falls back to /care_rules.

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// showRule builds one cleanup rule with a matcher, a validity window and
// the stop/latch flags on so the show page renders every field.
func showRule(t *testing.T, f *planFixture) (*models.CareRule, *models.CareMatcher) {
	t.Helper()
	matcher := &models.CareMatcher{
		ID:         uuid.Must(uuid.NewV4()),
		Name:       "MS-" + f.marker,
		Expression: `species = "CP-A1"`,
	}
	require.NoError(t, models.DB.Create(matcher))

	due := itemDueSoon(time.Now())
	from := due.Add(-24 * time.Hour).Truncate(time.Minute)
	to := due.Add(72 * time.Hour).Truncate(time.Minute)
	payload, err := json.Marshal(map[string]interface{}{"instructions": "Nettoyer la cage " + f.marker})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID:              uuid.Must(uuid.NewV4()),
		Name:            "RS-" + f.marker,
		Description:     nulls.NewString("Show page rule " + f.marker),
		ActionKind:      "cleanup",
		ActionPayload:   payload,
		Schedule:        careScheduleJSON(t, due),
		MatcherID:       uuid.NullUUID{UUID: matcher.ID, Valid: true},
		Active:          true,
		Priority:        7,
		ValidFrom:       &from,
		ValidTo:         &to,
		StopOnOuttake:   true,
		LatchMembership: true,
	}
	require.NoError(t, models.DB.Create(rule))
	return rule, matcher
}

// planGetHTML issues a browser-style GET (text/html) and returns status+body.
func planGetHTML(t *testing.T, client *http.Client, baseURL, path string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(raw)
}

// TestCareRuleShowHTMLBrowserGet (2-T1): a browser GET renders the read-only
// detail page with every field and the back link pointing at the ?back=
// target (the cleanup work screen).
func TestCareRuleShowHTMLBrowserGet(t *testing.T) {
	f := setupPlanFixture(t)
	rule, matcher := showRule(t, f)
	client, baseURL := planAdminClient(t)

	back := "/care_plan?kind=cleanup"
	code, body := planGetHTML(t, client, baseURL,
		"/care_rules/"+rule.ID.String()+"?back="+url.QueryEscape(back))
	require.Equal(t, http.StatusOK, code, "body: %.500s", body)

	// Content-Type negotiated to HTML, not the raw JSON blob of D2.
	require.NotContains(t, body[:min(len(body), 200)], `"action_kind"`,
		"browser GET must not render the raw JSON document")

	// Every field rendered.
	require.Contains(t, body, rule.Name, "rule name")
	require.Contains(t, body, "Show page rule "+f.marker, "description")
	require.Contains(t, body, matcher.Name, "matcher name")
	require.Contains(t, body, ">7</p>", "priority")
	require.Contains(t, body, "Cleanup", "localized kind badge (default en)")
	require.Contains(t, body, "Nettoyer la cage "+f.marker, "humanized payload content")
	require.Contains(t, body, "Every day at", "humanized schedule")
	require.Contains(t, body, rule.ValidFrom.Format("2006-01-02"), "valid from date")
	require.Contains(t, body, rule.ValidTo.Format("2006-01-02"), "valid to date")
	require.Contains(t, body, "Stop on outtake", "stop flag label (default en)")

	// Back link honours ?back= (href is HTML-attribute-escaped).
	require.Contains(t, body, `href="/care_plan?kind=cleanup"`, "back link target")

	// Admin actions present: edit + delete.
	require.Contains(t, body, "/care_rules/"+rule.ID.String()+"/edit", "edit link")
	require.Contains(t, body, `data-method="DELETE"`, "delete button")
}

// TestCareRuleShowJSONUnchanged (2-T2): the JSON branch renders the same
// payload shape as before the responder fork.
func TestCareRuleShowJSONUnchanged(t *testing.T) {
	f := setupPlanFixture(t)
	rule, matcher := showRule(t, f)
	client, baseURL := planAdminClient(t)

	code, raw := planDoJSON(t, client, baseURL, "GET", "/care_rules/"+rule.ID.String(), planToken(t, client, baseURL), nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.Contains(t, http.DetectContentType(raw), "text/plain") // sanity: body readable

	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, rule.ID.String(), out["id"])
	require.Equal(t, rule.Name, out["name"])
	require.Equal(t, "cleanup", out["action_kind"])
	require.Equal(t, matcher.ID.String(), out["matcher_id"])
	require.Equal(t, true, out["active"])
	require.Equal(t, float64(7), out["priority"])
	// Payload/schedule documents survive verbatim (json.RawMessage nests as
	// an object, not a string).
	payload, ok := out["action_payload"].(map[string]interface{})
	require.True(t, ok, "action_payload is a nested JSON object: %v", out["action_payload"])
	require.Equal(t, "Nettoyer la cage "+f.marker, payload["instructions"])
}

// TestCareRuleShowClickThrough (2-T3): the care_plan work screen renders the
// rule source link for both the cleanup and the medication kind; following
// the link returns the HTML show page whose back link returns to the work
// screen.
func TestCareRuleShowClickThrough(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	due := itemDueSoon(time.Now())
	cleanupRule, _ := showRule(t, f)
	medRule := ruleWithoutMatcher(t, models.DB, "RMED-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))

	for _, kind := range []string{"cleanup", "medication"} {
		kind := kind
		t.Run(kind, func(t *testing.T) {
			code, body := planGetHTML(t, client, baseURL, "/care_plan?kind="+kind)
			require.Equal(t, http.StatusOK, code, "body: %.300s", body)

			want := cleanupRule
			if kind == "medication" {
				want = medRule
			}
			// The source link targets the show page and carries the work
			// screen URL as its back param.
			require.Contains(t, body, "/care_rules/"+want.ID.String()+"?back=",
				"%s work screen must link the rule source", kind)

			// Follow the link exactly as emitted (query-escaped back param).
			link := "/care_rules/" + want.ID.String() + "?back=" + url.QueryEscape("/care_plan?kind="+kind)
			code2, showBody := planGetHTML(t, client, baseURL, link)
			require.Equal(t, http.StatusOK, code2, "body: %.300s", showBody)
			require.Contains(t, showBody, want.Name)
			require.Contains(t, showBody, `href="/care_plan?kind=`+kind+`"`,
				"show page back link returns to the %s work screen", kind)
		})
	}
}

// TestCareRuleShowNonAdminGate (2-T4): non-admin users are refused on the
// HTML branch too.
func TestCareRuleShowNonAdminGate(t *testing.T) {
	f := setupPlanFixture(t)
	rule, _ := showRule(t, f)
	reg, regURL := planRegularClient(t)

	code, _ := planGetHTML(t, reg, regURL, "/care_rules/"+rule.ID.String())
	require.Equal(t, http.StatusForbidden, code)
}

// TestCareRuleShowAllLocales (2-T5): the show page renders localized
// title + field labels in every UI language with no missing-key marker.
func TestCareRuleShowAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	rule, matcher := showRule(t, f)
	client, baseURL := planAdminClient(t)

	want := map[string][]string{
		"fr":    {"Règle de soins", "Retour", "Fenêtre de validité", "Nettoyage"},
		"en-US": {"Care rule", "Back", "Validity window", "Cleanup"},
		"de":    {"Pflegeregel", "Zurück", "Gültigkeitsfenster", "Reinigung"},
		"nl":    {"Zorgregel", "Terug", "Geldigheidsvenster", "Schoonmaak"},
	}

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/care_rules/"+rule.ID.String(), nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)
			body := string(raw)
			require.Equal(t, http.StatusOK, resp.StatusCode, "%s body: %.500s", lang, body)
			require.NotContains(t, body, "translation missing",
				"%s must not render missing-key markers", lang)
			require.Contains(t, body, matcher.Name, "%s matcher name", lang)
			for _, s := range want[lang] {
				esc := strings.ReplaceAll(s, "'", "&#39;")
				require.Contains(t, body, esc, "%s must render %q", lang, s)
			}
		})
	}
}

// TestCareRuleShowBackSanitized (2-T6): an external ?back= (open-redirect
// attempt) never lands in the back link — the link falls back to the rules
// list.
func TestCareRuleShowBackSanitized(t *testing.T) {
	f := setupPlanFixture(t)
	rule, _ := showRule(t, f)
	client, baseURL := planAdminClient(t)

	for _, evil := range []string{"https://evil.example", "//evil.example", "/\\evil.example"} {
		code, body := planGetHTML(t, client, baseURL,
			"/care_rules/"+rule.ID.String()+"?back="+url.QueryEscape(evil))
		require.Equal(t, http.StatusOK, code)
		// The back LINK itself never carries the hostile target (the
		// language-switcher URLs legitimately echo the request URL — out of
		// scope here, they point back at this same page).
		require.Contains(t, body, `<a href="/care_rules" class="btn btn-info" id="backLink">`,
			"back=%q falls back to the rules list", evil)
	}
}
