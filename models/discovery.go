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

// Discovery is used by pop to map your .model.Name.Proper.Pluralize.Underscore database table to your go code.
type Discovery struct {
	ID           uuid.UUID    `json:"id" db:"id"`
	Location     nulls.String `json:"location" db:"location"`
	PostalCode   nulls.String `json:"postal_code" db:"postal_code"`
	Village      nulls.String `json:"village" db:"village"`
	Date         time.Time    `json:"date" db:"date"`
	Reason       nulls.String `json:"reason" db:"reason"`
	Note         nulls.String `json:"note" db:"note"`
	Discoverer   Discoverer   `belongs_to:"discoverer" json:"discoverer,omitempty"`
	DiscovererID uuid.UUID    `json:"discoverer_id" db:"discoverer_id"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at" db:"updated_at"`
}

// DateFormated returns a formated date
func (d Discovery) DateFormated() string {
	return d.Date.Format(DateTimeFormat)
}

// String is not required by pop and may be deleted
func (d Discovery) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Discoveries is not required by pop and may be deleted
type Discoveries []Discovery

// String is not required by pop and may be deleted
func (d Discoveries) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (d *Discovery) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		// Discovery
		&validators.TimeIsPresent{Field: d.Date, Name: "Date"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (d *Discovery) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (d *Discovery) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
