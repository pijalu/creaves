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

// Outtake is used by pop to map your outtakes database table to your go code.
type Outtake struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	Animal    Animal       `json:"animal,omitempty" has_one:"animal"`
	Date      time.Time    `json:"date" db:"date"`
	Type      Outtaketype  `json:"type" belongs_to:"outtaketype"`
	TypeID    uuid.UUID    `json:"type_id" db:"outtaketype_id"`
	Location  nulls.String `json:"location" db:"location"`
	Note      nulls.String `json:"note" db:"note"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

// DateFormated returns a formated date
func (o Outtake) DateFormated() string {
	return o.Date.Format(DateTimeFormat)
}

// String is not required by pop and may be deleted
func (o Outtake) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// Outtakes is not required by pop and may be deleted
type Outtakes []Outtake

// String is not required by pop and may be deleted
func (o Outtakes) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (o *Outtake) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.TimeIsPresent{Field: o.Date, Name: "Date"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (o *Outtake) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (o *Outtake) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
