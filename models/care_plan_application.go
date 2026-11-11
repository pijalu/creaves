package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Application source types (§4.5): which planning level produced the
// occurrence. No DB FK on the source — dual target, enforced in code.
const (
	ApplicationSourceRule   = "rule"
	ApplicationSourceAnimal = "animal"
)

// Application fulfillment types (§4.5): which row the apply created.
const (
	ApplicationFulfillmentCare      = "care"
	ApplicationFulfillmentTreatment = "treatment"
)

// Application statuses (§4.5). Skip is an explicit application (with
// mandatory reason) so "missing" means truly forgotten; Defer is a snooze
// resurfacing at DeferredUntil (§10-A2). A defer creates no fulfillment
// record but still occupies the idempotency key.
const (
	ApplicationStatusApplied  = "applied"
	ApplicationStatusSkipped  = "skipped"
	ApplicationStatusDeferred = "deferred"
)

// CarePlanApplication is one fulfilled (or skipped/deferred) occurrence of a
// rule or animal plan (§4.5). AppliedAt is the click time (§10.1), not
// due_at — the created care/treatment row also carries click time.
// UNIQUE (source_type, source_id, animal_id, due_at) makes fulfillment
// idempotent (no double-apply from two browser tabs).
type CarePlanApplication struct {
	ID              uuid.UUID       `json:"id" db:"id"`
	SourceType      string          `json:"source_type" db:"source_type"`
	SourceID        uuid.UUID       `json:"source_id" db:"source_id"`
	SourceSnapshot  json.RawMessage `json:"source_snapshot" db:"source_snapshot"`
	AnimalID        int             `json:"animal_id" db:"animal_id"`
	DueAt           time.Time       `json:"due_at" db:"due_at"`
	AppliedAt       time.Time       `json:"applied_at" db:"applied_at"`
	UserID          uuid.UUID       `json:"user_id" db:"user_id"`
	FulfillmentType string          `json:"fulfillment_type" db:"fulfillment_type"`
	FulfillmentID   string          `json:"fulfillment_id" db:"fulfillment_id"`
	Status          string          `json:"status" db:"status"`
	// DeferredUntil is set only when Status=deferred (§10-A2/CP4): the
	// snooze target, clamped to before the next occurrence of the source.
	DeferredUntil *time.Time `json:"deferred_until" db:"deferred_until"`
	// FulfillmentDeleted is set by the care/treatment destroy hooks
	// (§10-CP1): the linked fulfillment row is gone but the application is
	// kept for audit.
	FulfillmentDeleted bool   `json:"fulfillment_deleted" db:"fulfillment_deleted"`
	Note               string `json:"note" db:"note"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CarePlanApplications is not required by pop and may be deleted
type CarePlanApplications []CarePlanApplication

// String is not required by pop and may be deleted
func (a CarePlanApplication) String() string {
	js, _ := json.Marshal(a)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (a *CarePlanApplication) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()
	verrs.Append(validate.Validate(
		&validators.StringInclusion{
			Field: a.SourceType,
			Name:  "SourceType",
			List:  []string{ApplicationSourceRule, ApplicationSourceAnimal},
		},
		&validators.UUIDIsPresent{Field: a.SourceID, Name: "SourceID"},
		&validators.IntIsPresent{Field: a.AnimalID, Name: "AnimalID"},
		&validators.TimeIsPresent{Field: a.DueAt, Name: "DueAt"},
		&validators.TimeIsPresent{Field: a.AppliedAt, Name: "AppliedAt"},
		&validators.UUIDIsPresent{Field: a.UserID, Name: "UserID"},
		&validators.StringInclusion{
			Field: a.FulfillmentType,
			Name:  "FulfillmentType",
			List:  []string{ApplicationFulfillmentCare, ApplicationFulfillmentTreatment},
		},
		&validators.StringIsPresent{Field: a.FulfillmentID, Name: "FulfillmentID"},
		&validators.StringInclusion{
			Field: a.Status,
			Name:  "Status",
			List:  []string{ApplicationStatusApplied, ApplicationStatusSkipped, ApplicationStatusDeferred},
		},
	))
	if len(a.SourceSnapshot) == 0 {
		verrs.Add("source_snapshot", "SourceSnapshot can not be empty.")
	}
	switch a.Status {
	case ApplicationStatusDeferred:
		if a.DeferredUntil == nil || a.DeferredUntil.IsZero() {
			verrs.Add("deferred_until", "DeferredUntil must be set when status is deferred (§10-A2).")
		}
		if a.Note == "" {
			verrs.Add("note", "Note is mandatory for deferred applications (§10-CP4).")
		}
	case ApplicationStatusSkipped:
		if a.Note == "" {
			verrs.Add("note", "Note is mandatory for skipped applications (§10-CP4).")
		}
	}
	if a.Status != ApplicationStatusDeferred && a.DeferredUntil != nil && !a.DeferredUntil.IsZero() {
		verrs.Add("deferred_until", "DeferredUntil must be NULL unless status is deferred (§10-A2).")
	}
	return verrs, nil
}
