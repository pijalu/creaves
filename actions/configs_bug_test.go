package actions

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
)

// ---------------------------------------------------------------------------
// Bug: the config index offered a delete button for the currently active
// configuration, even though Destroy always refuses it ("cannot delete the
// currently active configuration"). The button must not be rendered for the
// active config; it stays available for inactive ones.
//
// These tests drive the full App() (real layout, real routes) with an admin
// session obtained through the actual login flow. They need GO_ENV=test so
// models.DB points at the creaves_test MySQL database.
// ---------------------------------------------------------------------------

// csrfTokenRe extracts the CSRF token from the layout meta tag or a form's
// hidden authenticity_token input.
var csrfTokenRe = regexp.MustCompile(`(?:name="csrf-token" content=|name="authenticity_token"[^>]*value=)"([^"]+)"`)

// adminClient creates an approved admin user in models.DB, logs in through
// the real /auth flow and returns an http.Client carrying the session cookie.
// The user row is removed at test cleanup.
func adminClient(t *testing.T) *http.Client {
	t.Helper()
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	login := "cfgtest_admin_" + uuid.Must(uuid.NewV4()).String()[:8]
	password := "cfgpass123"
	u := &models.User{Login: login, Admin: true, Approved: true}
	u.Password = password
	u.PasswordConfirmation = password
	if _, err := u.Create(models.DB); err != nil {
		t.Fatalf("failed to create admin fixture: %v", err)
	}
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)

	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("cookiejar: %v", err)
	}
	client := srv.Client()
	client.Jar = jar
	// Keep redirects unfollowed so POST /auth returning 302 stops there.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	// GET the login form to obtain a session cookie + CSRF token.
	resp, err := client.Get(srv.URL + "/auth/new")
	if err != nil {
		t.Fatalf("GET /auth/new: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read /auth/new body: %v", err)
	}
	m := csrfTokenRe.FindSubmatch(body)
	if m == nil {
		t.Fatal("no authenticity_token on /auth/new")
	}

	vals := url.Values{}
	vals.Set("Login", login)
	vals.Set("Password", password)
	vals.Set("authenticity_token", string(m[1]))
	resp, err = client.PostForm(srv.URL+"/auth/", vals)
	if err != nil {
		t.Fatalf("POST /auth/: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("login POST /auth/ = %d, want 302", resp.StatusCode)
	}
	return client
}

// seedConfig inserts a config row and returns it.
func seedConfig(t *testing.T, name string, active bool) *models.Config {
	t.Helper()
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "cfgtest-" + name,
		Name:       name,
		Active:     active,
	}
	if err := cfg.SetSettings(models.DefaultSettings()); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if err := models.DB.Create(cfg); err != nil {
		t.Fatalf("seed config %q: %v", name, err)
	}
	return cfg
}

// TestConfigsListHidesDeleteForActiveConfig proves the delete button is not
// rendered for the currently active configuration but is rendered for
// another (inactive) config.
func TestConfigsListHidesDeleteForActiveConfig(t *testing.T) {
	saved := CurrentConfig
	t.Cleanup(func() { CurrentConfig = saved })

	active := seedConfig(t, "active-row", true)
	inactive := seedConfig(t, "inactive-row", false)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE id IN (?, ?)", active.ID.String(), inactive.ID.String()).Exec()
	})
	CurrentConfig = active

	client := adminClient(t)
	srv := httptest.NewServer(App())
	defer srv.Close()

	resp, err := client.Get(srv.URL + "/config/")
	if err != nil {
		t.Fatalf("GET /config/: %v", err)
	}
	defer resp.Body.Close()
	bb, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read /config/ body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /config/ = %d", resp.StatusCode)
	}
	body := string(bb)

	// The active config row must not contain a delete link.
	rowFor := func(id string) string {
		i := strings.Index(body, id)
		if i == -1 {
			t.Fatalf("config %q not found on index page", id)
		}
		// find enclosing <tr> ... </tr>
		start := strings.LastIndex(body[:i], "<tr>")
		end := strings.Index(body[i:], "</tr>")
		if start == -1 || end == -1 {
			t.Fatalf("row for %q not enclosed in <tr>", id)
		}
		return body[start : i+end]
	}

	activeRow := rowFor(active.InstanceID)
	if strings.Contains(activeRow, `data-method="DELETE"`) {
		t.Error("delete button rendered for the currently active configuration")
	}
	inactiveRow := rowFor(inactive.InstanceID)
	if !strings.Contains(inactiveRow, `data-method="DELETE"`) {
		t.Error("delete button missing for the inactive configuration")
	}
}

