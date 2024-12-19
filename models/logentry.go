package models

import (
	"creaves/utils"
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Logentry is used by pop to map your logentries database table to your go code.
type Logentry struct {
	ID          uuid.UUID `json:"id" db:"id"`
	User        User      `belongs_to:"user" json:"user"`
	UserID      uuid.UUID `json:"user_id" db:"user_id"`
	Description string    `json:"description" db:"description"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (l Logentry) String() string {
	jl, _ := json.Marshal(l)
	return string(jl)
}

// Logentries is not required by pop and may be deleted
type Logentries []Logentry

// String is not required by pop and may be deleted
func (l Logentries) String() string {
	jl, _ := json.Marshal(l)
	return string(jl)
}

// CreatedAtFormated returns a formated date
func (l Logentry) CreatedAtFormated() string {
	return l.CreatedAt.Format(DateTimeFormat)
}

// CreatedAtFormated returns a formated date
func (l Logentry) UpdatedAtFormated() string {
	return l.UpdatedAt.Format(DateTimeFormat)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (l *Logentry) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(l)
	return validate.Validate(
		&validators.StringIsPresent{Field: l.Description, Name: "Description"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (l *Logentry) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (l *Logentry) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
