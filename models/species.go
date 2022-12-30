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

// Species is used by pop to map your species database table to your go code.
type Species struct {
	ID             uuid.UUID     `json:"id" db:"id"`
	Species        string        `json:"species" db:"species"`
	Group          string        `json:"group" db:"group"`
	Family         string        `json:"family" db:"family"`
	CreavesSpecies string        `json:"creaves_species" db:"creaves_species"`
	CreavesGroup   string        `json:"creaves_group" db:"creaves_group"`
	Subside        nulls.Float64 `json:"subside" db:"subside"`
	CreatedAt      time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (s Species) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (s *Species) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: s.Species, Name: "Species"},
		&validators.StringIsPresent{Field: s.Group, Name: "Group"},
		&validators.StringIsPresent{Field: s.Family, Name: "Family"},
		&validators.StringIsPresent{Field: s.CreavesSpecies, Name: "CreavesSpecies"},
		&validators.StringIsPresent{Field: s.CreavesGroup, Name: "CreavesGroup"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (s *Species) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (s *Species) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
