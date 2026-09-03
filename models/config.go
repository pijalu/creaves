package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Config stores the application configuration including feature flags
type Config struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	InstanceID  string          `json:"instance_id" db:"instance_id"`
	Name        string          `json:"name" db:"name"`
	Description string          `json:"description" db:"description"`
	Active      bool            `json:"active" db:"active"`
	Settings    json.RawMessage `json:"settings" db:"settings"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// TableName overrides the table name used by Pop to `config`
func (Config) TableName() string {
	return "config"
}

// ConfigSettings represents the structure of the settings JSON
type ConfigSettings struct {
	// Event stream feature flag - controls all event stream functionality
	EnableEventStream bool `json:"enable_event_stream"`
	// Webhook configuration
	WebhookEnabled   bool   `json:"webhook_enabled"`
	WebhookURL       string `json:"webhook_url"`
	WebhookAPIKey    string `json:"webhook_api_key"`
	WebhookBatchSize int    `json:"webhook_batch_size"`
	WebhookMaxPerMin int    `json:"webhook_max_per_min"`
}

// DefaultSettings returns the default configuration settings
func DefaultSettings() ConfigSettings {
	return ConfigSettings{
		EnableEventStream: true,
		WebhookEnabled:    false,
		WebhookBatchSize:  1,
		WebhookMaxPerMin:  60,
	}
}

// GetSettings parses the settings JSON into a ConfigSettings struct
func (c *Config) GetSettings() (ConfigSettings, error) {
	settings := DefaultSettings()
	if len(c.Settings) == 0 {
		return settings, nil
	}
	err := json.Unmarshal(c.Settings, &settings)
	if err != nil {
		return DefaultSettings(), err
	}
	return settings, nil
}

// SetSettings updates the settings from a ConfigSettings struct
func (c *Config) SetSettings(settings ConfigSettings) error {
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	c.Settings = json.RawMessage(data)
	return nil
}

// MarshalJSON redacts the webhook API key from the settings blob so the
// shared secret is never exposed through JSON API responses.
func (c Config) MarshalJSON() ([]byte, error) {
	type alias Config // avoid infinite recursion
	out := alias(c)
	if len(out.Settings) > 0 {
		settings, err := c.GetSettings()
		if err == nil && settings.WebhookAPIKey != "" {
			settings.WebhookAPIKey = ""
			if data, err := json.Marshal(settings); err == nil {
				out.Settings = json.RawMessage(data)
			}
		}
	}
	return json.Marshal(out)
}

// String returns the JSON representation
func (c Config) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// Configs is a slice of Config
type Configs []Config

// String returns the JSON representation
func (c Configs) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// Validate gets run every time you call a "pop.Validate*" method
func (c *Config) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: c.InstanceID, Name: "InstanceID"},
		&validators.StringIsPresent{Field: c.Name, Name: "Name"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method
func (c *Config) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method
func (c *Config) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
