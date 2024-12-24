package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
)

// EntryCause is used by pop to map your entry_causes database table to your go code.
type EntryCause struct {
	ID         string    `json:"id" db:"id"`
	Cause      string    `json:"cause" db:"cause"`
	Detail     string    `json:"detail" db:"detail"`
	Nature     string    `json:"nature" db:"nature"`
	Indication string    `json:"indication" db:"indication"`
	SortOrder  int       `json:"sort_order" db:"sort_order"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (e EntryCause) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

// EntryCauses is not required by pop and may be deleted
type EntryCauses []EntryCause

// String is not required by pop and may be deleted
func (e EntryCauses) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

func (e EntryCause) Fmt(withId bool) string {
	var prefix = ""
	if withId {
		prefix = fmt.Sprintf("%s - ", e.ID)
	}
	if e.Cause == e.Detail {
		return fmt.Sprintf("%s%s", prefix, e.Cause)
	}
	return fmt.Sprintf("%s%s ➤ %s", prefix, e.Cause, e.Detail)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (e *EntryCause) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: e.ID, Name: "ID"},
		&validators.StringIsPresent{Field: e.Cause, Name: "Cause"},
		&validators.StringIsPresent{Field: e.Detail, Name: "Detail"},
		&validators.StringIsPresent{Field: e.Nature, Name: "Nature"},
		&validators.StringIsPresent{Field: e.Indication, Name: "Indication"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (e *EntryCause) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (e *EntryCause) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
