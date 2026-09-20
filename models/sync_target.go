package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// SyncTarget describes one consolidation hub (Creaves Console instance) this
// Creaves pushes its event stream to. A Creaves instance may fan out to
// multiple targets; each target carries its own endpoint, credentials and
// throttling settings, and deliveries are tracked per target in
// event_deliveries.
type SyncTarget struct {
	ID               uuid.UUID `json:"id" db:"id"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
	Name             string    `json:"name" db:"name"`
	Enabled          bool      `json:"enabled" db:"enabled"`
	WebhookURL       string    `json:"webhook_url" db:"webhook_url"`
	WebhookAPIKey    string    `json:"-" db:"webhook_api_key"`
	WebhookBatchSize int       `json:"webhook_batch_size" db:"webhook_batch_size"`
	WebhookMaxPerMin int       `json:"webhook_max_per_min" db:"webhook_max_per_min"`
}

// String is not required by pop and may be deleted, but added for debugging.
func (s SyncTarget) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// SyncTargets is not required by pop and may be deleted, but added for convenience.
type SyncTargets []SyncTarget

// EffectiveBatchSize returns the configured batch size, defaulting to 1.
func (s *SyncTarget) EffectiveBatchSize() int {
	if s.WebhookBatchSize < 1 {
		return 1
	}
	return s.WebhookBatchSize
}

// EffectiveMaxPerMin returns the configured per-minute delivery cap,
// defaulting to 60.
func (s *SyncTarget) EffectiveMaxPerMin() int {
	if s.WebhookMaxPerMin < 1 {
		return 60
	}
	return s.WebhookMaxPerMin
}

// MaskedAPIKey returns a masked version of the API key for display purposes.
func (s *SyncTarget) MaskedAPIKey() string {
	k := strings.TrimSpace(s.WebhookAPIKey)
	if k == "" {
		return ""
	}
	if len(k) <= 4 {
		return "••••"
	}
	return "••••" + k[len(k)-4:]
}

// Deliverable reports whether this target can receive events right now
// (enabled and configured with a URL).
func (s *SyncTarget) Deliverable() bool {
	return s.Enabled && strings.TrimSpace(s.WebhookURL) != ""
}

// Validate gets run every time you call a "pop.Validate*" method.
func (s *SyncTarget) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: s.Name, Name: "Name"},
		&validators.FuncValidator{
			Field:   s.WebhookURL,
			Name:    "WebhookURL",
			Message: "webhook URL is required",
			Fn: func() bool {
				return !s.Enabled || strings.TrimSpace(s.WebhookURL) != ""
			},
		},
	), nil
}

// EnabledSyncTargets returns every enabled sync target ordered by name.
func EnabledSyncTargets(tx *pop.Connection) (SyncTargets, error) {
	targets := SyncTargets{}
	if err := tx.Where("enabled = ?", true).Order("name asc").All(&targets); err != nil {
		return nil, err
	}
	return targets, nil
}

// HasEnabledSyncTarget reports whether at least one sync target is enabled
// and fully configured (has a webhook URL).
func HasEnabledSyncTarget(tx *pop.Connection) (bool, error) {
	n, err := tx.Where("enabled = ?", true).Where("webhook_url <> ''").Count(&SyncTarget{})
	return n > 0, err
}