// TestConfigsDestroyRejectsActiveConfig pins the server-side guard: deleting
// the currently active configuration returns 400 and keeps the row.
func TestConfigsDestroyRejectsActiveConfig(t *testing.T) {
	saved := CurrentConfig
	t.Cleanup(func() { CurrentConfig = saved })

	active := seedConfig(t, "destroy-active", true)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE id = ?", active.ID.String()).Exec()
	})
	CurrentConfig = active

	client := adminClient(t)
	srv := httptest.NewServer(App())
	defer srv.Close()

	// CSRF: fetch the config index to obtain a token tied to the session.
	resp, err := client.Get(srv.URL + "/config/")
	if err != nil {
		t.Fatalf("GET /config/: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read /config/ body: %v", err)
	}
	m := csrfTokenRe.FindSubmatch(b)
	if m == nil {
		t.Fatal("no authenticity_token on /config/")
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/config/"+active.ID.String(), nil)
	q := req.URL.Query()
	q.Set("_method", "DELETE")
	q.Set("authenticity_token", string(m[1]))
	req.URL.RawQuery = q.Encode()
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("DELETE active config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("DELETE active config = %d, want 400", resp.StatusCode)
	}
	// Row must still exist.
	still := &models.Config{}
	if err := models.DB.Find(still, active.ID); err != nil {
		t.Errorf("active config row deleted despite guard: %v", err)
	}
}

// TestConfigsDestroyAllowsInactiveConfig proves an inactive config can still
// be deleted through the resource.
func TestConfigsDestroyAllowsInactiveConfig(t *testing.T) {
	saved := CurrentConfig
	t.Cleanup(func() { CurrentConfig = saved })

	active := seedConfig(t, "destroy-current", true)
	inactive := seedConfig(t, "destroy-inactive", false)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE id IN (?, ?)", active.ID.String(), inactive.ID.String()).Exec()
	})
	CurrentConfig = active

	client := adminClient(t)
	srv := httptest.NewServer(App())
	defer srv.Close()

	resp, err := client.Get(srv.URL + "/config/")
	if err != nil {
		t.Fatalf("GET /config/: %v", err)
	}
	b, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read /config/ body: %v", err)
	}
	m := csrfTokenRe.FindSubmatch(b)
	if m == nil {
		t.Fatal("no authenticity_token on /config/")
	}

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/config/"+inactive.ID.String(), nil)
	q := req.URL.Query()
	q.Set("_method", "DELETE")
	q.Set("authenticity_token", string(m[1]))
	req.URL.RawQuery = q.Encode()
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("DELETE inactive config: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("DELETE inactive config = %d, want 303/302", resp.StatusCode)
	}
	gone := &models.Config{}
	if err := models.DB.Find(gone, inactive.ID); err == nil {
		t.Error("inactive config row still present after delete")
	}
}

// TestLoadConfigMultipleConfigsCoexist validates the bug report note:
// multiple configurations can exist (and be listed) at the same time. The
// active one stays the oldest active row; creating more configs never fails
// and never touches the current one.
func TestLoadConfigMultipleConfigsCoexist(t *testing.T) {
	tx := searchTestDB(t)

	saved := CurrentConfig
	CurrentConfig = nil
	t.Cleanup(func() { CurrentConfig = saved })

	first := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-multi-first", Name: "multi-first", Active: true}
	second := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-multi-second", Name: "multi-second", Active: false}
	third := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-multi-third", Name: "multi-third", Active: false}
	for _, c := range []*models.Config{first, second, third} {
		if err := c.SetSettings(models.DefaultSettings()); err != nil {
			t.Fatalf("SetSettings: %v", err)
		}
		if err := tx.Create(c); err != nil {
			t.Fatalf("seed config: %v", err)
		}
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM config WHERE id IN (?, ?, ?)",
			first.ID.String(), second.ID.String(), third.ID.String()).Exec()
	})

	// All three exist simultaneously in the DB.
	configs := &models.Configs{}
	if err := tx.Where("id IN (?, ?, ?)", first.ID.String(), second.ID.String(), third.ID.String()).All(configs); err != nil {
		t.Fatalf("list configs: %v", err)
	}
	if len(*configs) != 3 {
		t.Errorf("configs coexisting in DB = %d, want 3", len(*configs))
	}

	// LoadConfig still resolves to the oldest active config.
	cfg, err := LoadConfig(tx)
	if err != nil {
		t.Fatalf("LoadConfig with multiple configs: %v", err)
	}
	if cfg.ID != first.ID {
		t.Errorf("LoadConfig picked %v, want oldest active %v", cfg.ID, first.ID)
	}
	if CurrentConfig == nil || CurrentConfig.ID != first.ID {
		t.Error("CurrentConfig not refreshed to the oldest active config")
	}
}
