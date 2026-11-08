package models

import (
	"creaves/utils"
	"encoding/json"
	"strings"
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

// BeforeSave normalizes the phone on every write path (direct saves and
// eager association saves alike) — see normalizeBelgianPhone (#208.10).
func (d *Discoverer) BeforeSave(tx *pop.Connection) error {
	utils.TrimStringFields(d)
	d.Phone = normalizeLocalPhone(d.Phone)
	return nil
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (d *Discoverer) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(d)
	d.Phone = normalizeLocalPhone(d.Phone)
	return validate.Validate(), nil
}

// normalizeBelgianPhone rewrites a Belgian phone number entered in the
// international formats "+32470112233" or "0032470112233" to the local
// format "0470112233" (#208.10). Numbers without a Belgian country code are
// returned unchanged (only trimmed).
func normalizeBelgianPhone(s string) string {
	s = strings.TrimSpace(s)
	rest := s
	switch {
	case strings.HasPrefix(rest, "+32"):
		rest = rest[3:]
	case strings.HasPrefix(rest, "0032"):
		rest = rest[4:]
	default:
		return s
	}
	// keep only the digits of the national part
	var b strings.Builder
	for _, r := range rest {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()
	if digits == "" {
		return s
	}
	return "0" + digits
}

// normalizeLocalPhone applies normalizeBelgianPhone to a nullable phone.
func normalizeLocalPhone(p nulls.String) nulls.String {
	if !p.Valid {
		return p
	}
	return nulls.NewString(normalizeBelgianPhone(p.String))
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
