package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"

	"creaves/models"
)

// TestAnimalHeatO2ChangeCreatesSoinsCare: editing the HeatSource or Oxygen
// fields from the animal sheet must create a "soin"-typed care, mirroring
// the cage-move behavior (#205 item 10).
// Issue: https://github.com/pijalu/creaves/issues/205
func TestAnimalHeatO2ChangeCreatesSoinsCare(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	// The canonical "Soin" care type must be preferred by the auto-care;
	// csTSoins creates it when the test DB lacks it (hermetic).
	soinsID := csTSoins(t, tx)

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

	// Change both watched fields in one save.
	form := reloadForm()
	form.Set("HeatSource", "Lampe 40W")
	form.Set("Oxygen", "true")
	resp := postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "animal update failed: %s", truncate(b, 500))

	cares := models.Cares{}
	require.NoError(t, tx.Where("animal_id = ?", f.freeID).All(&cares))
	require.NotEmpty(t, cares)
	var found *models.Care
	for i := range cares {
		c := cares[i]
		if c.Note.Valid && c.Note.String != "" && c.TypeID == soinsID {
			found = &cares[i]
			break
		}
	}
	require.NotNil(t, found, "no soins-typed care created on heat/O2 change; cares=%+v", cares)
	require.Contains(t, found.Note.String, "Source de chaleur")
	require.Contains(t, found.Note.String, "Lampe 40W")
	require.Contains(t, found.Note.String, "Oxygène")
	require.Contains(t, found.Note.String, "oui")

	// Second update with no heat/O2 change must NOT create another care.
	token = todoToken(t, client, baseURL, editPath)
	form = reloadForm()
	// The checked O2 checkbox submits its hidden 'false' plus 'true'; the
	// binder keeps the last value. parseFormFields collapses to the first,
	// so re-assert the checked state explicitly.
	form.Set("Oxygen", "true")
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	count := 0
	cares2 := models.Cares{}
	require.NoError(t, tx.Where("animal_id = ?", f.freeID).All(&cares2))
	for _, c := range cares2 {
		if c.TypeID == soinsID {
			count++
		}
	}
	require.Equal(t, 1, count, "no-op update must not create an extra soins care")
}
