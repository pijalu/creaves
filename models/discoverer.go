package models

import (
	"creaves/utils"
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// Discoverer is used by pop to map your .model.Name.Proper.Pluralize.Underscore database table to your go code.
type Discoverer struct {
	ID            uuid.UUID    `json:"id" db:"id"`
	Firstname     nulls.String `json:"firstname" db:"firstname"`
	Lastname      nulls.String `json:"lastname" db:"lastname"`
	Address       nulls.String `json:"address" db:"address"`
	PostalCode    nulls.String `json:"postal_code" db:"postal_code"`
	City          nulls.String `json:"city" db:"city"`
	Country       nulls.String `json:"country" db:"country"`
	Email         nulls.String `json:"email" db:"email"`
	Phone         nulls.String `json:"phone" db:"phone"`
	Note          nulls.String `json:"note" db:"note"`
	ReturnRequest bool         `json:"return_request" db:"return_request"`
	Donation      nulls.String `json:"donation" db:"donation"`
	CreatedAt     time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at" db:"updated_at"`
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
	utils.TrimStringFields(d)
	return validate.Validate(), nil
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
