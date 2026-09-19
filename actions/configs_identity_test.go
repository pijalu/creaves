package actions

import (
	"io"
	"net/http"
	"net/url"
	"net/http/httptest"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
)

// CREAVES identity on the guest page (issue #150).

const identityName = "TS-CREAVES-Center"

func seedIdentityConfig(t *testing.T, withIdentity bool) *models.Config {
	t.Helper()
	tx := searchTestDB(t)

	cfg := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-identity", Name: "identity", Active: true}
	s := models.DefaultSettings()
	if withIdentity {
		s.CenterName = identityName
		s.AsblName = "ASBL Test"
		s.BceNumber = "0123.456.789"
		s.Address = "1 rue du Test, 1000 Testville"
		s.AccountNumber = "BE68 5440 1234 5678"
		s.Website = "https://creaves.example.org"
	}
	if err := cfg.SetSettings(s); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}
	if err := tx.Create(cfg); err != nil {
		t.Fatalf("seed config: %v", err)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM config WHERE id = ?", cfg.ID.String()).Exec()
	})
	return cfg
}

func guestBody(t *testing.T, client *http.Client, baseURL string) (int, string) {
	t.Helper()
	resp, err := client.Get(baseURL + "/guest/")
	if err != nil {
		t.Fatalf("GET /guest/: %v", err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	return resp.StatusCode, string(b)
}

// identityServer starts a fresh app server and installs cfg as the current
// config (restored on cleanup).
func identityServer(t *testing.T, cfg *models.Config) *httptest.Server {
	t.Helper()
	saved := CurrentConfigGet()
	CurrentConfigSet(cfg)
	t.Cleanup(func() { CurrentConfigSet(saved) })
	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	return srv
}

// TestGuestIdentityHiddenWhenEmpty: without identity the About entry and
// collapse block must not render at all (issue #150).
func TestGuestIdentityHiddenWhenEmpty(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	seedIdentityConfig(t, false)
	srv := identityServer(t, nil)
	client := srv.Client()
	status, body := guestBody(t, client, srv.URL)
	if status != http.StatusOK {
		t.Fatalf("guest status = %d", status)
	}
	if strings.Contains(body, "centerAbout") {
		t.Error("guest page must not render the About block when identity is empty")
	}
}

// TestGuestIdentityShownWhenSet: with identity configured the guest page
// shows the About entry with the center name and the identity table
// (issue #150).
func TestGuestIdentityShownWhenSet(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedIdentityConfig(t, true)
	srv := identityServer(t, cfg)
	client := srv.Client()
	status, body := guestBody(t, client, srv.URL)
	if status != http.StatusOK {
		t.Fatalf("guest status = %d", status)
	}
	for _, want := range []string{"centerAbout", identityName, "ASBL Test", "0123.456.789", "BE68 5440 1234 5678"} {
		if !strings.Contains(body, want) {
			t.Errorf("guest page missing %q", want)
		}
	}
}

// TestConfigUpdatePersistsIdentity: the Update handler stores the 6 identity
// fields from the form (issue #150).
func TestConfigUpdatePersistsIdentity(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedIdentityConfig(t, false)

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/config/"+cfg.ID.String()+"/edit")

	form := url.Values{}
	form.Set("_method", "PUT")
	form.Set("InstanceID", cfg.InstanceID)
	form.Set("Name", cfg.Name)
	form.Set("Description", cfg.Description)
	form.Set("Settings.CenterName", identityName)
	form.Set("Settings.AsblName", "ASBL Update")
	form.Set("Settings.Website", "https://updated.example.org")
	form.Set("Settings.GuestText1", "Guest text one")
	form.Set("Settings.GuestText2", "Guest text two")
	resp := postTodoForm(t, client, baseURL, "/config/"+cfg.ID.String(), token, form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("config update status = %d, want redirect", resp.StatusCode)
	}

	reloaded := &models.Config{}
	if err := searchTestDB(t).Find(reloaded, cfg.ID); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	s, err := reloaded.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	if s.CenterName != identityName || s.AsblName != "ASBL Update" || s.Website != "https://updated.example.org" {
		t.Errorf("identity not persisted: %+v", s)
	}
	if s.GuestText1 != "Guest text one" || s.GuestText2 != "Guest text two" {
		t.Errorf("guest texts not persisted: %+v", s)
	}
}
