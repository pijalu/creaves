package actions

import (
	"fmt"
	"html"
	"io"
	"net/http"
	"strings"
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
			animalPath:           "Protocoles de cet animal",
			"/landing/index":     "Plan du jour",
		},
		"en-US": {
			"/care_plan":         "Day plan",
			"/care_rules":        "Care rules",
			"/care_rules/new":    "New care rule",
			"/care_matchers":     "Matcher library",
			"/care_matchers/new": "New matcher",
			animalPath:           "Protocols for this animal",
			"/landing/index":     "Day plan",
		},
		"de": {
			"/care_plan":         "Tagesplan",
			"/care_rules":        "Pflegeregeln",
			"/care_rules/new":    "Neue Pflegeregel",
			"/care_matchers":     "Matcher-Bibliothek",
			"/care_matchers/new": "Neuer Matcher",
			animalPath:           "Protokolle dieses Tieres",
			"/landing/index":     "Tagesplan",
		},
		"nl": {
			"/care_plan":         "Dagplan",
			"/care_rules":        "Zorgregels",
			"/care_rules/new":    "Nieuwe zorgregel",
			"/care_matchers":     "Matcherbibliotheek",
			"/care_matchers/new": "Nieuwe matcher",
			animalPath:           "Protocollen van dit dier",
			"/landing/index":     "Dagplan",
		},
	}

	// R5-4a (U20/D-d): the animal-page protocol labels — tab + modal title
	// buttons — renamed from "plan" to "protocol" (UI values only). Asserted
	// explicitly per language on the animal page HTML.
	protocolLabels := map[string][]string{
		"fr":    {"Protocole", "Nouveau protocole", "Modifier le protocole"},
		"en-US": {"Protocol", "New protocol", "Edit protocol"},
		"de":    {"Protokoll", "Neues Protokoll", "Protokoll bearbeiten"},
		"nl":    {"Protocol", "Nieuw protocol", "Protocol bewerken"},
	}

	// §10-B6 dialog strings on the day-plan page (fixture renders a due,
	// applicable FEEDING item → bugs.md U1 feeding card with the apply-group
	// button + dosage modal).
	applyStrings := map[string][]string{
		"fr":    {"Appliquer le groupe", "Le dosage ne peut pas être calculé automatiquement", "Dernier poids enregistré", "Appliquer avec ce dosage"},
		"en-US": {"Apply group", "Dosage cannot be computed automatically", "Last recorded weight", "Apply with this dosage"},
		"de":    {"Gruppe anwenden", "Die Dosierung kann nicht automatisch berechnet werden", "Zuletzt erfasstes Gewicht", "Mit dieser Dosierung anwenden"},
		"nl":    {"Groep toepassen", "De dosering kan niet automatisch worden berekend", "Laatst geregistreerde gewicht", "Met deze dosering toepassen"},
	}

	// planUXStrings — new-UX labels that must appear in the static care_plan
	// HTML for every language: zone-dropdown label (round-2 §7.2 IA — the
	// kind-filter chips are gone) + confirm/detail/error modal titles
	// (mirrors locales/care_plan.<lang>.yaml). R5-2c (D-a) removed the
	// undo modal — undo is an instant toggle, so its title is gone.
	planUXStrings := map[string][]string{
		"fr":    {"Zone", "Confirmer l'application", "Détails de l'entrée", "Une erreur est survenue"},
		"en-US": {"Zone", "Confirm application", "Entry details", "Something went wrong"},
		"de":    {"Zone", "Anwendung bestätigen", "Eintragsdetails", "Etwas ist schiefgelaufen"},
		"nl":    {"Zone", "Toepassing bevestigen", "Details van de invoer", "Er is iets misgegaan"},
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
					// WP7 gate: no missing-key marker may reach ANY care-plan
					// surface (a missing fork key renders this marker).
					require.NotContains(t, string(raw), "translation missing",
						"%s %s must not render missing-key markers", lang, path)
					want := byLang[lang][path]
					require.NotEmpty(t, want)
					require.Contains(t, string(raw), want,
						"%s %s must show the localized title %q", lang, path, want)
					if path == animalPath {
						// R5-4a (U20): protocol labels on the animal page.
						for _, s := range protocolLabels[lang] {
							require.Contains(t, string(raw), s,
								"%s %s must render the protocol label %q", lang, path, s)
						}
						// The old wording must be gone (values-only rename, keys
						// untouched — "Nouveau plan" etc. must not render).
						for _, s := range []string{"New plan", "Nouveau plan", "Neuer Plan", "Nieuw plan",
							"Plans for this animal", "Plans de cet animal", "Pläne dieses Tieres", "Plannen van dit dier"} {
							require.NotContains(t, string(raw), s,
								"%s %s must not render the legacy plan label %q", lang, path, s)
						}
					}
					if path == "/care_plan" {
						for _, s := range applyStrings[lang] {
							require.Contains(t, string(raw), s,
								"%s /care_plan must render the §10-B6 apply string %q", lang, s)
						}
						// New-UX strings always present in the static template
						// (kind filter chip, confirm / detail / error modal
						// labels). Slot labels only render when medication items
						// exist, so they are not asserted here.
						for _, s := range planUXStrings[lang] {
							// Plush HTML-escapes apostrophes (l&#39;...).
							esc := strings.ReplaceAll(s, "'", "&#39;")
							require.Contains(t, string(raw), esc,
								"%s /care_plan must render the new-UX string %q", lang, s)
						}
					}
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

// TestDashboardMedicationSectionAllLocales renders /dashboard/ in all four
// UI languages and asserts the "Medication today" section (bugs.md R5-2b):
// t()-driven heading, the original table shape (Number|Cage|Species|
// Treatments — localized headers ×4), one row per animal and NO card grid.
func TestDashboardMedicationSectionAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	// One medication slot due earlier today (R5-2a today-only window):
	// without it the dashboard table renders zero rows and the row-level
	// assertions below would pass vacuously.
	now := time.Now()
	due := now.Add(-time.Minute)
	if due.Day() != now.Day() {
		due = now // midnight edge: keep the slot inside today
	}
	med := ruleWithoutMatcher(t, models.DB, "RMED-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))
	_ = med // cleanup via the fixture's marker-tagged care_rules delete
	client, baseURL := planAdminClient(t)

	wantHeading := map[string]string{
		"fr":    "Médication aujourd'hui",
		"en-US": "Medication today",
		"de":    "Medikation heute",
		"nl":    "Medicatie vandaag",
	}
	// Localized table headers (R5-2b): Number | Cage | Species | Treatments.
	wantCols := map[string][]string{
		"fr":    {"N°", "Cage", "Espèce", "Traitements"},
		"en-US": {"Number", "Cage", "Species", "Treatments"},
		"de":    {"Nr.", "Käfig", "Art", "Behandlungen"},
		"nl":    {"Nr.", "Kooi", "Soort", "Behandelingen"},
	}
	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+"/dashboard/", nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			rawBytes, _ := io.ReadAll(resp.Body)
			raw := string(rawBytes)
			// t() output is HTML-escaped (FR "aujourd&#39;hui") — compare
			// on the unescaped text.
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"%s /dashboard/ rendered: %s", lang, raw[:min(len(raw), 2000)])
			// WP7 gate: no missing-key marker on the dashboard either.
			require.NotContains(t, string(raw), "translation missing",
				"%s /dashboard/ must not render missing-key markers", lang)
			unescaped := html.UnescapeString(raw)
			require.Contains(t, unescaped, wantHeading[lang],
				"%s /dashboard/ must show the localized medication heading %q", lang, wantHeading[lang])
			require.NotContains(t, string(raw), "animalsToTreat",
				"%s /dashboard/ must not render the old animals-to-treat table", lang)
			// R5-2b: the original table shape — one row per animal, no cards
			for _, col := range wantCols[lang] {
				require.Contains(t, string(raw), ">"+col+"</th>",
					"%s /dashboard/ medication table must carry the localized %q header", lang, col)
			}
			require.Contains(t, string(raw), "plan-med-row",
				"%s /dashboard/ must render one table row per animal", lang)
			require.NotContains(t, string(raw), "plan-med-card",
				"%s /dashboard/ must not render the old card grid", lang)
			// slot buttons show the actual due time (any number of slots),
			// not the 3 fixed bucket names; the shared drug-series partial
			// renders one line per (drug, dosage) — Dash-3 removed the
			// per-row count badge (Dash-1/4/5/8 markup, round-2 §8.1).
			require.Contains(t, string(raw), "plan-med-slot", "medication toggle markup present")
			require.Contains(t, string(raw), "med-series-line", "drug series line present")
			require.NotContains(t, string(raw), "dash-med-count",
				"%s /dashboard/ must not render the per-row count badge (Dash-3)", lang)
			// Dash-7 (round-2 §8.2): the eye deep-links the animal page with
			// the occurrence reference (?item=&due=) + the treatment-tab hash.
			require.Contains(t, string(raw), "dash-med-view",
				"%s /dashboard/ must carry the unconditional view link", lang)
			require.Contains(t, string(raw), "item=",
				"%s /dashboard/ eye must deep-link the occurrence (item=)", lang)
			require.Contains(t, string(raw), "#nav-treatment",
				"%s /dashboard/ eye must target the treatment tab", lang)
		})
	}
}
// TestAnimalShowPlanTodayRowsAllLocales renders one animal's show page
// (the Treatment tab source, bugs.md U26 — fix 7) in all four UI
// languages and asserts the merged accordion: the dedicated Today block
// is gone, today's plan occurrence renders as an original-look accordion
// row (payload label + protocol backlink + clock badge), and NO missing-
// key markers appear anywhere on the page (the WP7 render gate for the
// animal surface — the /care_plan and /dashboard gates live in the tests
// above).
func TestAnimalShowPlanTodayRowsAllLocales(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	// A legacy treatment dated today — without it the accordion has no
	// current-date card and the plan row would render nowhere. The app
	// stores treatment dates as UTC midnight (time.Parse of "2006-01-02"),
	// so mimic that or the card lands on the previous day.
	today := time.Now().UTC().Truncate(24 * time.Hour)
	legacyTr := &models.Treatment{
		Date: today, AnimalID: f.animalIDs[0],
		Drug: "LegacyTr-" + f.marker, Dosage: "1 ml",
	}
	require.NoError(t, models.DB.Create(legacyTr))

	// One medication occurrence due (earlier) today — without it no plan
	// row would render and the assertions below would pass vacuously.
	now := time.Now()
	due := now.Add(-time.Minute)
	if due.Day() != now.Day() {
		due = now // midnight edge: keep the occurrence inside today
	}
	_ = ruleWithoutMatcher(t, models.DB, "ATODAY-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "ATDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))

	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+fmt.Sprintf("/animals/%d", f.animalIDs[0]), nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
			resp, err := client.Do(req)
			require.NoError(t, err)
			defer resp.Body.Close()
			raw, _ := io.ReadAll(resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"%s animal show rendered: %.2000s", lang, raw)
			unescaped := html.UnescapeString(string(raw))
			// U26 (fix 7): the dedicated Today block is gone.
			require.NotContains(t, unescaped, "animalTodayBlock",
				"%s animal show must not render the old TODAY block", lang)
			require.NotContains(t, string(raw), "plan-med-slot",
				"%s animal show must not render series toggle buttons", lang)
			// The plan occurrence merges into the accordion as an
			// original-look row: payload label + due-hour clock badge.
			require.Contains(t, unescaped, "ATDrug-"+f.marker,
				"%s animal show must render the plan row label", lang)
			require.Contains(t, unescaped, due.Format("15:04")+" \u00b7 ",
				"%s animal show must render the clock badge title", lang)
			// Protocol backlink: rule sources open the care-rules library.
			require.Contains(t, unescaped, "ATODAY-"+f.marker,
				"%s animal show must link the source rule", lang)
			require.Contains(t, string(raw), `href="/care_rules"`,
				"%s animal show plan row must target the rules library", lang)
			// WP7 gate: no missing-key markers, and the today key itself
			// must never render raw (a raw render means the fork lost it).
			require.NotContains(t, string(raw), "translation missing",
				"%s animal show must not render missing-key markers", lang)
			require.NotContains(t, string(raw), "care_plan.section.today",
				"%s animal show must not render the raw today key", lang)
		})
	}
}
