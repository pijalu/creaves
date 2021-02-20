package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop"
	"github.com/gobuffalo/validate"
	"github.com/gobuffalo/validate/validators"
	"github.com/gofrs/uuid"
)

// Discoverer is used by pop to map your .model.Name.Proper.Pluralize.Underscore database table to your go code.
type Discoverer struct {
	ID        uuid.UUID    `json:"id" db:"id"`
	Firstname string       `json:"firstname" db:"firstname"`
	Lastname  string       `json:"lastname" db:"lastname"`
	Address   string       `json:"address" db:"address"`
	City      string       `json:"city" db:"city"`
	Country   string       `json:"country" db:"country"`
	Email     nulls.String `json:"email" db:"email"`
	Phone     nulls.String `json:"phone" db:"phone"`
	Note      nulls.String `json:"note" db:"note"`
	CreatedAt time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (d Discoverer) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Discoverers is not required by pop and may be deleted
type Discoverers []Discoverer

// String is not required by pop and may be deleted
func (d Discoverers) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (d *Discoverer) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: d.Firstname, Name: "Firstname"},
		&validators.StringIsPresent{Field: d.Lastname, Name: "Lastname"},
		&validators.StringIsPresent{Field: d.Address, Name: "Address"},
		&validators.StringIsPresent{Field: d.City, Name: "City"},
		&validators.StringIsPresent{Field: d.Country, Name: "Country"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (d *Discoverer) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (d *Discoverer) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
