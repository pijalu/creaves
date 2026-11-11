package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// bugs.md H3 / Phase 4: the legacy feeding schedule columns
// (feeding_start/end/period) are frozen. The animal form renders them
// read-only with a hint pointing to the animal Plan tab, and
// AnimalsResource.Update ignores any crafted AnimalFeeding* params so the
// freeze cannot be bypassed by POST.

// TestAnimalUpdateIgnoresLegacyFeedingParams posts the legacy feeding params
// with new values and asserts the columns keep their DB values.
func TestAnimalUpdateIgnoresLegacyFeedingParams(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	// Seed the legacy columns with known values.
	require.NoError(t, tx.RawQuery(
		"UPDATE animals SET feeding_start = '2000-01-01 08:00:00', feeding_end = '2000-01-01 18:00:00', feeding_period = 120 WHERE id = ?",
		f.freeID).Exec())

	editPath := fmt.Sprintf("/animals/%d/edit", f.freeID)
	token := todoToken(t, client, baseURL, editPath)

	reloadForm := func() url.Values {
		t.Helper()
		raw := parseFormFields(t, fetchPageGET(t, client, baseURL+editPath))
		form := url.Values{}
		for k, vs := range raw {
			if len(vs) > 0 && vs[0] != "" {
				form[k] = vs
			}
		}
		form.Set("_method", "PUT")
		return form
	}

	// Crafted POST: attempt to move the frozen columns.
	form := reloadForm()
	form.Set("AnimalFeedingStartTime", "06:30")
	form.Set("AnimalFeedingEndTime", "22:45")
	form.Set("AnimalFeedingPeriodHourMinute", "01:30")
	resp := postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "animal update failed: %s", truncate(b, 500))

	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	require.Equal(t, "08:00", a.FeedingStartFmt(), "feeding_start must stay frozen")
	require.Equal(t, "18:00", a.FeedingEndFmt(), "feeding_end must stay frozen")
	require.Equal(t, 120, a.FeedingPeriod, "feeding_period must stay frozen")
}

// TestAnimalFormLegacyFeedingFrozenAllLocales renders the animal edit form in
// every UI language: the legacy feeding schedule must be read-only (no
// AnimalFeeding* inputs, no mealCount JS) and — since BUG-13 — the form shows
// a summary of the ACTUAL protocol feeding plans (none here) and falls back to
// the localized hint linking to the animal Protocol tab (i18n key parity
// across the 4 locales). The frozen legacy times are NOT displayed any more
// (they rendered phantom defaults when the columns were NULL).
func TestAnimalFormLegacyFeedingFrozenAllLocales(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	require.NoError(t, tx.RawQuery(
		"UPDATE animals SET feeding_start = '2000-01-01 08:00:00', feeding_end = '2000-01-01 18:00:00', feeding_period = 120 WHERE id = ?",
		f.freeID).Exec())

	// Rendered form: plush HTML-escapes apostrophes (&#39;).
	wantHint := map[string]string{
		"fr":    "Le nourrissage se gère dans l&#39;onglet Protocole de l&#39;animal.",
		"en-US": "Feeding is managed in the animal&#39;s Protocol tab.",
		"de":    "Die Fütterung wird im Reiter „Protokoll“ des Tieres verwaltet.",
		"nl":    "De voeding wordt beheerd in het tabblad Protocol van het dier.",
	}

	editPath := fmt.Sprintf("/animals/%d/edit", f.freeID)
	for _, lang := range []string{"fr", "en-US", "de", "nl"} {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			req, err := http.NewRequest("GET", baseURL+editPath, nil)
			require.NoError(t, err)
			req.Header.Set("Accept", "text/html")
			req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
			resp, err := client.Do(req)
			require.NoError(t, err)
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			body := string(raw)
			require.Equal(t, http.StatusOK, resp.StatusCode,
				"%s %s rendered: %.500s", lang, editPath, body)

			require.Contains(t, body, wantHint[lang],
				"%s edit form must render the localized Protocol-tab hint", lang)
			require.Contains(t, body, fmt.Sprintf(`/animals/%d#nav-plan`, f.freeID),
				"%s hint must link to the animal Protocol tab", lang)
			// BUG-13: the frozen legacy schedule line is GONE (no plan exists
			// for this animal — a summary would render instead).
			require.NotContains(t, body, "feeding-plan-summary",
				"%s must not render a plan summary when no feeding plan exists", lang)

			// No editable inputs or meal-count JS for the legacy schedule.
			require.NotContains(t, body, `name="AnimalFeedingStartTime"`, "%s must not render an editable start input", lang)
			require.NotContains(t, body, `name="AnimalFeedingEndTime"`, "%s must not render an editable end input", lang)
			require.NotContains(t, body, `name="AnimalFeedingPeriodHourMinute"`, "%s must not render an editable period input", lang)
			require.NotContains(t, body, "mealCount", "%s must not render the mealCount JS", lang)
			require.NotContains(t, body, "calculatePeriodRepetitions", "%s must not render the meal-count calculator", lang)
		})
	}

	// New-animal form (unsaved animal, no Protocol tab yet): hint renders without
	// a link and without the legacy inputs.
	req, err := http.NewRequest("GET", baseURL+"/animals/new", nil)
	require.NoError(t, err)
	req.AddCookie(&http.Cookie{Name: "lang", Value: "fr"})
	resp, err := client.Do(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode, "/animals/new rendered: %.500s", raw)
	body := string(raw)
	require.Contains(t, body, "Le nourrissage se gère dans l&#39;onglet Protocole")
	require.NotContains(t, body, "#nav-plan", "new form has no Protocol tab to link to")
	require.NotContains(t, body, `name="AnimalFeedingStartTime"`)
	require.NotContains(t, body, `name="AnimalFeedingEndTime"`)
	require.NotContains(t, body, `name="AnimalFeedingPeriodHourMinute"`)
	require.NotContains(t, body, "mealCount")
}
