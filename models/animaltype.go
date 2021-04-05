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

// Animaltype is used by pop to map your animaltypes database table to your go code.
type Animaltype struct {
	ID          uuid.UUID    `json:"id" db:"id"`
	Name        string       `json:"name" db:"name"`
	Default     bool         `json:"default" db:"def"`
	Description nulls.String `json:"description" db:"description"`
	HasRing     bool         `json:"has_ring" db:"has_ring"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (a Animaltype) String() string {
	ja, _ := json.Marshal(a)
	return string(ja)
}

// Animaltypes is not required by pop and may be deleted
type Animaltypes []Animaltype

// String is not required by pop and may be deleted
func (a Animaltypes) String() string {
	ja, _ := json.Marshal(a)
	return string(ja)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (a *Animaltype) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: a.Name, Name: "Name"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (a *Animaltype) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (a *Animaltype) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
