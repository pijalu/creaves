package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Treatment is used by pop to map your treatments database table to your go code.
type Treatment struct {
	ID             uuid.UUID    `json:"id" db:"id"`
	Date           time.Time    `json:"date" db:"date"`
	AnimalID       int          `json:"animal_id" db:"animal_id"`
	Animal         *Animal      `json:"animal,omitempty" belongs_to:"animal"`
	Drug           string       `json:"drug" db:"drug"`
	Dosage         string       `json:"dosage" db:"dosage"`
	Remarks        nulls.String `json:"remarks" db:"remarks"`
	Timebitmap     int          `json:"timebitmap" db:"timebitmap"`
	Timedonebitmap int          `json:"timedonebitmap" db:"timedonebitmap"`
	CreatedAt      time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time    `json:"updated_at" db:"updated_at"`
}

// TreatmentTemplate is the object used to create a serie of treaments
type TreatmentTemplate struct {
	DateFrom time.Time `json:"dateFrom"`
	DateTo   time.Time `json:"dateTo"`

	AnimalID int          `json:"animal_id"`
	Animal   *Animal      `json:"animal,omitempty" belongs_to:"animal"`
	Drug     string       `json:"drug"`
	Dosage   string       `json:"dosage"`
	Remarks  nulls.String `json:"remarks"`

	Morning bool `json:"morning"`
	Noon    bool `json:"noon"`
	Evening bool `json:"evening"`
}

// Time bitmaps
const (
	Treatement_MORNING = 1
	Treatement_NOON    = 2
	Treatement_EVENING = 4
)

func TreatmentBoolToBitmap(morning bool, noon bool, evening bool) int {
	bitmap := 0
	if morning {
		bitmap += Treatement_MORNING
	}
	if noon {
		bitmap += Treatement_NOON
	}
	if evening {
		bitmap += Treatement_EVENING
	}
	return bitmap
}

// DateFromFormated return date formated
func (t *TreatmentTemplate) DateFromFormated() string {
	return t.DateFrom.Format(DateFormat)
}

// DateToFormated return date formated
func (t *TreatmentTemplate) DateToFormated() string {
	return t.DateTo.Format(DateFormat)
}

// String is not required by pop and may be deleted
func (t *TreatmentTemplate) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// String is not required by pop and may be deleted
func (t Treatment) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// Treatments is not required by pop and may be deleted
type Treatments []Treatment

// String is not required by pop and may be deleted
func (t Treatments) String() string {
	jt, _ := json.Marshal(t)
	return string(jt)
}

// DateFormated returns a formated date
func (t *Treatment) DateFormated() string {
	return t.Date.Format(DateFormat)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (t *Treatment) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.Validate(
		&validators.TimeIsPresent{Field: t.Date, Name: "Date"},
		&validators.IntIsPresent{Field: t.AnimalID, Name: "AnimalID"},
		&validators.StringIsPresent{Field: t.Drug, Name: "Drug"},
		&validators.StringIsPresent{Field: t.Dosage, Name: "Dosage"},
		&validators.IntIsPresent{Field: t.Timebitmap, Name: "Timebitmap"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (t *Treatment) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (t *Treatment) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

func (t *Treatment) ScheduleRequired(key int) bool {
	return (t.Timebitmap & key) > 0
}

func (t *Treatment) ScheduleRequiredMorning() bool {
	return t.ScheduleRequired(Treatement_MORNING)
}

func (t *Treatment) ScheduleRequiredNoon() bool {
	return t.ScheduleRequired(Treatement_NOON)
}

func (t *Treatment) ScheduleRequiredEvening() bool {
	return t.ScheduleRequired(Treatement_EVENING)
}

func (t *Treatment) ScheduleStatus(key int) nulls.Bool {
	if t.ScheduleRequired(key) {
		return nulls.NewBool((t.Timedonebitmap & key) > 0)
	}
	return nulls.Bool{Valid: false}
}

func (t *Treatment) ScheduleStatusMorning() nulls.Bool {
	return t.ScheduleStatus(Treatement_MORNING)
}

func (t *Treatment) ScheduleStatusNoon() nulls.Bool {
	return t.ScheduleStatus(Treatement_NOON)
}

func (t *Treatment) ScheduleStatusEvening() nulls.Bool {
	return t.ScheduleStatus(Treatement_EVENING)
}

func (t *Treatment) SetAllScheduleRequired(m bool, n bool, e bool) {
	t.Timebitmap = 0
	if m {
		t.Timebitmap |= Treatement_MORNING
	}
	if n {
		t.Timebitmap |= Treatement_NOON
	}
	if e {
		t.Timebitmap |= Treatement_EVENING
	}
}

func (t *Treatment) SetAllScheduleStatus(m bool, n bool, e bool) {
	t.Timedonebitmap = 0
	if m {
		t.Timedonebitmap |= Treatement_MORNING
	}
	if n {
		t.Timedonebitmap |= Treatement_NOON
	}
	if e {
		t.Timedonebitmap |= Treatement_EVENING
	}
}
