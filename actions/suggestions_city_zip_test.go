package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// bugs.md #9: stored discoverer.city values sometimes merge the zip code
// ("67000 Strasbourg"). Search must still match (and show the details), but
// what the endpoints return — and therefore what the forms fill — must be
// the clean city plus a separate postal code. Stored rows are not rewritten.
// ---------------------------------------------------------------------------

// TestSplitPostalCity covers the zip-merged-in-city cleanup helper.
func TestSplitPostalCity(t *testing.T) {
	cases := []struct {
		name          string
		postal, city  string
		wantPostal    string
		wantCity      string
	}{
		{"leading zip, empty postal", "", "67000 Strasbourg", "67000", "Strasbourg"},
		{"leading zip, existing postal kept", "67000", "67000 Strasbourg", "67000", "Strasbourg"},
		{"trailing zip", "", "Strasbourg 67000", "67000", "Strasbourg"},
		{"country-prefixed zip", "", "B-6700 Sankt Vith", "6700", "Sankt Vith"},
		{"underscore variant", "", "4280_Avin", "4280", "Avin"},
		{"underscore variant with trailing space", "", "1300_Wavre ", "1300", "Wavre"},
		{"already clean", "67000", "Strasbourg", "67000", "Strasbourg"},
		{"empty city", "", "", "", ""},
		{"city only", "", "Strasbourg", "", "Strasbourg"},
		{"whitespace trimmed", "  ", "  67000 Strasbourg  ", "67000", "Strasbourg"},
		{"short number not a zip", "", "Le 3 Mousquetaires", "", "Le 3 Mousquetaires"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPostal, gotCity := splitPostalCity(tc.postal, tc.city)
			assert.Equal(t, tc.wantPostal, gotPostal)
			assert.Equal(t, tc.wantCity, gotCity)
		})
	}
}

// seedMixedCityDiscoverer creates a discoverer whose stored city merges the
// zip, mirroring the polluted production data.
func seedMixedCityDiscoverer(t *testing.T, firstname, lastname, city string) *models.Discoverer {
	t.Helper()
	tx := searchTestDB(t)
	d := &models.Discoverer{
		ID:        uuid.Must(uuid.NewV4()),
		Firstname: nulls.NewString(firstname),
		Lastname:  nulls.NewString(lastname),
		City:      nulls.NewString(city),
		Email:     nulls.NewString("gaelle@example.org"),
		Note:      nulls.NewString("seeded note for fill check"),
	}
	require.NoError(t, tx.Create(d))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM discoverers WHERE id = ?", d.ID.String()).Exec()
	})
	return d
}

// TestDiscovererLookupMatchesMixedCity proves the reception-form discoverer
// picker finds a discoverer whose stored city is "67000 Strasbourg" when
// searching by name, by city and by zip — and that the returned entry always
// carries the clean city and a separate postal code.
func TestDiscovererLookupMatchesMixedCity(t *testing.T) {
	requireMySQLTestDB(t)
	d := seedMixedCityDiscoverer(t, "LookupFirst", "LookupLast", "67000 Strasbourg")

	client, baseURL := adminClientWithURL(t)

	for _, q := range []string{"LookupLast", "Strasbourg", "67000"} {
		t.Run("query="+q, func(t *testing.T) {
			resp, err := client.Get(baseURL + "/suggestions/discoverer_lookup?q=" + url.QueryEscape(q))
			require.NoError(t, err)
			defer resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			var entries []discovererLookupEntry
			require.NoError(t, json.Unmarshal(body, &entries))

			var found *discovererLookupEntry
			for i := range entries {
				if entries[i].ID == d.ID {
					found = &entries[i]
				}
			}
			require.NotNil(t, found, "discoverer must be returned for query %q", q)
			assert.Equal(t, "Strasbourg", found.City, "city must be clean")
			assert.Equal(t, "67000", found.PostalCode, "zip must move to the postal code")
			assert.Equal(t, "seeded note for fill check", found.Note, "note must round-trip so the form can fill every discoverer field")
			assert.Equal(t, "gaelle@example.org", found.Email, "email must round-trip")
			assert.NotContains(t, found.Label, "67000 67000", "label must not duplicate the zip")
			assert.Contains(t, found.Label, "Strasbourg", "label shows the details")
		})
	}
}

// TestSuggestionsDiscovererCityCleaned proves the discoverer-form city
// autocomplete matches raw stored values (a zip search finds the entry) but
// returns the cleaned, de-duplicated city names.
func TestSuggestionsDiscovererCityCleaned(t *testing.T) {
	requireMySQLTestDB(t)
	seedMixedCityDiscoverer(t, "CityFirst", "CityLast", "4000 Liège")

	client, baseURL := adminClientWithURL(t)

	// Search by zip: raw stored value matches, returned value is clean.
	resp, err := client.Get(baseURL + "/suggestions/discoverer_city?q=4000")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	var got []string
	require.NoError(t, json.Unmarshal(body, &got))
	assert.Contains(t, got, "Liège", "zip search must yield the clean city")
	for _, v := range got {
		assert.NotRegexp(t, `^\d{4,6} `, v, "no suggestion may contain a merged zip: %q", v)
	}

	// Search by city name: also matches the merged stored value.
	resp2, err := client.Get(baseURL + "/suggestions/discoverer_city?q=" + url.QueryEscape("Liège"))
	require.NoError(t, err)
	defer resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	body2, err := io.ReadAll(resp2.Body)
	require.NoError(t, err)
	var got2 []string
	require.NoError(t, json.Unmarshal(body2, &got2))
	assert.Contains(t, got2, "Liège")
}
