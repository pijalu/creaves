package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop"
	"github.com/gobuffalo/validate"
	"github.com/gobuffalo/validate/validators"
	"github.com/gofrs/uuid"
)

// Dosage is the Drug dosage
type Dosage struct {
	ID uuid.UUID `json:"id" db:"id"`

	DrugID uuid.UUID `json:"drug_id" db:"drug_id"`
	Drug   *Drug     `belongs_to:"drug"`

	AnimaltypeID uuid.UUID   `json:"animaltype_id" db:"animaltype_id"`
	Animaltype   *Animaltype `belongs_to:"animaltype"`

	Enabled        bool          `json:"enabled" db:"enabled"`
	Description    nulls.String  `json:"description" db:"description"`
	DosagePerGrams nulls.Float64 `json:"dosage_per_grams" db:"dosage_per_grams"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (d Dosage) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Dosages is a list of dosages
type Dosages []Dosage

// String render a list of dosage to string
func (d Dosages) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Drug is used by pop to map your .model.Name.Proper.Pluralize.Underscore database table to your go code.
type Drug struct {
	ID uuid.UUID `json:"id" db:"id"`

	Name        string       `json:"name" db:"name"`
	Description nulls.String `json:"description" db:"description"`
	Dosages     []Dosage     `json:"dosages,omitempty" has_many:"dosages"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (d Drug) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Drugs is not required by pop and may be deleted
type Drugs []Drug

// String is not required by pop and may be deleted
func (d Drugs) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (d *Drug) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.StringIsPresent{Field: d.Name, Name: "Name"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (d *Drug) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (d *Drug) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
