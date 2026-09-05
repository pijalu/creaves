package models

import (
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

type ResyncRun struct {
	ID                     uuid.UUID  `json:"id" db:"id"`
	InstanceID             string     `json:"instance_id" db:"instance_id"`
	Status                 string     `json:"status" db:"status"`
	StartedAt              time.Time  `json:"started_at" db:"started_at"`
	FinishedAt             *time.Time `json:"finished_at" db:"finished_at"`
	TotalAnimals           int        `json:"total_animals" db:"total_animals"`
	AnimalsProcessed       int        `json:"animals_processed" db:"animals_processed"`
	EventsCreated          int        `json:"events_created" db:"events_created"`
	EventsSkippedUnchanged int        `json:"events_skipped_unchanged" db:"events_skipped_unchanged"`
	EventsDelivered        int        `json:"events_delivered" db:"events_delivered"`
	EventsFailed           int        `json:"events_failed" db:"events_failed"`
	// Producer-announced expected sync state, computed once at run start
	// (same live payload builder as ComputeSyncStatus) and echoed to the
	// console in every delivery envelope of this run ("sync" block). The
	// console stores it on the instance row and displays
	// stored/announced(total) plus a checksum comparison against it.
	AnnouncedExpectedTotal    int        `json:"announced_expected_total" db:"announced_expected_total"`
	AnnouncedExpectedChecksum *string    `json:"announced_expected_checksum" db:"announced_expected_checksum"`
	AnnouncedAt               *time.Time `json:"announced_at" db:"announced_at"`
	Errors                    string     `json:"errors" db:"errors"`
	CreatedAt                 time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt                 time.Time  `json:"updated_at" db:"updated_at"`
}

func (r *ResyncRun) Validate(*pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
func (r *ResyncRun) ValidateCreate(*pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
func (r *ResyncRun) ValidateUpdate(*pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
func (r *ResyncRun) Complete(now time.Time) { r.Status = "completed"; r.FinishedAt = &now }
func (r *ResyncRun) Fail(now time.Time, err error) {
	r.Status = "failed"
	r.FinishedAt = &now
	if err != nil {
		r.Errors = err.Error()
	}
}
func (r *ResyncRun) Cancel(now time.Time) { r.Status = "cancelled"; r.FinishedAt = &now }
