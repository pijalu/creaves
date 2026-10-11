package models

import (
	"encoding/json"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
)

// CareMatcher is a named, reusable matcher query (§4.4): a boolean DSL
// expression (§5.2) selecting animals. Referenced by CareRule.MatcherID;
// many rules may share one matcher (library reuse model, §4.6).
type CareMatcher struct {
	ID   uuid.UUID `json:"id" db:"id"`
	Name string    `json:"name" db:"name"`
	// Nullable column: a plain string here made every scan fail on a NULL
	// row and 500 the whole day plan (see CareRule.Description, TEST-4).
	Description nulls.String `json:"description" db:"description"`
	// Expression is the matcher DSL (§5.2). Parsed and semantically
	// validated at save time — a matcher that fails to parse must never
	// reach the database.
	Expression string `json:"expression" db:"expression"`
	// Derived marks a seed-internal composite matcher (item 9, 2026-10-07):
	// the care_rules schema references ONE matcher, so composite seed rules
	// (SR1/SR4/SR6/SR12) inline their condition into a dedicated matcher.
	// 2026-10-28 ruling (BUG-27): pure seed provenance, internal bookkeeping
	// — NOT user-facing. Editing ANY matcher re-evaluates every referencing
	// rule, so the library shows the coupling for every matcher via the
	// used-by count + rule list, not via a derived-only badge.
	Derived bool `json:"derived" db:"derived"`
	// UsedBy is the number of care_rules referencing this matcher. Not a
	// db column — filled by the library list handler (care_matchers.go).
	UsedBy int `json:"used_by" db:"-"`
	// UsedByRules lists the referencing rules (id, name, kind, active) so
	// the library row can offer a clickable usage dropdown with show/edit
	// links (BUG-27). Not a db column — filled with UsedBy.
	UsedByRules []CareRuleRef `json:"used_by_rules" db:"-"`
	CreatedAt   time.Time     `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at" db:"updated_at"`
}

// CareRuleRef is the lightweight rule projection used by the matcher
// library's used-by dropdown (BUG-27) — just enough to name the rule
// and link to its show/edit pages.
type CareRuleRef struct {
	ID         uuid.UUID `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	ActionKind string    `json:"action_kind" db:"action_kind"`
	Active     bool      `json:"active" db:"active"`
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
