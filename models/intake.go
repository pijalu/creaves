package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Intake is used by pop to map your intakes database table to your go code.
type Intake struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	Date      time.Time    `json:"date" db:"date"`
	General   string       `json:"general" db:"general"`
	Wounds    nulls.String `json:"wounds" db:"wounds"`
	Parasites nulls.String `json:"parasites" db:"parasites"`
	Remarks   nulls.String `json:"remarks" db:"remarks"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

// DateFormated returns a formated date
func (i Intake) DateFormated() string {
	return i.Date.Format(DateTimeFormat)
}

// String is not required by pop and may be deleted
func (i Intake) String() string {
	ji, _ := json.Marshal(i)
	return string(ji)
}

// Intakes is not required by pop and may be deleted
type Intakes []Intake

// String is not required by pop and may be deleted
func (i Intakes) String() string {
	ji, _ := json.Marshal(i)
	return string(ji)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (i *Intake) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: i.General, Name: "General"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (i *Intake) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (i *Intake) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
