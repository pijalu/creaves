package actions

import (
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// TestReceptionCreateRejectsFutureDates: the reception form must refuse a
// discovery or intake date in the future (issue #208-12). The server rejects
// with a redirect back to /reception/new and persists nothing.
func TestReceptionCreateRejectsFutureDates(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	client, baseURL := adminClientWithURL(t)

	resp, err := client.Get(baseURL + "/reception/new")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET reception/new: %s", truncate(body, 500))
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no authenticity_token on /reception/new")

	// The flatpickr widgets must block future dates client-side too.
	require.Contains(t, string(body), "maxDate: new Date()")

	countAnimals := func() int {
		cnt, err := tx.Count(&models.Animal{})
		require.NoError(t, err)
		return cnt
	}

	future := time.Now().Add(48 * time.Hour).Format(models.DateTimeFormat)
	past := time.Now().Add(-24 * time.Hour).Format(models.DateTimeFormat)

	cases := []struct {
		name           string
		discoveryDate  string
		intakeDate     string
	}{
		{"future discovery date", future, past},
		{"future intake date", past, future},
		{"both future", future, future},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			before := countAnimals()
			form := url.Values{
				"AnimalCount":        {"1"},
				"Discovery.Date":     {tc.discoveryDate},
				"Intake.Date":        {tc.intakeDate},
				"authenticity_token": {string(m[1])},
			}
			// Disable redirect following to inspect the response directly.
			noRedirect := *client
			noRedirect.CheckRedirect = func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			}
			resp, err := noRedirect.PostForm(baseURL+"/animals/", form)
			require.NoError(t, err)
			rbody, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.Equal(t, http.StatusSeeOther, resp.StatusCode, "body: %s", truncate(rbody, 500))
			require.Equal(t, "/reception/new", resp.Header.Get("Location"))
			require.Equal(t, before, countAnimals(), "no animal must be created for %s", tc.name)
		})
	}
}
