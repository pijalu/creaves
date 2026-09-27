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

// CareAnimalPlan is the single-animal planning level (§4.7): created
// directly from the animal page by any caretaker. Same action payload and
// schedule JSON as rules — full parity. When ReplacesKind is true the plan
// suppresses ALL generic-rule occurrences of the same action kind for the
// animal (§10-A3 kind-level override); otherwise only same-slot collisions
// are suppressed.
type CareAnimalPlan struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	AnimalID      int             `json:"animal_id" db:"animal_id"`
	Name          string          `json:"name" db:"name"`
	ActionKind    string          `json:"action_kind" db:"action_kind"`
	ActionPayload json.RawMessage `json:"action_payload" db:"action_payload"`
	Schedule      json.RawMessage `json:"schedule" db:"schedule"`
	ReplacesKind  bool            `json:"replaces_kind" db:"replaces_kind"`
	Active        bool            `json:"active" db:"active"`
	// CreatedBy is the authoring user; NULL for plans created by the
	// startup converter (§8.1 — the converter is not a user, and the
	// column carries a users FK).
	CreatedBy     nulls.UUID      `json:"created_by" db:"created_by"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CareAnimalPlans is not required by pop and may be deleted
type CareAnimalPlans []CareAnimalPlan

// String is not required by pop and may be deleted
func (p CareAnimalPlan) String() string {
	js, _ := json.Marshal(p)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (p *CareAnimalPlan) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()
	verrs.Append(validate.Validate(
		&validators.IntIsPresent{Field: p.AnimalID, Name: "AnimalID"},
		&validators.StringIsPresent{Field: p.Name, Name: "Name"},
		&validators.StringIsPresent{Field: p.ActionKind, Name: "ActionKind"},
	))
	validateActionAndSchedule(verrs, p.ActionKind, p.ActionPayload, p.Schedule)
	return verrs, nil
}
