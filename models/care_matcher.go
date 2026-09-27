package models

import (
	"encoding/json"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// CareMatcher is a named, reusable matcher query (§4.4): a boolean DSL
// expression (§5.2) selecting animals. Referenced by CareRule.MatcherID;
// many rules may share one matcher (library reuse model, §4.6).
type CareMatcher struct {
	ID          uuid.UUID `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description string    `json:"description" db:"description"`
	// Expression is the matcher DSL (§5.2). Parsed and semantically
	// validated at save time — a matcher that fails to parse must never
	// reach the database.
	Expression string    `json:"expression" db:"expression"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
}

// CareMatchers is not required by pop and may be deleted
type CareMatchers []CareMatcher

// String is not required by pop and may be deleted
func (m CareMatcher) String() string {
	js, _ := json.Marshal(m)
	return string(js)
}

// Validate gets run every time you call a "pop.Validate*" method.
func (m *CareMatcher) Validate(tx *pop.Connection) (*validate.Errors, error) {
	verrs := validate.NewErrors()
	verrs.Append(validate.Validate(
		&validators.StringIsPresent{Field: m.Name, Name: "Name"},
		&validators.StringLengthInRange{
			Field: m.Name, Name: "Name", Min: 1, Max: 200,
		},
		&validators.StringIsPresent{Field: m.Expression, Name: "Expression"},
	))
	if m.Expression != "" {
		if _, err := careplan.ParseValidatedWith(m.Expression, careplan.DefaultRegistry()); err != nil {
			verrs.Add("expression", err.Error())
		}
	}
	return verrs, nil
}
