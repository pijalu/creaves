package models

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// Animal is used by pop to map your animals database table to your go code.
type Animal struct {
	ID           int               `json:"id" db:"id"`
	Year         int               `json:"Year" db:"year"`
	YearNumber   int               `json:"YearNumber" db:"yearNumber"`
	Ring         nulls.String      `json:"ring" db:"ring"`
	Species      string            `json:"species" db:"species"`
	Gender       nulls.String      `json:"gender" db:"gender"`
	Cage         nulls.String      `json:"cage" db:"cage"`
	Zone         nulls.String      `json:"zone" db:"zone"`
	Feeding      nulls.String      `json:"feeding" db:"feeding"`
	ForceFeed    bool              `json:"forceFeed" db:"force_feed"`
	Animalage    Animalage         `json:"animalage" belongs_to:"animalage"`
	AnimalageID  uuid.UUID         `json:"animalage_id" db:"animalage_id"`
	Animaltype   Animaltype        `json:"animaltype" belongs_to:"animaltype"`
	AnimaltypeID uuid.UUID         `json:"animaltype_id" db:"animaltype_id"`
	Discovery    Discovery         `json:"discovery" belongs_to:"discovery"`
	DiscoveryID  uuid.UUID         `json:"discovery_id" db:"discovery_id"`
	Intake       Intake            `json:"dssaaintake" belongs_to:"intake"`
	IntakeID     uuid.UUID         `json:"intake_id" db:"intake_id"`
	Outtake      *Outtake          `json:"outtake,omitempty" belongs_to:"outtake"`
	OuttakeID    nulls.UUID        `json:"outtake_id" db:"outtake_id"`
	Cares        []Care            `json:"cares,omitempty" has_many:"cares"`
	Treatments   Treatments        `json:"treatmentes,omitempty" has_many:"treatments"`
	VetVisits    []Veterinaryvisit `json:"veternary_visits,omitempty" has_many:"veternaryvisits"`
	IntakeDate   time.Time         `json:"intakeDate" db:"IntakeDate"`
	CreatedAt    time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at" db:"updated_at"`
}

// View keys
type AnimalViewKey struct {
	ID   string
	Name string
}

// Animals is not required by pop and may be deleted
type Animals []Animal

// TreatmentsMap is an organized treaments list
type AnimalsByTypeMap map[AnimalViewKey]Animals

// Return orderedkeys from map
func (t AnimalsByTypeMap) OrderedKeys() []AnimalViewKey {
	var keys []AnimalViewKey
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys
}

type AnimalByZoneMap map[AnimalViewKey]Animals

// Return orderedkeys from map
func (t AnimalByZoneMap) OrderedKeys() []AnimalViewKey {
	var keys []AnimalViewKey
	for k := range t {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Name < keys[j].Name
	})
	return keys
}

// YearNumberFormatted returns the year number formatted
func (a Animal) YearNumberFormatted() string {
	return fmt.Sprintf("%d/%d", a.YearNumber, a.Year%100)
}

// YearNumberFormatted returns the year number formatted
func (a Animal) ZoneAsString() string {
	return a.Zone.String
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

// LastWeight returns the last weight of the animal
func (a Animal) LastWeight() nulls.Int {
	// No cares
	if len(a.Cares) < 1 {
		return nulls.Int{}
	}
	maxDate := a.Cares[0].Date
	w := a.Cares[0].Weight
	for i := 1; i < len(a.Cares); i++ {
		c := a.Cares[i]
		if len(strings.TrimSpace(w.String)) == 0 ||
			len(strings.TrimSpace(c.Weight.String)) > 0 && maxDate.Before(c.Date) {
			maxDate = c.Date
			w = c.Weight
		}
	}

	// return value
	if w.Valid {
		i, err := strconv.Atoi(w.String)
		if err == nil {
			return nulls.NewInt(i)
		}
	}

	return nulls.Int{}
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
