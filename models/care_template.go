package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// CareTemplate is a reusable note template a user can insert into the note
// field of a "suivi" care (issue #158). Templates are private to their
// creator; admins may delete any.
type CareTemplate struct {
	ID      uuid.UUID `json:"id" db:"id"`
	UserID  uuid.UUID `json:"user_id" db:"user_id"`
	Name    string    `json:"name" db:"name"`
	Content string    `json:"content" db:"content"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// CareTemplates is not required by pop and may be deleted
type CareTemplates []CareTemplate

// String is not required by pop and may be deleted
func (t CareTemplate) String() string {
	js, _ := json.Marshal(t)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (t *CareTemplate) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: t.Name, Name: "Name"},
		&validators.StringIsPresent{Field: t.Content, Name: "Content"},
	), nil
}
