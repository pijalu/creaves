package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// Locality is used by pop to map your localities database table to your go code.
type Locality struct {
	ID              string    `json:"id" db:"id"`
	Country         string    `json:"country" db:"country"`
	Region          string    `json:"region" db:"region"`
	Province        string    `json:"province" db:"province"`
	Municipality    string    `json:"municipality" db:"municipality"`
	SubMunicipality bool      `json:"sub_municipality" db:"sub_municipality"`
	PostalCode      string    `json:"postal_code" db:"postal_code"`
	Locality        string    `json:"locality" db:"locality"`
	Zoning          string    `json:"zoning" db:"zoning"`
	Direction       string    `json:"direction" db:"direction"`
	CreatedAt       time.Time `json:"created_at" db:"created_at"`
	UpdatedAt       time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (l Locality) String() string {
	jl, _ := json.Marshal(l)
	return string(jl)
}

// Localities is not required by pop and may be deleted
type Localities []Locality

// String is not required by pop and may be deleted
func (l Localities) String() string {
	jl, _ := json.Marshal(l)
	return string(jl)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (l *Locality) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: l.ID, Name: "ID"},
		&validators.StringIsPresent{Field: l.Country, Name: "Country"},
		&validators.StringIsPresent{Field: l.Region, Name: "Region"},
		&validators.StringIsPresent{Field: l.Province, Name: "Province"},
		&validators.StringIsPresent{Field: l.Municipality, Name: "Municipality"},
		&validators.StringIsPresent{Field: l.PostalCode, Name: "PostalCode"},
		&validators.StringIsPresent{Field: l.Locality, Name: "Locality"},
		&validators.StringIsPresent{Field: l.Zoning, Name: "Zoning"},
		&validators.StringIsPresent{Field: l.Direction, Name: "Direction"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (l *Locality) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (l *Locality) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
