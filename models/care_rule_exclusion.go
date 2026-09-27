package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// CareRuleExclusion is a per-animal opt-out from a rule (§4.1): an excluded
// animal never matches the rule, regardless of the matcher result. The
// reason is shown in the why-explanation on the rule page and the animal's
// Plan tab.
type CareRuleExclusion struct {
	ID        uuid.UUID `json:"id" db:"id"`
	RuleID    uuid.UUID `json:"rule_id" db:"rule_id"`
	AnimalID  int       `json:"animal_id" db:"animal_id"`
	Reason    string    `json:"reason" db:"reason"`
	CreatedBy uuid.UUID `json:"created_by" db:"created_by"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CareRuleExclusions is not required by pop and may be deleted
type CareRuleExclusions []CareRuleExclusion

// String is not required by pop and may be deleted
func (e CareRuleExclusion) String() string {
	js, _ := json.Marshal(e)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (e *CareRuleExclusion) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.UUIDIsPresent{Field: e.RuleID, Name: "RuleID"},
		&validators.IntIsPresent{Field: e.AnimalID, Name: "AnimalID"},
		&validators.StringLengthInRange{
			Field: e.Reason, Name: "Reason", Min: 0, Max: 500,
		},
	), nil
}
