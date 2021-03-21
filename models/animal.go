package models

import (
	"encoding/json"
	"sort"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// Animal is used by pop to map your animals database table to your go code.
type Animal struct {
	ID           int               `json:"id" db:"id"`
	Ring         nulls.String      `json:"ring" db:"ring"`
	Species      string            `json:"species" db:"species"`
	Cage         nulls.String      `json:"cage" db:"cage"`
	Feeding      nulls.String      `json:"feeding" db:"feeding"`
	Animalage    Animalage         `json:"animalage" belongs_to:"animalage"`
	AnimalageID  uuid.UUID         `json:"animalage_id" db:"animalage_id"`
	Animaltype   Animaltype        `json:"animaltype" belongs_to:"animaltype"`
	AnimaltypeID uuid.UUID         `json:"animaltype_id" db:"animaltype_id"`
	Discovery    Discovery         `json:"discovery" belongs_to:"discovery"`
	DiscoveryID  uuid.UUID         `json:"discovery_id" db:"discovery_id"`
	Intake       Intake            `json:"intake" belongs_to:"intake"`
	IntakeID     uuid.UUID         `json:"intake_id" db:"intake_id"`
	Outtake      *Outtake          `json:"outtake,omitempty" belongs_to:"outtake"`
	OuttakeID    nulls.UUID        `json:"outtake_id" db:"outtake_id"`
	Cares        []Care            `json:"cares,omitempty" has_many:"cares"`
	Treatments   Treatments        `json:"treatmentes,omitempty" has_many:"treatments"`
	VetVisits    []Veterinaryvisit `json:"veternary_visits,omitempty" has_many:"veternaryvisits"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at" db:"updated_at"`
}

// Animals is not required by pop and may be deleted
type Animals []Animal

// TreatmentsMap is an organized treaments list
type AnimalsByTypeMap map[Animaltype]Animals

// Return orderedkeys from map
func (t AnimalsByTypeMap) OrderedKeys() []Animaltype {
	var keys []Animaltype
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys
}

// String is not required by pop and may be deleted
func (a Animal) String() string {
	ja, _ := json.Marshal(a)
	return string(ja)
}

// String is not required by pop and may be deleted
func (a Animals) String() string {
	ja, _ := json.Marshal(a)
	return string(ja)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (a *Animal) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (a *Animal) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (a *Animal) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
