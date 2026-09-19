package models

import (
	"creaves/utils"
	"encoding/json"
	"strings"
	"time"
	"unicode"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
	"golang.org/x/text/unicode/norm"
)

// Outtake is used by pop to map your outtakes database table to your go code.
type Outtake struct {
	ID       uuid.UUID    `json:"id" db:"id"`
	Animal   Animal       `json:"animal,omitempty" has_one:"animal"`
	Date     time.Time    `json:"date" db:"date"`
	Type     Outtaketype  `json:"type" belongs_to:"outtaketype"`
	TypeID   uuid.UUID    `json:"type_id" db:"outtaketype_id"`
	Location nulls.String `json:"location" db:"location"`
	Note     nulls.String `json:"note" db:"note"`
	// Stay duration in whole hours between the animal intake date and the
	// outtake date (issue #175). NULL for outtakes encoded before the
	// column existed.
	StayDuration nulls.Int `json:"stay_duration" db:"stay_duration"`
	// Corpse destination tracking (issue #149). Only meaningful when the
	// outtake type has Dead=true, i.e. the outtake produced a corpse.
	// Destination is free text with autocomplete from previously used
	// values; By is the user who recorded the marking (set server-side).
	CorpseDestination     nulls.String `json:"corpse_destination" db:"corpse_destination"`
	CorpseDestinationAt   nulls.Time   `json:"corpse_destination_at" db:"corpse_destination_at"`
	CorpseDestinationByID nulls.UUID   `json:"corpse_destination_by_id" db:"corpse_destination_by_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}

func (o Outtake) IsSelected(value interface{}) bool {
	return o.TypeID == value.(uuid.UUID)
}

// DateFormated returns a formated date
func (o Outtake) DateFormated() string {
	return o.Date.Format(DateTimeFormat)
}

// ComputeStayDuration sets StayDuration to the whole hours between the
// animal intake date and the outtake date (issue #175). Negative stays are
// clamped to 0; missing dates leave the value untouched.
func (o *Outtake) ComputeStayDuration(intake time.Time) {
	if intake.IsZero() || o.Date.IsZero() {
		return
	}
	h := int(o.Date.Sub(intake).Hours())
	if h < 0 {
		h = 0
	}
	o.StayDuration = nulls.NewInt(h)
}

// String is not required by pop and may be deleted
func (o Outtake) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// Outtakes is not required by pop and may be deleted
type Outtakes []Outtake

// String is not required by pop and may be deleted
func (o Outtakes) String() string {
	jo, _ := json.Marshal(o)
	return string(jo)
}

// deaccent returns s with diacritics removed ("indigénat" → "indigenat")
// so guards on French names also match accented spellings.
func deaccent(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (o *Outtake) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(o)
	errs := validate.Validate(
		&validators.TimeIsPresent{Field: o.Date, Name: "Date"},
	)
	// No outtake date in the future (issue #175). Small tolerance absorbs
	// sub-second skew between the encoder's clock and freshly-picked "now".
	if !o.Date.IsZero() && o.Date.After(time.Now().Add(time.Minute)) {
		errs.Add("Date", "cannot be in the future")
	}
	// "ID d'indigénat" is an entry cause and must never be usable as an
	// outtake cause (issue #175). Accents are normalized so every spelling
	// of "indigénat" is caught.
	if o.TypeID != uuid.Nil {
		ot := &Outtaketype{}
		if err := tx.Find(ot, o.TypeID); err == nil && strings.Contains(strings.ToLower(deaccent(ot.Name)), "indigen") {
			errs.Add("TypeID", "ID d'indigénat is not allowed as an outtake cause")
		}
	}
	return errs, nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (o *Outtake) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (o *Outtake) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
