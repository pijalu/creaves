package actions

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGuardTestApp builds a minimal Buffalo app injecting the shared MySQL
// test database and an optional current_user, then registers the given
// handler on a GET path. JSON requests are used so the handlers respond
// through responder.Wants("json") without needing the template engine.
func newGuardTestApp(t *testing.T, u *models.User, path string, h buffalo.Handler) *buffalo.App {
	t.Helper()
	tx := searchTestDB(t)
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			c.Set("tx", tx)
			if u != nil {
				c.Set("current_user", u)
			}
			return next(c)
		}
	})
	a.GET(path, h)
	return a
}

func newJSONGet(path string) *http.Request {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Accept", "application/json")
	return req
}

// TestAnimaltypeAndOuttaketypeMaintainerOnly pins the access rule: the
// animal-types and outtake-types reference tables are maintainer-only
// surfaces. A plain admin (admin without the maintainer flag) must get 403
// on the list handlers; a maintainer gets 200.
func TestAnimaltypeAndOuttaketypeMaintainerOnly(t *testing.T) {
	plainAdmin := &models.User{Login: "guard_plain_admin", Admin: true, Approved: true}
	maintainer := &models.User{Login: "guard_maintainer", Admin: true, Maintainer: true, Approved: true}

	for _, tc := range []struct {
		name    string
		handler buffalo.Handler
		path    string
	}{
		{"animaltypes", AnimaltypesResource{}.List, "/animaltypes/"},
		{"outtaketypes", OuttaketypesResource{}.List, "/outtaketypes/"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := newGuardTestApp(t, plainAdmin, tc.path, tc.handler)
			w := httptest.NewRecorder()
			a.ServeHTTP(w, newJSONGet(tc.path))
			require.Equal(t, http.StatusForbidden, w.Code,
				"plain admin must get 403 on %s (got %d)", tc.path, w.Code)

			a2 := newGuardTestApp(t, maintainer, tc.path, tc.handler)
			w2 := httptest.NewRecorder()
			a2.ServeHTTP(w2, newJSONGet(tc.path))
			require.Equal(t, http.StatusOK, w2.Code,
				"maintainer must get 200 on %s (got %d)", tc.path, w2.Code)
		})
	}
}

// decodeJSONList decodes a JSON array response into a slice of Species.
func decodeSpeciesList(t *testing.T, w *httptest.ResponseRecorder) []models.Species {
	t.Helper()
	var got []models.Species
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	return got
}

// TestSpeciesListSearchFilter proves the species index q filter narrows the
// listing: matching a canonical column returns only matching rows, and a
// token that exists only as a translation value still finds the species.
func TestSpeciesListSearchFilter(t *testing.T) {
	maintainer := &models.User{Login: "guard_maintainer", Admin: true, Maintainer: true, Approved: true}
	a := newGuardTestApp(t, maintainer, "/species/", SpeciesResource{}.List)
	tx := searchTestDB(t)

	// Self-contained fixtures (uuid-scoped, cleaned up) so the test does
	// not depend on pre-existing reference data.
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	famA := "FamA-" + marker
	famB := "FamB-" + marker
	atID := uuid.Must(uuid.NewV4())
	at := models.Animaltype{ID: atID, Name: "Search type " + marker}
	require.NoError(t, tx.Create(&at))
	spA := models.Species{ID: "ssearch-a-" + marker, Species: "Alpha " + marker, Class: "Mammalia", Order: "OrdA-" + marker, Family: famA, CreavesSpecies: "ALPHA-" + marker, SubsideGroup: "g", AgwGroup: "g", NativeStatus: "n", AnimaltypeID: &atID}
	spB := models.Species{ID: "ssearch-b-" + marker, Species: "Beta " + marker, Class: "Aves", Order: "OrdB-" + marker, Family: famA, CreavesSpecies: "BETA-" + marker, SubsideGroup: "g", AgwGroup: "g", NativeStatus: "n", AnimaltypeID: &atID}
	spC := models.Species{ID: "ssearch-c-" + marker, Species: "Gamma " + marker, Class: "Aves", Order: "OrdB-" + marker, Family: famB, CreavesSpecies: "GAMMA-" + marker, SubsideGroup: "g", AgwGroup: "g", NativeStatus: "n", AnimaltypeID: &atID}
	for _, sp := range []models.Species{spA, spB, spC} {
		s := sp
		require.NoError(t, tx.Create(&s))
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE id IN (?, ?, ?)", spA.ID, spB.ID, spC.ID).Exec()
		tx.RawQuery("DELETE FROM translations WHERE table_name = 'species' AND record_id = ?", spC.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", atID).Exec()
	})

	fetch := func(q string) []models.Species {
		t.Helper()
		w := httptest.NewRecorder()
		a.ServeHTTP(w, newJSONGet("/species/?per_page=1000&q="+url.QueryEscape(q)))
		require.Equal(t, http.StatusOK, w.Code)
		return decodeSpeciesList(t, w)
	}
	ids := func(got []models.Species) []string {
		out := make([]string, 0, len(got))
		for _, s := range got {
			out = append(out, s.ID)
		}
		return out
	}

	// Canonical family match returns both species of that family only.
	assert.ElementsMatch(t, []string{spA.ID, spB.ID}, ids(fetch(famA)))

	// Canonical name match returns exactly that species.
	assert.ElementsMatch(t, []string{spA.ID}, ids(fetch("Alpha "+marker)))

	// Translation-only match: a token stored only in translations (table
	// 'species') must find the species whose canonical columns do not
	// contain the token.
	const tokenPrefix = "zzspecsearch-"
	token := tokenPrefix + marker
	trID := uuid.Must(uuid.NewV4()).String()
	require.NoError(t, tx.RawQuery(
		"INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES (?, 'species', ?, 'species', 'en-US', ?, NOW(), NOW())",
		trID, spC.ID, token,
	).Exec())
	assert.ElementsMatch(t, []string{spC.ID}, ids(fetch(token)))

	// No-match token returns an empty list.
	assert.Empty(t, fetch("zznomatch-" + marker))
}
