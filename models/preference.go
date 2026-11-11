package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Preference is ONE day-plan view setting per action kind (R8-5): how much
// late work and how much future work the work screen shows for that kind.
// NULL = no cap = the pre-preferences behavior (show everything) — the
// seeded default, so installing the table changes nothing until an admin
// does.
type Preference struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Kind string    `json:"kind" db:"kind"`
	// LateShowHours: late/missing occurrences older than this (now - due)
	// disappear from the day plan's late tier. NULL shows all.
	LateShowHours nulls.Int `json:"late_show_hours" db:"late_show_hours"`
	// FutureShowHours: later-tier occurrences due further than this ahead
	// disappear from the day plan's later tier. NULL shows all.
	FutureShowHours nulls.Int `json:"future_show_hours" db:"future_show_hours"`
	// NowWindowMinutes: reserved (R8-5) — the span around "now" the view
	// treats as the current moment. NULL keeps the engine's per-schedule
	// grace/lookahead windows.
	NowWindowMinutes nulls.Int `json:"now_window_minutes" db:"now_window_minutes"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// TableName overrides the table name used by Pop to `preferences`.
func (Preference) TableName() string { return "preferences" }

// String is not required by pop and may be deleted
func (p Preference) String() string {
	jp, _ := json.Marshal(p)
	return string(jp)
}

// Preferences is not required by pop and may be deleted
type Preferences []Preference

// Validate gets run every time you call a "pop.Validate*" method.
func (p *Preference) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Name: "Kind", Field: p.Kind},
	), nil
}
