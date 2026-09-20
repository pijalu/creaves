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
	form.Set("InstanceID", cfg.InstanceID)
	form.Set("Name", cfg.Name)
	form.Set("Settings.CenterName", "Updated Center")
	form.Set("Settings.GuestText1", "updated guest text")
	resp := postTodoForm(t, client, baseURL, "/config/"+cfg.ID.String(), token, form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("identity update status = %d, want redirect", resp.StatusCode)
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

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/config/"+cfg.ID.String()+"/sync")

	form := url.Values{}
	form.Set("_method", "PUT")
	form.Set("form_scope", "sync")
	form.Set("Settings.EnableEventStream", "true")
	form.Set("Settings.WebhookEnabled", "true")
	form.Set("Settings.WebhookURL", "http://new-console.example.org/webhook/events")
	form.Set("Settings.WebhookAPIKey", "") // blank => keep stored key
	form.Set("Settings.WebhookBatchSize", "7")
	form.Set("Settings.WebhookMaxPerMin", "90")
	resp := postTodoForm(t, client, baseURL, "/config/"+cfg.ID.String(), token, form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("sync update status = %d, want redirect", resp.StatusCode)
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

// The sync edit page renders for admins and carries the scope marker.
func TestConfigSyncEditPageRenders(t *testing.T) {
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	cfg := seedScopeConfig(t)

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/config/"+cfg.ID.String()+"/sync")
	if token == "" {
		t.Fatal("no CSRF token on sync edit page — page did not render the form")
	}
}
