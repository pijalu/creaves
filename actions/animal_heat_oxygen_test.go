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

// TestAnimalHeatSourceOxygenOnAnimal (#199-17): heat source and O2 attach to
// the animal (below the feeding-schedule options on the edit form), persist
// through the update flow and are displayed on the animal sheet read mode.
// The care form no longer carries those inputs; historical care values stay
// in the DB untouched.
func TestAnimalHeatSourceOxygenOnAnimal(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	editPath := fmt.Sprintf("/animals/%d/edit", f.freeID)
	editURL := baseURL + editPath
	token := todoToken(t, client, baseURL, editPath)

	// 1. The animal edit form carries the fields (care tab), the care form
	//    does not.
	editHTML := fetchPageGET(t, client, editURL)
	require.Contains(t, editHTML, `name="HeatSource"`, "animal form must carry heat source")
	require.Contains(t, editHTML, `name="Oxygen"`, "animal form must carry oxygen")

	reloadForm := func() url.Values {
		t.Helper()
		raw := parseFormFields(t, fetchPageGET(t, client, editURL))
		// Drop empty keys (see TestAnimalReadyForReleaseFlag: cached
		// reference selects render without a selected option).
		form := url.Values{}
		for k, vs := range raw {
			if len(vs) > 0 && vs[0] != "" {
				form[k] = vs
			}
		}
		form.Set("_method", "PUT")
		return form
	}

	// 2. Set heat source + oxygen on the animal.
	form := reloadForm()
	form.Set("HeatSource", "Lampe infrarouge e2e")
	form.Set("Oxygen", "true")
	form.Add("Oxygen", "false")
	resp := postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "animal update failed: %s", truncate(b, 500))

	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	require.True(t, a.HeatSource.Valid)
	require.Equal(t, "Lampe infrarouge e2e", a.HeatSource.String)
	require.True(t, a.Oxygen)

	// 3. Read mode displays both (pinned to the care-tab list markup: the
	//    audit table also mentions the raw value after a change).
	showHTML := fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d", f.freeID))
	require.Contains(t, showHTML, `<label class="small d-block">Heat source</label>`)
	require.Contains(t, showHTML, `<p class="d-inline-block">Lampe infrarouge e2e</p>`)
	require.Contains(t, showHTML, `<label class="small d-block">Oxygen</label>`)

	// 4. Clear them again (heat source emptied, oxygen unchecked).
	form = reloadForm()
	form.Set("HeatSource", "")
	form.Set("Oxygen", "false")
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "animal update (clear) failed")

	a = models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	require.False(t, a.HeatSource.Valid && a.HeatSource.String != "", "heat source must be cleared")
	require.False(t, a.Oxygen, "oxygen must be cleared")

	showHTML = fetchPageGET(t, client, baseURL+fmt.Sprintf("/animals/%d", f.freeID))
	require.NotContains(t, showHTML, `<label class="small d-block">Heat source</label>`, "read mode must hide cleared heat source")
	require.NotContains(t, showHTML, `<label class="small d-block">Oxygen</label>`, "read mode must hide cleared oxygen")
}

// TestCareFormHasNoHeatSourceOxygen (#199-17): the care entry form no longer
// exposes the heat source / O2 inputs — they moved to the animal.
func TestCareFormHasNoHeatSourceOxygen(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	a := models.Animal{}
	require.NoError(t, tx.Eager().Find(&a, f.freeID))

	careFormHTML := fetchPageGET(t, client, baseURL+fmt.Sprintf("/cares/new?animal_year_number=%d/%02d", a.YearNumber, a.Year%100))
	require.NotContains(t, careFormHTML, `name="HeatSource"`, "care form must not carry heat source anymore")
	require.NotContains(t, careFormHTML, `name="Oxygen"`, "care form must not carry oxygen anymore")
}
