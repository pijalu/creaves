package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// CarePlanConversion is the idempotency marker of the one-shot startup
// conversion (§8.1): key='startup_v1' with a completion timestamp makes
// re-boots a no-op; deleting the marker row re-runs the converter.
type CarePlanConversion struct {
	Key        string    `json:"key" db:"key"`
	FinishedAt time.Time `json:"finished_at" db:"finished_at"`
	// Report is the §8.1 step-5 conversion report (JSON): per-animal
	// converted/skipped lines, per-step counters. Persisted so the admin
	// can resolve skips by hand (EN9 data-hygiene panel, §13).
	Report nulls.String `json:"report" db:"report"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CarePlanConversions is not required by pop and may be deleted
type CarePlanConversions []CarePlanConversion

// String is not required by pop and may be deleted
func (c CarePlanConversion) String() string {
	js, _ := json.Marshal(c)
	return string(js)
}

// TableName pins the §8.1 table name (singular, per spec) — pop's default
// pluralization would target care_plan_conversions.
func (CarePlanConversion) TableName() string {
	return "care_plan_conversion"
}

// Validate gets run every time you call a "pop.Validate*" method.
func (c *CarePlanConversion) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: c.Key, Name: "Key"},
		&validators.TimeIsPresent{Field: c.FinishedAt, Name: "FinishedAt"},
	), nil
}
