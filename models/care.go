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

// Care is used by pop to map your cares database table to your go code.
type Care struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Date time.Time `json:"date" db:"date"`

	AnimalID int    `json:"animal_id" db:"animal_id"`
	Animal   Animal `json:"animal,omitempty" belongs_to:"animal"`

	Type   Caretype  `json:"type" belongs_to:"caretype"`
	TypeID uuid.UUID `json:"type_id" db:"type_id"`

	Weight    nulls.String `json:"weight" db:"weight"`
	Note      nulls.String `json:"note" db:"note"`
	Clean     nulls.Bool   `json:"clean" db:"clean"`
	InWarning nulls.Bool   `json:"in_warning" db:"in_warning"`

	LinkToID nulls.UUID `json:"link_to_id" db:"link_to_id"`
	LinkTo   *Care      `json:"link_to,omitempty" belongs_to:"care"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (c Care) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// DateFormated returns a formated date
func (c Care) DateFormated() string {
	return c.Date.Format(DateTimeFormat)
}

// Cares is not required by pop and may be deleted
type Cares []Care

// String is not required by pop and may be deleted
func (c Cares) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (c *Care) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.TimeIsPresent{Field: c.Date, Name: "Date"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (c *Care) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (c *Care) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
