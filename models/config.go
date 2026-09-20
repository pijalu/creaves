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

// ConfigSettings represents the structure of the settings JSON.
// Webhook delivery is configured per sync target (sync_targets table); the
// config only keeps the global event-stream flag. Legacy webhook_* keys in
// stored settings blobs are ignored on read (backfilled into sync_targets by
// migration 20261010090000).
type ConfigSettings struct {
	// Event stream feature flag - controls all event stream functionality
	EnableEventStream bool `json:"enable_event_stream"`
	// CREAVES identity (issue #150), shown on the guest page. Not secret.
	CenterName    string `json:"center_name"`
	AsblName      string `json:"asbl_name"`
	BceNumber     string `json:"bce_number"`
	Address       string `json:"address"`
	AccountNumber string `json:"account_number"`
	Website       string `json:"website"`
	// Free-text blocks the center fills with its own information; they are
	// rendered on the public guest page (#197 sub-item 9).
	GuestText1 string `json:"guest_text_1"`
	GuestText2 string `json:"guest_text_2"`
}

// DefaultSettings returns the default configuration settings
func DefaultSettings() ConfigSettings {
	return ConfigSettings{
		EnableEventStream: true,
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
