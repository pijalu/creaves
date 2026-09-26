// Package careplan implements the Care Expert System engine (spec:
// creaves/docs/care-expert.md). It is deliberately free of Pop/MySQL
// dependencies (DIP, §9): the matcher field registry resolves values from the
// AnimalContext abstraction, the DSL parser/evaluator work on the AST, the
// occurrence generator works on PlanSource values, and fulfillment writers are
// interfaces satisfied by the real DB writers and by test fakes.
package careplan

import (
	"time"
)

// AnimalContext is the enriched view of one animal the matcher evaluates
// against (§5.1, §9 DIP). The service layer assembles it in bulk via the
// existing EnrichAnimalsOptimized pass; the engine never queries the DB.
//
// Missing values are represented by the empty string / nil pointer and make
// predicates fail closed (§5.2: missing value → predicate false + trace
// reason "no value").
type AnimalContext struct {
	ID int

	// animals columns
	Species string
	Gender  string
	Zone    string
	Cage    string // "" = no cage (virtual « sans cage » bucket)
	Feeding string // current diet free text

	ForceFeed bool

	// resolved reference names (canonical French base values)
	AnimalType string // animaltypes.name
	AnimalAge  string // animalages.name (bébé/juvénile/adulte)

	// species resolved by name (species table)
	SpeciesClass        string
	SpeciesOrder        string
	SpeciesFamily       string
	SpeciesAGWGroup     string
	SpeciesSubsideGroup string
	SpeciesNativeStatus string
	SpeciesGame         bool
	SpeciesHuntable     bool

	// intake condition
	HasParasites bool
	Parasites    string
	HasWounds    bool
	Wounds       string
	IntakeGeneral string
	IntakeRemarks string

	// latest veterinaryvisit.diagnostic ("" when none)
	VetDiagnostic string

	// LastWeightG is the last recorded weight in grams; nil = no weight on
	// record (weight predicates fail closed with reason "no value", §10-B4).
	LastWeightG *float64

	// IntakeDate is the intake instant (planning anchor, §10-B3).
	IntakeDate time.Time

	// OuttakeDate is the outtake instant or nil while in care. Occurrences
	// are clamped to [IntakeDate, OuttakeDate).
	OuttakeDate *time.Time

	// EvalTime is the reference instant for now-dependent fields
	// (days_in_care). Set once per evaluation pass by the service layer.
	EvalTime time.Time
}

// DaysInCare returns whole days since intake at EvalTime (never negative).
func (a *AnimalContext) DaysInCare() int {
	if a.IntakeDate.IsZero() {
		return 0
	}
	d := int(a.EvalTime.Sub(a.IntakeDate).Hours() / 24)
	if d < 0 {
		d = 0
	}
	return d
}

// InCare reports whether the animal is currently in care (no outtake or
// outtake in the future relative to EvalTime).
func (a *AnimalContext) InCare() bool {
	return a.OuttakeDate == nil || a.OuttakeDate.After(a.EvalTime)
}
