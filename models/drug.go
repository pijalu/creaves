package models

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Dosage is the Drug dosage
type Dosage struct {
	ID uuid.UUID `json:"id" db:"id"`

	DrugID uuid.UUID `json:"drug_id" db:"drug_id"`
	Drug   *Drug     `json:"-" belongs_to:"drug"`

	AnimaltypeID uuid.UUID   `json:"animaltype_id" db:"animaltype_id"`
	Animaltype   *Animaltype `json:"-" belongs_to:"animaltype"`

	Enabled            bool          `json:"enabled" db:"enabled"`
	Description        nulls.String  `json:"description" db:"description"`
	DosagePerGrams     nulls.Float64 `json:"dosage_per_grams" db:"dosage_per_grams"`
	DosagePerGramsUnit nulls.String  `json:"dosage_per_grams_unit" db:"dosage_per_grams_unit"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (d Dosage) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

func (d Dosage) PerKilo() nulls.Float64 {
	if d.DosagePerGrams.Valid {
		result, err := strconv.ParseFloat(fmt.Sprintf("%.4f", d.DosagePerGrams.Float64*1000), 64)
		if err != nil {
			result = 0
		}
		return nulls.NewFloat64(result)
	}
	return d.DosagePerGrams
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
	Dosages     []Dosage     `json:"dosages" has_many:"dosages"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// String is not required by pop and may be deleted
func (d Drug) String() string {
	jd, _ := json.Marshal(d)
	return string(jd)
}

// DosagePerAnimalTypeID returns the dosage remapped by animal type
func (d Drug) DosagePerAnimalTypeID() map[uuid.UUID]Dosage {
	r := map[uuid.UUID]Dosage{}
	for _, ds := range d.Dosages {
		r[ds.AnimaltypeID] = ds
	}
	return r
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
