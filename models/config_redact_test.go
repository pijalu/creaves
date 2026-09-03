package models

import (
	"encoding/json"
	"strings"
	"testing"
)

// Bug 4 regression: Config JSON responses must never include the webhook API
// key, which is the shared secret authorizing pushes to the console.

func TestConfigMarshalJSONRedactsWebhookAPIKey(t *testing.T) {
	c := Config{
		InstanceID: "test-instance",
		Name:       "Test",
	}
	settings := DefaultSettings()
	settings.WebhookAPIKey = "super-secret-key-123"
	if err := c.SetSettings(settings); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(data), "super-secret-key-123") {
		t.Errorf("marshaled config leaks webhook API key: %s", data)
	}
	if !strings.Contains(string(data), `"webhook_api_key":""`) {
		t.Errorf("expected redacted webhook_api_key field in output: %s", data)
	}
}

func TestConfigMarshalJSONKeepsOtherSettings(t *testing.T) {
	c := Config{
		InstanceID: "test-instance",
		Name:       "Test",
	}
	settings := DefaultSettings()
	settings.WebhookAPIKey = "super-secret-key-123"
	if err := c.SetSettings(settings); err != nil {
		t.Fatalf("SetSettings: %v", err)
	}

	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded struct {
		Settings ConfigSettings `json:"settings"`
	}
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if decoded.Settings.WebhookBatchSize != settings.WebhookBatchSize {
		t.Errorf("batch size lost: got %d, want %d", decoded.Settings.WebhookBatchSize, settings.WebhookBatchSize)
	}
}

func TestConfigMarshalJSONEmptySettings(t *testing.T) {
	c := Config{InstanceID: "x", Name: "y"}
	if _, err := json.Marshal(c); err != nil {
		t.Fatalf("Marshal with empty settings: %v", err)
	}
}
