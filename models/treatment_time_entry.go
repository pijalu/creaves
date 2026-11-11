package models

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// TreatmentTimeEntry is one expected administration time of a treatment
// (bugs.md R5-3a, U25 / D-e): per-time storage replacing the legacy
// Timedonebitmap, which stays in the treatments table but dormant (rollback
// path). One treatment row owns one entry per expected time — more than
// three runs per day are representable, unlike the 3-bucket bitmap.
type TreatmentTimeEntry struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	TreatmentID   uuid.UUID    `json:"treatment_id" db:"treatment_id"`
	AnimalID      int          `json:"animal_id" db:"animal_id"`
	DueAt         time.Time    `json:"due_at" db:"due_at"`
	TimeLabel     string       `json:"time_label" db:"time_label"`
	Status        string       `json:"status" db:"status"`
	AppliedAt     nulls.Time   `json:"applied_at" db:"applied_at"`
	UserID        nulls.UUID   `json:"user_id" db:"user_id"`
	Note          nulls.String `json:"note" db:"note"`
	Source        string       `json:"source" db:"source"`
	ApplicationID nulls.UUID   `json:"application_id" db:"application_id"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
}

// Entry status values (bugs.md R5-3a).
const (
	TreatmentEntryStatusPending = "pending"
	TreatmentEntryStatusDone    = "done"
	TreatmentEntryStatusSkipped = "skipped"
)

// Entry source values (bugs.md R5-3a): protocol = written by the care-plan
// fulfillment, manual = seeded by the manual treatment form, migration =
// backfilled from the legacy bitmap by care:migrate_treatment_times.
const (
	TreatmentEntrySourceProtocol  = "protocol"
	TreatmentEntrySourceManual    = "manual"
	TreatmentEntrySourceMigration = "migration"
)

// DefaultEntryTimes maps the legacy 3-bucket bitmap to the default per-time
// labels used when seeding manual treatments and migrating legacy rows
// (bugs.md R5-3b/R5-3d).
var DefaultEntryTimes = map[int]string{
	Treatement_MORNING: "08:00",
	Treatement_NOON:    "12:00",
	Treatement_EVENING: "18:00",
}

// TreatmentTimeEntries is a sortable collection of TreatmentTimeEntry.
type TreatmentTimeEntries []TreatmentTimeEntry

// String is not required by pop and may be deleted
func (t TreatmentTimeEntries) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// SortByTime orders the entries by their due time ascending.
func (t TreatmentTimeEntries) SortByTime() {
	sort.Slice(t, func(i, j int) bool {
		return t[i].DueAt.Before(t[j].DueAt)
	})
}

// FindByLabel returns the first entry whose time_label matches, or nil.
func (t TreatmentTimeEntries) FindByLabel(label string) *TreatmentTimeEntry {
	for i := range t {
		if t[i].TimeLabel == label {
			return &t[i]
		}
	}
	return nil
}

// FindByApplication returns the first entry linked to the given care-plan
// application, or nil.
func (t TreatmentTimeEntries) FindByApplication(appID uuid.UUID) *TreatmentTimeEntry {
	for i := range t {
		if t[i].ApplicationID.Valid && t[i].ApplicationID.UUID == appID {
			return &t[i]
		}
	}
	return nil
}

// HasDone reports whether at least one entry is done.
func (t TreatmentTimeEntries) HasDone() bool {
	for i := range t {
		if t[i].Status == TreatmentEntryStatusDone {
			return true
		}
	}
	return false
}

// InBucket reports whether the entry's due hour falls in [minH, maxH).
func (t *TreatmentTimeEntry) InBucket(minH, maxH int) bool {
	h := t.DueAt.Hour()
	return h >= minH && h < maxH
}

// Validate gets run every time you call a "pop.Validate*" method.
func (t *TreatmentTimeEntry) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.UUIDIsPresent{Field: t.TreatmentID, Name: "TreatmentID"},
		&validators.IntIsPresent{Field: t.AnimalID, Name: "AnimalID"},
		&validators.StringIsPresent{Field: t.TimeLabel, Name: "TimeLabel"},
		&validators.StringInclusion{Field: t.Status, Name: "Status",
			List: []string{TreatmentEntryStatusPending, TreatmentEntryStatusDone, TreatmentEntryStatusSkipped}},
		&validators.StringInclusion{Field: t.Source, Name: "Source",
			List: []string{TreatmentEntrySourceProtocol, TreatmentEntrySourceManual, TreatmentEntrySourceMigration}},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate".
func (t *TreatmentTimeEntry) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate".
func (t *TreatmentTimeEntry) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// MarkDone flips a pending/skipped entry to done (bugs.md R5-3b toggle).
func (t *TreatmentTimeEntry) MarkDone(at time.Time, userID uuid.UUID) {
	t.Status = TreatmentEntryStatusDone
	t.AppliedAt = nulls.NewTime(at)
	t.UserID = nulls.NewUUID(userID)
}

// Revert flips a done entry back to pending, clearing the apply stamp
// (bugs.md R5-3b unapply).
func (t *TreatmentTimeEntry) Revert() {
	t.Status = TreatmentEntryStatusPending
	t.AppliedAt = nulls.Time{}
	t.UserID = nulls.UUID{}
	t.ApplicationID = nulls.UUID{}
}
