package actions

// Phase 6 / defect D6 — /care_rules list sorting/filtering/search/paging.
//
// 6-T1 sort asc/desc on each whitelisted column (name/kind/priority/active)
// 6-T2 each filter alone (kind, active, matcher_id) + combined with sort
// 6-T3 q= case-insensitive substring over name AND description
// 6-T4 page=2&per_page=N correct slice + pager markup
// 6-T5 JSON branch unchanged: full set regardless of params
// 6-T6 locale sweep: filter/sort controls render localized, no missing keys

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ruleNamesInOrder extracts the rendered rule-name probe values in row order
// from the list page body. Names contain the unique fixture marker so the
// probes stay isolated from the seeded library.
func ruleNamesInOrder(body string, marker string) []string {
	var names []string
	for _, probe := range []string{"SRT-A-", "SRT-B-", "SRT-C-", "SRT-D-"} {
		if strings.Contains(body, probe+marker) {
			names = append(names, probe+marker)
		}
	}
	// Sort by position in the HTML document = rendered row order.
	pos := func(n string) int { return strings.Index(body, n) }
	for i := 1; i < len(names); i++ {
		for j := i; j > 0 && pos(names[j]) < pos(names[j-1]); j-- {
			names[j], names[j-1] = names[j-1], names[j]
		}
	}
	// Strip the marker so callers compare against the bare probe prefixes.
	for i, n := range names {
		names[i] = strings.TrimSuffix(n, marker)
	}
	return names
}

// sortFixtureRules builds four rules with distinct names/kinds/priorities/
// active flags, two sharing one matcher. All names carry the fixture marker
// so list assertions stay isolated from the seeded library.
func sortFixtureRules(t *testing.T, f *planFixture) (*models.CareMatcher, *models.CareMatcher) {
	t.Helper()
	tx := models.DB

	m1 := &models.CareMatcher{ID: uuid.Must(uuid.NewV4()), Name: "SM1-" + f.marker, Expression: `species = "CP-A1"`}
	m2 := &models.CareMatcher{ID: uuid.Must(uuid.NewV4()), Name: "SM2-" + f.marker, Expression: `species = "CP-A2"`}
	require.NoError(t, tx.Create(m1))
	require.NoError(t, tx.Create(m2))

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	mk := func(name, kind string, prio int, active bool, m *models.CareMatcher, desc string) {
		rule := &models.CareRule{
			ID:            uuid.Must(uuid.NewV4()),
			Name:          name + f.marker,
			Description:   nulls.NewString(desc),
			ActionKind:    kind,
			ActionPayload: planRulePayload(t, "cleanup", map[string]interface{}{"instructions": "i"}),
			Schedule:      sched,
			Active:        active,
			Priority:      prio,
		}
		if m != nil {
			rule.MatcherID = uuid.NullUUID{UUID: m.ID, Valid: true}
		}
		require.NoError(t, tx.Create(rule))
	}

	mk("SRT-A-", "cleanup", 40, true, m1, "AlphaDesc-"+f.marker)
	mk("SRT-B-", "medication", 10, false, m1, "BravoDesc-"+f.marker)
	mk("SRT-C-", "cleanup", 30, true, m2, "CharlieDesc-"+f.marker)
	mk("SRT-D-", "weighing", 20, false, nil, "DeltaDesc-"+f.marker)
	return m1, m2
}

