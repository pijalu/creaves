package actions

import (
	"net/http"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
)

// Scoped config update (bug 4): the identity view and the sync view submit
// only their own field group; the other group must be preserved.

func seedScopeConfig(t *testing.T) *models.Config {
	t.Helper()
	tx := searchTestDB(t)

	cfg := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-scope", Name: "scope", Active: true}
	s := models.DefaultSettings()
	s.CenterName = "Original Center"
	s.GuestText1 = "original guest text"
	s.WebhookURL = "http://console.example.org/webhook/events"
	s.WebhookAPIKey = "secret-key-123"
	s.WebhookBatchSize = 5
	s.WebhookMaxPerMin = 120
	s.WebhookEnabled = true
	s.EnableEventStream = true
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

func reloadSettings(t *testing.T, id uuid.UUID) models.ConfigSettings {
	t.Helper()
	reloaded := &models.Config{}
	if err := searchTestDB(t).Find(reloaded, id); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	s, err := reloaded.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings: %v", err)
	}
	return s
}

// reloadConfig fetches the config row anew from the test DB.
func reloadConfig(t *testing.T, id uuid.UUID) *models.Config {
	t.Helper()
	reloaded := &models.Config{}
	if err := searchTestDB(t).Find(reloaded, id); err != nil {
		t.Fatalf("reload config: %v", err)
	}
	return reloaded
}

// submitConfigForm PUTs the given form to the config endpoint and requires
// a redirect response.
func submitConfigForm(t *testing.T, client *http.Client, baseURL, token string, id uuid.UUID, form url.Values) {
	t.Helper()
	resp := postTodoForm(t, client, baseURL, "/config/"+id.String(), token, form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("config update status = %d, want redirect", resp.StatusCode)
	}
}

// Identity scope: center fields update; sync fields (URL, key, limits,
// toggles) keep their stored values.
func TestConfigUpdateScopeIdentityPreservesSync(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedScopeConfig(t)

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/config/"+cfg.ID.String()+"/edit")

	form := url.Values{}
	form.Set("_method", "PUT")
	form.Set("form_scope", "identity")
	// Identity form no longer carries the instance ID: a forged/leftover
	// value must be ignored (stored value preserved).
	form.Set("InstanceID", "tampered-instance-id")
	form.Set("Name", cfg.Name)
	form.Set("Settings.CenterName", "Updated Center")
	form.Set("Settings.GuestText1", "updated guest text")
	submitConfigForm(t, client, baseURL, token, cfg.ID, form)

	reloaded := reloadConfig(t, cfg.ID)
	if reloaded.InstanceID != cfg.InstanceID {
		t.Errorf("instance ID clobbered by identity update: got %q, want %q", reloaded.InstanceID, cfg.InstanceID)
	}

	s := reloadSettings(t, cfg.ID)
	if s.CenterName != "Updated Center" || s.GuestText1 != "updated guest text" {
		t.Errorf("identity fields not updated: %+v", s)
	}
	if s.WebhookURL != "http://console.example.org/webhook/events" {
		t.Errorf("webhook URL clobbered by identity update: %q", s.WebhookURL)
	}
	if s.WebhookAPIKey != "secret-key-123" {
		t.Errorf("webhook API key clobbered by identity update")
	}
	if s.WebhookBatchSize != 5 || s.WebhookMaxPerMin != 120 {
		t.Errorf("webhook limits reset by identity update: batch=%d max=%d", s.WebhookBatchSize, s.WebhookMaxPerMin)
	}
	if !s.WebhookEnabled || !s.EnableEventStream {
		t.Errorf("toggles reset by identity update: webhook=%v stream=%v", s.WebhookEnabled, s.EnableEventStream)
	}
}

