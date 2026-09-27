package models

import (
	"encoding/json"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// validateActionAndSchedule runs the §4.2 payload and §4.3 schedule
// validations shared by CareRule and CareAnimalPlan (both planning levels
// use "full parity" action payloads and schedule JSON, §4.7).
func validateActionAndSchedule(verrs *validate.Errors, kind string, payload, schedule json.RawMessage) {
	if err := careplan.ValidateActionKind(kind); err != nil {
		verrs.Add("action_kind", err.Error())
	}
	if len(payload) == 0 {
		verrs.Add("action_payload", "ActionPayload can not be empty.")
	} else if err := careplan.ValidateActionPayload(kind, payload); err != nil {
		verrs.Add("action_payload", err.Error())
	}
	if len(schedule) == 0 {
		verrs.Add("schedule", "Schedule can not be empty.")
	} else if _, err := careplan.ParseScheduleJSON(schedule); err != nil {
		verrs.Add("schedule", err.Error())
	}
}

// CareRule is a generic (rule-level) care plan: action + schedule + a named
// matcher selecting the animals it applies to (§4.1 of docs/care-expert.md).
type CareRule struct {
	ID            uuid.UUID       `json:"id" db:"id"`
	Name          string          `json:"name" db:"name"`
	Description   string          `json:"description" db:"description"`
	ActionKind    string          `json:"action_kind" db:"action_kind"`
	ActionPayload json.RawMessage `json:"action_payload" db:"action_payload"`
	Schedule      json.RawMessage `json:"schedule" db:"schedule"`
	MatcherID     uuid.NullUUID   `json:"matcher_id" db:"matcher_id"`
	Active        bool            `json:"active" db:"active"`
	Priority      int             `json:"priority" db:"priority"`
	ValidFrom     *time.Time      `json:"valid_from" db:"valid_from"`
	ValidTo       *time.Time      `json:"valid_to" db:"valid_to"`
	StopOnOuttake bool            `json:"stop_on_outtake" db:"stop_on_outtake"`
	// LatchMembership enables the course latch (§5.4): when the schedule
	// carries duration_days, an animal with ≥1 application stays in the
	// rule until the course ends even if it stops matching.
	LatchMembership bool      `json:"latch_membership" db:"latch_membership"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// CareRules is not required by pop and may be deleted
type CareRules []CareRule

// String is not required by pop and may be deleted
func (r CareRule) String() string {
	js, _ := json.Marshal(r)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (r *CareRule) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()
	verrs.Append(validate.Validate(
		&validators.StringIsPresent{Field: r.Name, Name: "Name"},
		&validators.StringIsPresent{Field: r.ActionKind, Name: "ActionKind"},
	))
	if r.ValidFrom != nil && r.ValidTo != nil && r.ValidTo.Before(*r.ValidFrom) {
		verrs.Add("valid_to", "ValidTo must not be before ValidFrom.")
	}
	validateActionAndSchedule(verrs, r.ActionKind, r.ActionPayload, r.Schedule)
	return verrs, nil
}