// TestCareRulesListSort (6-T1): each whitelisted column sorts ascending and
// descending; an unknown sort key falls back to name asc.
func TestCareRulesListSort(t *testing.T) {
	f := setupPlanFixture(t)
	sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)

	asc := []string{"SRT-A-", "SRT-B-", "SRT-C-", "SRT-D-"}
	desc := []string{"SRT-D-", "SRT-C-", "SRT-B-", "SRT-A-"}

	// name asc/desc — alphabetical order of the marker-suffixed names.
	for _, c := range []struct {
		query string
		want  []string
	}{
		{"sort=name&dir=asc", asc},
		{"sort=name&dir=desc", desc},
		{"sort=bogus&dir=desc", asc}, // unknown key → default name asc
	} {
		code, body := planGetHTML(t, client, baseURL, "/care_rules?"+c.query)
		require.Equal(t, http.StatusOK, code, "body: %.300s", body)
		assert.Equal(t, c.want, ruleNamesInOrder(body, f.marker), "query %s", c.query)
	}

	// priority asc: B(10) D(20) C(30) A(40); desc reverses exactly.
	code, body := planGetHTML(t, client, baseURL, "/care_rules?sort=priority&dir=asc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-B-", "SRT-D-", "SRT-C-", "SRT-A-"},
		ruleNamesInOrder(body, f.marker))
	code, body = planGetHTML(t, client, baseURL, "/care_rules?sort=priority&dir=desc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-A-", "SRT-C-", "SRT-D-", "SRT-B-"},
		ruleNamesInOrder(body, f.marker))

	// kind asc: cleanup < medication < weighing (A,C then B then D).
	code, body = planGetHTML(t, client, baseURL, "/care_rules?sort=kind&dir=asc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-A-", "SRT-C-", "SRT-B-", "SRT-D-"},
		ruleNamesInOrder(body, f.marker))
	code, body = planGetHTML(t, client, baseURL, "/care_rules?sort=kind&dir=desc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-D-", "SRT-B-", "SRT-A-", "SRT-C-"},
		ruleNamesInOrder(body, f.marker))

	// active asc: false(0) before true(1) → B,D then A,C; desc reverses.
	code, body = planGetHTML(t, client, baseURL, "/care_rules?sort=active&dir=asc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-B-", "SRT-D-", "SRT-A-", "SRT-C-"},
		ruleNamesInOrder(body, f.marker))
	code, body = planGetHTML(t, client, baseURL, "/care_rules?sort=active&dir=desc")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t,
		[]string{"SRT-A-", "SRT-C-", "SRT-B-", "SRT-D-"},
		ruleNamesInOrder(body, f.marker))
}


// TestCareRulesListFilters (6-T2): kind, active and matcher_id filters each
// restrict the list; combined with sort the order holds on the subset.
func TestCareRulesListFilters(t *testing.T) {
	f := setupPlanFixture(t)
	m1, _ := sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)

	markerRules := func(query string) []string {
		code, body := planGetHTML(t, client, baseURL, "/care_rules?"+query)
		require.Equal(t, http.StatusOK, code, "body: %.300s", body)
		return ruleNamesInOrder(body, f.marker)
	}

	// kind=cleanup → A, C (name asc default).
	assert.Equal(t, []string{"SRT-A-", "SRT-C-"}, markerRules("kind=cleanup"))
	// kind=bogus → no marker rule.
	assert.Empty(t, markerRules("kind=bogus-kind"))
	// active=false → B, D.
	assert.Equal(t, []string{"SRT-B-", "SRT-D-"}, markerRules("active=false"))
	// active=true → A, C.
	assert.Equal(t, []string{"SRT-A-", "SRT-C-"}, markerRules("active=true"))
	// matcher_id=m1 → A, B; invalid uuid ignored → all four.
	assert.Equal(t, []string{"SRT-A-", "SRT-B-"}, markerRules("matcher_id="+m1.ID.String()))
	assert.Equal(t,
		[]string{"SRT-A-", "SRT-B-", "SRT-C-", "SRT-D-"},
		markerRules("matcher_id=not-a-uuid"))

	// Combined: kind=cleanup + sort=priority desc → A(40) before C(30).
	assert.Equal(t, []string{"SRT-A-", "SRT-C-"},
		markerRules("kind=cleanup&sort=priority&dir=desc"))
	// Combined: active=false + sort=name desc → D, B.
	assert.Equal(t, []string{"SRT-D-", "SRT-B-"},
		markerRules("active=false&sort=name&dir=desc"))
	// Combined: matcher + active.
	assert.Equal(t, []string{"SRT-A-"},
		markerRules("matcher_id="+m1.ID.String()+"&active=true"))
}

