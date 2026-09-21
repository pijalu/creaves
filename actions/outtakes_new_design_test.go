package actions

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// getOuttakesNew fetches /outtakes/new with an optional lang cookie and
// returns the status code and body.
func getOuttakesNew(t *testing.T, client *http.Client, baseURL, lang string) (int, string) {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+"/outtakes/new", nil)
	require.NoError(t, err)
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// TestOuttakesNewPickerDesign pins the /outtakes/new selection page layout
// (#199-16): it mirrors /cares/new — an inline animal picker (GET back to
// /outtakes/new) and an inline cage picker (GET to the cage batch flow
// /outtakes/cage), both localized through t() in all four languages.
func TestOuttakesNewPickerDesign(t *testing.T) {
	client, baseURL := adminClientWithURL(t)

	code, body := getOuttakesNew(t, client, baseURL, "")
	require.Equal(t, http.StatusOK, code, "GET /outtakes/new: %s", truncate([]byte(body), 500))

	// Both pickers present, mirroring cares/new: animal form GETs back to
	// /outtakes/new, cage form GETs the cage batch flow.
	require.Contains(t, body, `action="/outtakes/new/"`, "animal picker form missing")
	require.Contains(t, body, `action="/outtakes/cage"`, "cage picker form missing")
	require.Contains(t, body, `name="animal_year_number"`, "animal number input missing")
	require.Contains(t, body, `name="cage"`, "cage input missing")

	// EN headings (terminology aligned on "Outcome", see outtakes/index).
	require.Contains(t, body, "Select animal for outcome")
	require.Contains(t, body, "Select cage for outcome")

	// FR headings — the wording quoted in issue #199-16.
	code, body = getOuttakesNew(t, client, baseURL, "fr")
	require.Equal(t, http.StatusOK, code)
	// t() output is HTML-escaped: apostrophes render as &#39;.
	require.Contains(t, body, "Sélectionnez l&#39;animal pour l&#39;issue")
	require.Contains(t, body, "Sélectionnez la cage pour l&#39;issue")
	// Localized form partial, not the EN base one.
	require.Contains(t, body, `action="/outtakes/cage"`)

	// DE + NL render (t() keys must exist in every locale).
	for _, lang := range []string{"de", "nl"} {
		code, body = getOuttakesNew(t, client, baseURL, lang)
		require.Equal(t, http.StatusOK, code, "lang %s", lang)
		require.NotContains(t, body, "outtakes.new.select_animal", "untranslated key for lang %s", lang)
	}
}

// TestOuttakesNewPickerNotShownWithAnimal: once an animal is picked the
// selection forms disappear and the outcome form header is shown.
func TestOuttakesNewPickerNotShownWithAnimal(t *testing.T) {
	client, baseURL := adminClientWithURL(t)
	tx := searchTestDB(t)
	f := createOuttakeRulesFixture(t, tx, "", models.OuttakeLocationModeFree)

	resp, err := client.Get(baseURL + "/outtakes/new?animal_year_number=" +
		strconv.Itoa(f.animalYearNumber) + "/" + strconv.Itoa(f.animalYear%100))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET new with animal: %s", truncate(body, 500))
	s := string(body)
	require.False(t, strings.Contains(s, "Select animal for outcome"), "picker still visible with animal")
	require.Contains(t, s, "Outcome of animal")
}
