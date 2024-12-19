package models

import (
	"creaves/utils"
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Travel is used by pop to map your .model.Name.Proper.Pluralize.Underscore database table to your go code.
type Travel struct {
	ID           uuid.UUID    `json:"id" db:"id"`
	Date         time.Time    `json:"date" db:"date"`
	AnimalID     int          `json:"animal_id" db:"animal_id"`
	Animal       *Animal      `json:"animal,omitempty" belongs_to:"animal"`
	UserID       uuid.UUID    `json:"user_id" db:"user_id"`
	User         *User        `json:"user" belongs_to:"user"`
	TraveltypeID uuid.UUID    `json:"traveltype_id" db:"traveltype_id"`
	Traveltype   *Traveltype  `json:"traveltype,omitempty" belongs_to:"traveltype"`
	TypeDetails  nulls.String `json:"type_details" db:"type_details"`
	Distance     int          `json:"distance" db:"distance"`
	Details      nulls.String `json:"details" db:"details"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (t Travel) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Travels is not required by pop and may be deleted
type Travels []Travel

// String is not required by pop and may be deleted
func (t Travels) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// DateFormated returns a formated date
func (t *Travel) DateFormated() string {
	return t.Date.Format(DateTimeFormat)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (t *Travel) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(t)
	return validate.Validate(
		&validators.IntIsPresent{Field: t.Distance, Name: "Distance"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (t *Travel) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (t *Travel) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
