package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// SubsideGroup is used by pop to map your subside_groups database table to your go code.
type SubsideGroup struct {
	ID        string    `json:"id" db:"id"`
	Group     string    `json:"group" db:"group"`
	Size      int       `json:"size" db:"size"`
	Amount    float64   `json:"amount" db:"amount"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (s SubsideGroup) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// SubsideGroups is not required by pop and may be deleted
type SubsideGroups []SubsideGroup

// String is not required by pop and may be deleted
func (s SubsideGroups) String() string {
	js, _ := json.Marshal(s)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (s *SubsideGroup) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: s.ID, Name: "ID"},
		&validators.StringIsPresent{Field: s.Group, Name: "Group"},
		&validators.IntIsPresent{Field: s.Size, Name: "Size"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (s *SubsideGroup) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (s *SubsideGroup) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
