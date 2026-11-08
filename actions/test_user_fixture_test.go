package actions

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// testUserFixture creates (and registers cleanup for) a user fixture with
// the given admin flag. Requires GO_ENV=test so models.DB points at
// creaves_test. (Formerly feedingGuideUser; renamed when the feeding-guide
// feature was removed — #208.1.)
func testUserFixture(t *testing.T, admin bool) (login, password string) {
	t.Helper()
	requireMySQLTestDB(t)
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	login = "tufix_" + uuid.Must(uuid.NewV4()).String()[:8]
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

// testUserLogin performs the form login dance (CSRF + session cookie)
// against a fresh app server and returns the authenticated client.
// (Formerly feedingGuideLogin.)
func testUserLogin(t *testing.T, login, password string) (*http.Client, string) {
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