// TestCareRulesListSearch (6-T3): q= matches name and description
// case-insensitively.
func TestCareRulesListSearch(t *testing.T) {
	f := setupPlanFixture(t)
	sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)

	search := func(q string) []string {
		code, body := planGetHTML(t, client, baseURL, "/care_rules?q="+q)
		require.Equal(t, http.StatusOK, code, "body: %.300s", body)
		return ruleNamesInOrder(body, f.marker)
	}

	// Name substring, wrong case → all four (marker is in every name).
	assert.Equal(t,
		[]string{"SRT-A-", "SRT-B-", "SRT-C-", "SRT-D-"},
		search(strings.ToLower("SRT-")))
	// Description-only hit (name does not contain it).
	assert.Equal(t, []string{"SRT-B-"}, search(strings.ToLower("BravoDesc")))
	// Case-insensitive on description.
	assert.Equal(t, []string{"SRT-D-"}, search("DELTADESC"))
	// No match.
	assert.Empty(t, search("zzz-no-such-rule"))
}

// TestCareRulesListPagination (6-T4): page=2&per_page=2 yields the second
// slice of the ordered set and renders the pager.
func TestCareRulesListPagination(t *testing.T) {
	f := setupPlanFixture(t)
	sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)

	// per_page=2 over the marker-only filtered set (q=<marker> appears in
	// all four names and descriptions).
	base := "/care_rules?q=" + f.marker + "&per_page=2&sort=name&dir=asc"
	code, body := planGetHTML(t, client, baseURL, base+"&page=1")
	require.Equal(t, http.StatusOK, code, "body: %.300s", body)
	assert.Equal(t, []string{"SRT-A-", "SRT-B-"}, ruleNamesInOrder(body, f.marker))
	require.Contains(t, body, "pagination", "pager markup present")

	code, body = planGetHTML(t, client, baseURL, base+"&page=2")
	require.Equal(t, http.StatusOK, code, "body: %.300s", body)
	assert.Equal(t, []string{"SRT-C-", "SRT-D-"}, ruleNamesInOrder(body, f.marker),
		"page 2 must be the next slice of the same ordering")

	// Pager preserves the active filters in its links.
	require.Contains(t, body, "q="+f.marker, "pager links carry the search param")
}

// TestCareRulesListJSONUnchanged (6-T5): the JSON branch ignores every list
// parameter and returns the full set — the API contract the care-plan engine
// relies on.
func TestCareRulesListJSONUnchanged(t *testing.T) {
	f := setupPlanFixture(t)
	sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	count := func(path string) int {
		code, raw := planDoJSON(t, client, baseURL, "GET", path, token, nil)
		require.Equal(t, http.StatusOK, code, "body: %.200s", raw)
		var out []map[string]interface{}
		require.NoError(t, json.Unmarshal(raw, &out))
		return len(out)
	}

	full := count("/care_rules")
	require.Equal(t, full, count("/care_rules?sort=name&dir=desc"),
		"JSON ignores sort params")
	require.Equal(t, full, count("/care_rules?kind=cleanup&active=true&q=SRT&page=2&per_page=1"),
		"JSON ignores filter/search/pagination params")

	// And the marker rules are all in the full set.
	_, raw := planDoJSON(t, client, baseURL, "GET", "/care_rules", token, nil)
	for _, probe := range []string{"SRT-A-", "SRT-B-", "SRT-C-", "SRT-D-"} {
		require.Contains(t, string(raw), probe+f.marker)
	}
}

// TestCareRulesListControlsAllLocales (6-T6): filter bar + sortable headers
// render in all four locales with localized labels and no missing-key marker.
func TestCareRulesListControlsAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	sortFixtureRules(t, f)
	client, baseURL := planAdminClient(t)

	want := map[string][]string{
		"fr":    {"Recherche", "Nom ou description…", "Réinitialiser les filtres"},
		"en-US": {"Search", "Name or description…", "Reset filters"},
		"de":    {"Suche", "Name oder Beschreibung…", "Filter zurücksetzen"},
		"nl":    {"Zoeken", "Naam of beschrijving…", "Filters wissen"},
	}

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/care_rules", nil)
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
			// Filter form + sortable headers + pager containers.
			require.Contains(t, body, `id="care_rules_filters"`, "%s filter form", lang)
			for _, col := range []string{"sort=name", "sort=kind", "sort=priority", "sort=active"} {
				require.Contains(t, body, col, "%s sortable column %q", lang, col)
			}
			for _, s := range want[lang] {
				esc := strings.ReplaceAll(s, "'", "&#39;")
				require.Contains(t, body, esc, "%s must render %q", lang, s)
			}
		})
	}
}
