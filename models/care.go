package models

import (
	"creaves/utils"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// Care is used by pop to map your cares database table to your go code.
type Care struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Date time.Time `json:"date" db:"date"`

	AnimalID int    `json:"animal_id" db:"animal_id"`
	Animal   Animal `json:"-" belongs_to:"animal"`

	Type   Caretype  `json:"type" belongs_to:"caretype"`
	TypeID uuid.UUID `json:"type_id" db:"type_id"`

	Weight    nulls.String `json:"weight" db:"weight"`
	Note      nulls.String `json:"note" db:"note"`
	Clean     nulls.Bool   `json:"clean" db:"clean"`
	InWarning nulls.Bool   `json:"in_warning" db:"in_warning"`

	// Heat source + oxygen support recorded on the care (issue #158).
	HeatSource nulls.String `json:"heat_source" db:"heat_source"`
	Oxygen     bool         `json:"oxygen" db:"oxygen"`

	LinkToID nulls.UUID `json:"link_to_id" db:"link_to_id"`
	LinkTo   *Care      `json:"link_to,omitempty" belongs_to:"care"`

	// Display annotations computed by actions.annotateCaresForDisplay for the
	// animal care tab (issue #158). Never persisted.
	WeightTrend   string `json:"weight_trend" db:"-"`
	DayGroupStart bool   `json:"day_group_start" db:"-"`
	DayGroupEnd   bool   `json:"day_group_end" db:"-"`
	IsToday       bool   `json:"is_today" db:"-"`

	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type CareWithAnimalNumber struct {
	Care
	Year       int `json:"Year" db:"year"`
	YearNumber int `json:"YearNumber" db:"yearNumber"`
}

// YearNumberFormatted returns the year number formatted
func (c CareWithAnimalNumber) YearNumberFormatted() string {
	return fmt.Sprintf("%d/%d", c.YearNumber, c.Year%100)
}

// String is not required by pop and may be deleted
func (c Care) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// DateFormated returns a formated date
func (c Care) DateFormated() string {
	return c.Date.Format(DateTimeFormat)
}

// Cares is not required by pop and may be deleted
type Cares []Care

// AnnotateCaresForDisplay computes the per-row display annotations for the
// animal care tab (issue #158): weight trend versus the chronologically
// previous weighted care ("up" green / "down" red), day-group boundaries and
// the today flag. cares must be ordered by date descending, as loaded for the
// animal page. Pure function over the slice — safe to call per request.
func AnnotateCaresForDisplay(cares []Care, now time.Time) {
	day := func(t time.Time) string { return t.Format("2006-01-02") }

	for i := range cares {
		c := &cares[i]
		c.IsToday = day(c.Date) == day(now)
		c.DayGroupStart = true
		c.DayGroupEnd = true
		if i > 0 {
			c.DayGroupStart = day(cares[i-1].Date) != day(c.Date)
		}
		if i < len(cares)-1 {
			c.DayGroupEnd = day(cares[i+1].Date) != day(c.Date)
		}
	}

	// Weight trend: the chronologically previous care is the NEXT entry in
	// the date-descending list; compare against the nearest earlier weighted
	// care and skip unparseable/missing weights.
	for i := range cares {
		w, ok := ParseCareWeight(cares[i].Weight)
		if !ok {
			continue
		}
		for j := i + 1; j < len(cares); j++ {
			p, okp := ParseCareWeight(cares[j].Weight)
			if !okp {
				continue
			}
			if w < p {
				cares[i].WeightTrend = "down"
			} else if w > p {
				cares[i].WeightTrend = "up"
			}
			break
		}
	}
}

// ParseCareWeight parses a care weight string (grams, dot or comma decimal
// separator). Returns ok=false for empty or non-numeric values.
func ParseCareWeight(w nulls.String) (float64, bool) {
	if !w.Valid {
		return 0, false
	}
	s := strings.TrimSpace(strings.Replace(w.String, ",", ".", 1))
	if s == "" {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return f, true
}

// WeightTrendClass returns the Bootstrap text color class for the care
// weight cell: red when the weight dropped versus the previous weighted
// care, green when it rose, muted otherwise (issue #158).
func (c Care) WeightTrendClass() string {
	switch c.WeightTrend {
	case "down":
		return "text-danger font-weight-bold"
	case "up":
		return "text-success font-weight-bold"
	}
	return "text-muted"
}

// String is not required by pop and may be deleted
func (c Cares) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (c *Care) Validate(tx *pop.Connection) (*validate.Errors, error) {
	utils.TrimStringFields(c)
	return validate.Validate(
		&validators.TimeIsPresent{Field: c.Date, Name: "Date"},
	), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (c *Care) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
// This method is not required and may be deleted.
func (c *Care) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}
