package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// Bug 4 regression: settings JSON must never include a webhook API key —
// the shared secret authorizing pushes to a console now lives on the
// sync_targets table, never on the config settings blob. Legacy blobs that
// still carry the old keys are ignored on read (json.Unmarshal skips
// unknown fields).

func TestConfigMarshalJSONHasNoWebhookSecrets(t *testing.T) {
	c := Config{
		InstanceID: "test-instance",
		Name:       "Test",
	}
	if err := c.SetSettings(DefaultSettings()); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	for _, legacy := range []string{"webhook_api_key", "webhook_url", "webhook_enabled"} {
		if strings.Contains(string(data), legacy) {
			t.Errorf("marshaled config carries legacy webhook field %q: %s", legacy, data)
		}
	}
}

func TestConfigSettingsIgnoresLegacyWebhookKeys(t *testing.T) {
	// A settings blob written before the sync_targets migration still holds
	// the old webhook keys; reading it must not fail and must not leak them
	// back out on re-marshal.
	legacy := json.RawMessage(`{"enable_event_stream":true,"webhook_enabled":true,"webhook_url":"http://console.example/webhook/events","webhook_api_key":"super-secret-key-123","webhook_batch_size":5,"webhook_max_per_min":120}`)
	c := Config{InstanceID: "x", Name: "y", Settings: legacy}

	settings, err := c.GetSettings()
	if err != nil {
		t.Fatalf("GetSettings on legacy blob: %v", err)
	}
	if !settings.EnableEventStream {
		t.Errorf("enable_event_stream lost from legacy blob: %+v", settings)
	}

	out, err := json.Marshal(settings)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(out), "super-secret-key-123") {
		t.Errorf("legacy webhook API key leaked through settings round-trip: %s", out)
	}
}

func TestConfigMarshalJSONEmptySettings(t *testing.T) {
	c := Config{InstanceID: "x", Name: "y"}
	if _, err := json.Marshal(c); err != nil {
		t.Fatalf("Marshal with empty settings: %v", err)
	}
}