// Sync scope: webhook fields update; identity fields keep stored values; a
// blank API key preserves the stored key.
func TestConfigUpdateScopeSyncPreservesIdentity(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedScopeConfig(t)
	// The sync configuration page addresses the active config only.
	saved := CurrentConfigGet()
	CurrentConfigSet(cfg)
	t.Cleanup(func() { CurrentConfigSet(saved) })

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_configuration")

	form := url.Values{}
	form.Set("_method", "PUT")
	form.Set("form_scope", "sync")
	form.Set("InstanceID", "cfgtest-scope-renamed")
	form.Set("Settings.EnableEventStream", "true")
	form.Set("Settings.WebhookEnabled", "true")
	form.Set("Settings.WebhookURL", "http://new-console.example.org/webhook/events")
	form.Set("Settings.WebhookAPIKey", "") // blank => keep stored key
	form.Set("Settings.WebhookBatchSize", "7")
	form.Set("Settings.WebhookMaxPerMin", "90")
	submitConfigForm(t, client, baseURL, token, cfg.ID, form)

	reloaded := reloadConfig(t, cfg.ID)
	if reloaded.InstanceID != "cfgtest-scope-renamed" {
		t.Errorf("instance ID not updated by sync form: %q", reloaded.InstanceID)
	}
	// Sync scope must not touch the identity columns.
	if reloaded.Name != cfg.Name || reloaded.Active != cfg.Active {
		t.Errorf("identity columns clobbered by sync update: %+v", reloaded)
	}

	s := reloadSettings(t, cfg.ID)
	if s.WebhookURL != "http://new-console.example.org/webhook/events" {
		t.Errorf("webhook URL not updated: %q", s.WebhookURL)
	}
	if s.WebhookBatchSize != 7 || s.WebhookMaxPerMin != 90 {
		t.Errorf("webhook limits not updated: batch=%d max=%d", s.WebhookBatchSize, s.WebhookMaxPerMin)
	}
	if s.WebhookAPIKey != "secret-key-123" {
		t.Errorf("blank API key wiped stored key")
	}
	if s.CenterName != "Original Center" || s.GuestText1 != "original guest text" {
		t.Errorf("identity clobbered by sync update: %+v", s)
	}
}

// The sync edit page renders for admins (via the admin Synchronization
// menu URL) and carries the scope marker.
func TestConfigSyncEditPageRenders(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedScopeConfig(t)
	saved := CurrentConfigGet()
	CurrentConfigSet(cfg)
	t.Cleanup(func() { CurrentConfigSet(saved) })

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_configuration")
	if token == "" {
		t.Fatal("no CSRF token on sync edit page — page did not render the form")
	}
}

// The legacy per-config sync URL redirects to the admin sync page for the
// active config and is gone for any other config.
func TestConfigSyncLegacyURLBehavior(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	active := seedScopeConfig(t)
	// Seed a second (non-active) config with a distinct instance ID.
	other := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: "cfgtest-scope-other", Name: "scope-other", Active: false}
	if err := searchTestDB(t).Create(other); err != nil {
		t.Fatalf("seed second config: %v", err)
	}
	t.Cleanup(func() {
		searchTestDB(t).RawQuery("DELETE FROM config WHERE id = ?", other.ID.String()).Exec()
	})
	saved := CurrentConfigGet()
	CurrentConfigSet(active)
	t.Cleanup(func() { CurrentConfigSet(saved) })

	client, baseURL := adminClientWithURL(t)

	resp, err := client.Get(baseURL + "/config/" + active.ID.String() + "/sync")
	if err != nil {
		t.Fatalf("GET legacy sync URL: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusMovedPermanently {
		t.Errorf("active legacy sync URL status = %d, want 301", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/sync_configuration" {
		t.Errorf("active legacy sync URL Location = %q, want /sync_configuration", loc)
	}

	resp, err = client.Get(baseURL + "/config/" + other.ID.String() + "/sync")
	if err != nil {
		t.Fatalf("GET non-active legacy sync URL: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("non-active legacy sync URL status = %d, want 404", resp.StatusCode)
	}
}
