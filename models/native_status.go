package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// NativeStatus is used by pop to map your native_statuses database table to your go code.
type NativeStatus struct {
	ID         string       `json:"id" db:"id"`
	Status     string       `json:"status" db:"status"`
	Indication string       `json:"indication" db:"indication"`
	Freeable   bool         `json:"freeable" db:"freeable"`
	Precision  nulls.String `json:"precision" db:"precision"`
	CreatedAt  time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time    `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (n NativeStatus) String() string {
	jn, _ := json.Marshal(n)
	return string(jn)
}

// NativeStatuses is not required by pop and may be deleted
type NativeStatuses []NativeStatus

// String is not required by pop and may be deleted
func (n NativeStatuses) String() string {
	jn, _ := json.Marshal(n)
	return string(jn)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (n *NativeStatus) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: n.ID, Name: "ID"},
		&validators.StringIsPresent{Field: n.Status, Name: "Status"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (n *NativeStatus) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (n *NativeStatus) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
