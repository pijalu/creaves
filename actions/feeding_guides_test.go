package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// feedingGuideUser creates (and registers cleanup for) a user fixture with
// the given role. Requires GO_ENV=test so models.DB points at creaves_test.
func feedingGuideUser(t *testing.T, admin bool) (login, password string) {
	t.Helper()
	requireMySQLTestDB(t)
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	login = "fgtest_" + uuid.Must(uuid.NewV4()).String()[:8]
	password = "fgpass123"
	u := &models.User{Login: login, Admin: admin, Approved: true}
	u.Password = password
	u.PasswordConfirmation = password
	if _, err := u.Create(models.DB); err != nil {
		t.Fatalf("failed to create user fixture: %v", err)
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})
	return login, password
}

// feedingGuideLogin performs the form login dance (CSRF + session cookie)
// against a fresh app server and returns the authenticated client.
func feedingGuideLogin(t *testing.T, login, password string) (*http.Client, string) {
	t.Helper()

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := srv.Client()
	client.Jar = jar
	// Keep redirects unfollowed so POST /auth returning 302 stops there.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := client.Get(srv.URL + "/auth/new")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no authenticity_token on /auth/new")

	vals := url.Values{}
	vals.Set("Login", login)
	vals.Set("Password", password)
	vals.Set("authenticity_token", string(m[1]))
	resp, err = client.PostForm(srv.URL+"/auth/", vals)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "login POST /auth/")

	return client, srv.URL
}

// feedingGuideToken fetches the guides page as an admin and extracts a CSRF
// token for subsequent POSTs.
func feedingGuideToken(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	resp, err := client.Get(baseURL + "/feeding_guides")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET /feeding_guides")
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no authenticity_token on /feeding_guides")
	return string(m[1])
}

func TestFeedingGuidesAdminOnly(t *testing.T) {
	// Non-admin: the page must refuse with 403.
	login, password := feedingGuideUser(t, false)
	client, baseURL := feedingGuideLogin(t, login, password)

	resp, err := client.Get(baseURL + "/feeding_guides")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, resp.StatusCode, "non-admin GET /feeding_guides: %s", truncateStr145(body, 300))

	// Admin: 200 with the management UI.
	alogin, apassword := feedingGuideUser(t, true)
	aclient, abaseURL := feedingGuideLogin(t, alogin, apassword)
	resp, err = aclient.Get(abaseURL + "/feeding_guides")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "Feeding guides")
}

func TestFeedingGuidesCRUDRoundtrip(t *testing.T) {
	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	token := feedingGuideToken(t, client, baseURL)

	// Create
	form := url.Values{
		"SpeciesName":        {"TestSpecies145"},
		"Stage":              {models.FeedingStageAdult},
		"Text":               {"Adult diet for tests"},
		"authenticity_token": {token},
	}
	resp, err := client.PostForm(baseURL+"/feeding_guides", form)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "create should redirect")

	g := &models.FeedingGuide{}
	require.NoError(t, models.DB.Where("species_name = ? AND stage = ?", "TestSpecies145", models.FeedingStageAdult).First(g))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM feeding_guides WHERE species_name = ?", "TestSpecies145").Exec()
	})
	require.Equal(t, "Adult diet for tests", g.Text)

	// Duplicate rejected: still exactly one row.
	resp, err = client.PostForm(baseURL+"/feeding_guides", form)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	count, err := models.DB.Where("species_name = ?", "TestSpecies145").Count(&models.FeedingGuide{})
	require.NoError(t, err)
	require.Equal(t, 1, count, "duplicate must not create a second row")

	// Update
	token = feedingGuideToken(t, client, baseURL)
	form = url.Values{
		"ID":                 {strconv.Itoa(g.ID)},
		"SpeciesName":        {"TestSpecies145"},
		"Stage":              {models.FeedingStageJuvenile},
		"Text":               {"Juvenile diet for tests"},
		"authenticity_token": {token},
	}
	resp, err = client.PostForm(baseURL+"/feeding_guides", form)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)

	updated := &models.FeedingGuide{}
	require.NoError(t, models.DB.Find(updated, g.ID))
	require.Equal(t, models.FeedingStageJuvenile, updated.Stage)
	require.Equal(t, "Juvenile diet for tests", updated.Text)

	// Destroy
	token = feedingGuideToken(t, client, baseURL)
	form = url.Values{"authenticity_token": {token}}
	resp, err = client.PostForm(baseURL+"/feeding_guides/"+strconv.Itoa(g.ID)+"/delete", form)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)

	count, err = models.DB.Where("species_name = ?", "TestSpecies145").Count(&models.FeedingGuide{})
	require.NoError(t, err)
	require.Equal(t, 0, count, "destroy must remove the row")
}

func TestSuggestionsFeedingGuide(t *testing.T) {
	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	g := &models.FeedingGuide{
		SpeciesName: "TestSpecies145Sugg",
		Stage:       models.FeedingStageBaby,
		Text:        "Baby diet for suggestions",
	}
	require.NoError(t, models.DB.Create(g))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM feeding_guides WHERE species_name = ?", "TestSpecies145Sugg").Exec()
	})

	resp, err := client.Get(baseURL + "/suggestions/feeding_guide?species=TestSpecies145Sugg&stage=" + url.QueryEscape(models.FeedingStageBaby))
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got map[string]string
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "Baby diet for suggestions", got["text"])

	// Unknown species → empty text, still 200.
	resp, err = client.Get(baseURL + "/suggestions/feeding_guide?species=NoSuchSpecies&stage=" + url.QueryEscape(models.FeedingStageBaby))
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got = map[string]string{}
	require.NoError(t, json.Unmarshal(body, &got))
	require.Equal(t, "", got["text"])
}

// truncateStr145 caps error output; named to avoid clashing with the
// package-level truncate helper in outtakes_rules_test.go.
func truncateStr145(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n])
}
