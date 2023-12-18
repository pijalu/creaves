package models

import (
	"encoding/json"
	"net/url"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Zone is used by pop to map your zones database table to your go code.
type Zone struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Zone      string    `json:"zone" db:"zone"`
	Type      string    `json:"type" db:"type"`
	Default   bool      `json:"default" db:"default"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type ZoneKey struct {
	Zone string `json:"zone"`
}

func (z ZoneKey) ZoneEscape() string {
	return url.PathEscape(z.Zone)
}

// String is not required by pop and may be deleted
func (z Zone) String() string {
	jz, _ := json.Marshal(z)
	return string(jz)
}

// Zones is not required by pop and may be deleted
type Zones []Zone

// String is not required by pop and may be deleted
func (z Zones) String() string {
	jz, _ := json.Marshal(z)
	return string(jz)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (z *Zone) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: z.Zone, Name: "Zone"},
		&validators.StringIsPresent{Field: z.Type, Name: "Type"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (z *Zone) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (z *Zone) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
