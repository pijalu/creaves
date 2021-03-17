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

// Veterinaryvisit is used by pop to map your veterinaryvisits database table to your go code.
type Veterinaryvisit struct {
	ID         uuid.UUID    `json:"id" db:"id"`
	Date       time.Time    `json:"date" db:"date"`
	UserID     uuid.UUID    `json:"user_id" db:"user_id"`
	User       User         `json:"user,omitempty" belongs_to:"user"`
	AnimalID   int          `json:"animal_id" db:"animal_id"`
	Animal     Animal       `json:"animal,omitempty" belongs_to:"animal"`
	Diagnostic nulls.String `json:"diagnostic" db:"diagnostic"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (v Veterinaryvisit) String() string {
	jv, _ := json.Marshal(v)
	return string(jv)
}

// Veterinaryvisits is not required by pop and may be deleted
type Veterinaryvisits []Veterinaryvisit

// String is not required by pop and may be deleted
func (v Veterinaryvisits) String() string {
	jv, _ := json.Marshal(v)
	return string(jv)
}

// DateFormated returns a formated date
func (v Veterinaryvisit) DateFormated() string {
	return v.Date.Format(DateTimeFormat)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (v *Veterinaryvisit) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.TimeIsPresent{Field: v.Date, Name: "Date"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (v *Veterinaryvisit) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (v *Veterinaryvisit) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
