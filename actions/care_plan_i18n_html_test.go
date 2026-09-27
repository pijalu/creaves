package actions

import (
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestCarePlanPagesAllLocales renders every care-plan UI surface (day plan,
// rules list + editor, matcher library + editor, animal Plan tab, landing
// badge) under all four UI languages (fr base, en-US, de, nl — the locale
// fork convention) and asserts the localized page title is present. This is
// the localized template smoke test: a template variant that fails to parse
// or render fails here per language.
func TestCarePlanPagesAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	animalPath := fmt.Sprintf("/animals/%d", f.animalIDs[0])

	// One saved matcher so the library page and editor select render rows.
	matcher := &models.CareMatcher{
		ID:         uuid.Must(uuid.NewV4()),
		Name:       "M-" + f.marker,
		Expression: `species = "CP-A1"`,
	}
	require.NoError(t, models.DB.Create(matcher))
	_ = f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	byLang := map[string]map[string]string{
		"fr": {
			"/care_plan":         "Plan de la journée",
			"/care_rules":        "Règles de soins",
			"/care_rules/new":    "Nouvelle règle de soins",
			"/care_matchers":     "Bibliothèque de sélecteurs",
			"/care_matchers/new": "Nouveau sélecteur",
			animalPath:           "Plans de cet animal",
			"/landing/index":     "Plan du jour",
		},
		"en-US": {
			"/care_plan":         "Day plan",
			"/care_rules":        "Care rules",
			"/care_rules/new":    "New care rule",
			"/care_matchers":     "Matcher library",
			"/care_matchers/new": "New matcher",
			animalPath:           "Plans for this animal",
			"/landing/index":     "Day plan",
		},
		"de": {
			"/care_plan":         "Tagesplan",
			"/care_rules":        "Pflegeregeln",
			"/care_rules/new":    "Neue Pflegeregel",
			"/care_matchers":     "Matcher-Bibliothek",
			"/care_matchers/new": "Neuer Matcher",
			animalPath:           "Pläne dieses Tieres",
			"/landing/index":     "Tagesplan",
		},
		"nl": {
			"/care_plan":         "Dagplan",
			"/care_rules":        "Zorgregels",
			"/care_rules/new":    "Nieuwe zorgregel",
			"/care_matchers":     "Matcherbibliotheek",
			"/care_matchers/new": "Nieuwe matcher",
			animalPath:           "Plannen van dit dier",
			"/landing/index":     "Dagplan",
		},
	}

	paths := []string{
		"/care_plan", "/care_rules", "/care_rules/new",
		"/care_matchers", "/care_matchers/new",
		animalPath, "/landing/index",
	}

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			for _, path := range paths {
				path := path
				t.Run(path, func(t *testing.T) {
					req, err := http.NewRequest("GET", baseURL+path, nil)
					require.NoError(t, err)
					req.Header.Set("Accept", "text/html")
					req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
					resp, err := client.Do(req)
					require.NoError(t, err)
					defer resp.Body.Close()
					raw, _ := io.ReadAll(resp.Body)
					require.Equal(t, http.StatusOK, resp.StatusCode,
						"%s %s rendered: %s", lang, path, raw[:min(len(raw), 3000)])
					want := byLang[lang][path]
					require.NotEmpty(t, want)
					require.Contains(t, string(raw), want,
						"%s %s must show the localized title %q", lang, path, want)
				})
			}
		})
	}
}

// TestCarePlanEditorsAllLocales renders the edit variants of one persisted
// matcher + rule in all four languages.
func TestCarePlanEditorsAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	matcher := &models.CareMatcher{
		ID:         uuid.Must(uuid.NewV4()),
		Name:       "ME-" + f.marker,
		Expression: `species = "CP-A1"`,
	}
	require.NoError(t, models.DB.Create(matcher))
	rule := f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	type wantRow struct {
		path string
		want string
	}
	byLang := map[string][]wantRow{
		"fr": {
			{"/care_rules/" + rule.ID.String() + "/edit", "Modifier la règle de soins"},
			{"/care_matchers/" + matcher.ID.String() + "/edit", "Modifier le sélecteur"},
		},
		"en-US": {
			{"/care_rules/" + rule.ID.String() + "/edit", "Edit care rule"},
			{"/care_matchers/" + matcher.ID.String() + "/edit", "Edit matcher"},
		},
		"de": {
			{"/care_rules/" + rule.ID.String() + "/edit", "Pflegeregel bearbeiten"},
			{"/care_matchers/" + matcher.ID.String() + "/edit", "Matcher bearbeiten"},
		},
		"nl": {
			{"/care_rules/" + rule.ID.String() + "/edit", "Zorgregel bewerken"},
			{"/care_matchers/" + matcher.ID.String() + "/edit", "Matcher bewerken"},
		},
	}

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		for _, row := range byLang[lang] {
			row := row
			t.Run(lang+row.path, func(t *testing.T) {
				req, err := http.NewRequest("GET", baseURL+row.path, nil)
				require.NoError(t, err)
				req.Header.Set("Accept", "text/html")
				req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
				resp, err := client.Do(req)
				require.NoError(t, err)
				raw, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				require.Equal(t, http.StatusOK, resp.StatusCode, "%s %s: %.300s", lang, row.path, raw)
				require.Contains(t, string(raw), row.want)
			})
		}
	}
}

// planTxForTests keeps the pop import anchored if the fixtures above ever
// move to a shared helper file.
var _ = func() *pop.Connection { return models.DB }
