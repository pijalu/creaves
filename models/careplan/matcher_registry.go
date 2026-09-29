package careplan

import (
	"fmt"
	"sort"
)

// Field value types (§5.1). A provider's Type governs which literal kinds the
// DSL accepts and which operators are legal.
const (
	TypeString = "string"
	TypeNumber = "number"
	TypeBool   = "bool"
)

// DSL operators (§5.2). The values are the exact DSL spellings so validation
// errors can quote them verbatim.
const (
	OpEq       = "="
	OpNeq      = "!="
	OpLt       = "<"
	OpLte      = "<="
	OpGt       = ">"
	OpGte      = ">="
	OpIn       = "IN"
	OpBetween  = "BETWEEN"
	OpRegex    = "~"
	OpNotRegex = "!~"
	OpContains = "CONTAINS"

	// Explicit case-insensitive operators (bugs.md U26/U27, D-g): the CI
	// twin of every =-family and IN-family op. The values are the exact
	// DSL spellings so validation errors quote them verbatim.
	OpEqCI  = "=*"
	OpNeqCI = "!=*"
	OpInCI  = "INCI"
)

// opsByType lists the operators each field type legally supports (§5.1):
// strings match by equality/list/pattern, numbers by comparison/range, and
// bools by equality only (NOT covers negation). The CI variants are legal
// on strings only — case is meaningless for numbers and booleans.
var opsByType = map[string]map[string]bool{
	TypeString: {OpEq: true, OpNeq: true, OpIn: true, OpRegex: true, OpNotRegex: true, OpContains: true,
		OpEqCI: true, OpNeqCI: true, OpInCI: true},
	TypeNumber: {OpLt: true, OpLte: true, OpGt: true, OpGte: true, OpBetween: true},
	TypeBool:   {OpEq: true},
}

// ResolvedValue is the result of resolving one field for one animal.
// Missing=true marks "no value on record" — predicates fail closed (§5.2).
type ResolvedValue struct {
	Value   interface{} // string | float64 | bool; nil when Missing
	Missing bool
}

func strValue(s string) ResolvedValue {
	if s == "" {
		return ResolvedValue{Missing: true}
	}
	return ResolvedValue{Value: s}
}

// strValueKeepEmpty treats the empty string as a real value — the cage is the
// one field where "" is meaningful (the « sans cage » bucket, §3 decision 5).
func strValueKeepEmpty(s string) ResolvedValue { return ResolvedValue{Value: s} }

func numValue(f float64) ResolvedValue { return ResolvedValue{Value: f} }
func boolValue(b bool) ResolvedValue   { return ResolvedValue{Value: b} }

func numValuePtr(f *float64) ResolvedValue {
	if f == nil {
		return ResolvedValue{Missing: true}
	}
	return ResolvedValue{Value: *f}
}

// FieldProvider knows how to resolve one matchable field from the enriched
// animal context, which ops it supports, and how the admin UI labels it (§5.1).
// Registration is append-only Go code: a new structured condition field becomes
// matchable by adding one entry — no change to rule storage or evaluation.
type FieldProvider struct {
	Key      string
	LabelKey string // i18n key for the admin UI
	Type     string // string | number | bool
	Ops      []string
	Resolve  func(a *AnimalContext) ResolvedValue
}

// Allows reports whether op may be used with this field.
func (p FieldProvider) Allows(op string) bool {
	for _, o := range p.Ops {
		if o == op {
			return true
		}
	}
	return false
}

// Registry maps field key → provider. Registries are append-only (OCP): the
// default set never mutates; callers derive extended registries from
// NewRegistry (pre-populated with the default set) via Register.
type Registry struct {
	fields map[string]FieldProvider
}

// NewRegistry returns a fresh registry pre-populated with the default field
// set (§5.1). Extend it with Register; the shared DefaultRegistry is never
// affected.
func NewRegistry() *Registry {
	r := &Registry{fields: make(map[string]FieldProvider, len(defaultFields))}
	for _, p := range defaultFields {
		r.fields[p.Key] = p
	}
	return r
}

// DefaultRegistry returns the shared registry with the §5.1 initial field set.
func DefaultRegistry() *Registry { return defaultRegistry }

var (
	defaultFields   = buildDefaultFields()
	defaultRegistry = NewRegistry()
)

// Register adds a provider. It rejects duplicates, empty keys/labels, unknown
// types and op/type mismatches so bad providers fail at registration time —
// not at plan-render time.
func (r *Registry) Register(p FieldProvider) error {
	if p.Key == "" {
		return fmt.Errorf("careplan: field key must not be empty")
	}
	if _, dup := r.fields[p.Key]; dup {
		return fmt.Errorf("careplan: field %q is already registered", p.Key)
	}
	legal, ok := opsByType[p.Type]
	if !ok {
		return fmt.Errorf("careplan: field %q has unknown type %q", p.Key, p.Type)
	}
	if p.LabelKey == "" {
		return fmt.Errorf("careplan: field %q must declare LabelKey", p.Key)
	}
	if len(p.Ops) == 0 {
		return fmt.Errorf("careplan: field %q must declare at least one op", p.Key)
	}
	for _, op := range p.Ops {
		if !legal[op] {
			return fmt.Errorf("careplan: field %q (%s) must not allow op %q", p.Key, p.Type, op)
		}
	}
	r.fields[p.Key] = p
	return nil
}

// Get looks a provider up by key.
func (r *Registry) Get(key string) (FieldProvider, bool) {
	p, ok := r.fields[key]
	return p, ok
}

// MustGet returns the provider for key or panics — for engine-internal lookups
// of keys known to exist (tests, defaults).
func (r *Registry) MustGet(key string) FieldProvider {
	p, ok := r.fields[key]
	if !ok {
		panic(fmt.Sprintf("careplan: unknown field %q", key))
	}
	return p
}

// Fields returns all providers sorted by key (stable admin-UI dropdown order).
func (r *Registry) Fields() []FieldProvider {
	out := make([]FieldProvider, 0, len(r.fields))
	for _, p := range r.fields {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

// buildDefaultFields returns the §5.1 initial registry: species taxonomy
// (resolved by name), animal type/age, location, condition, intake flags and
// free-text fields for regex matching. Text fields mark the empty string as
// "no value" (fail closed); the cage is the documented exception.
func buildDefaultFields() []FieldProvider {
	eqIn := []string{OpEq, OpIn}
	eqInRegex := []string{OpEq, OpIn, OpRegex}
	numeric := []string{OpLt, OpLte, OpGt, OpGte, OpBetween}
	text := []string{OpRegex, OpNotRegex, OpContains}
	boolEq := []string{OpEq}

	// withCIOps mirrors every =-family / IN-family op a string field
	// already allows into its explicit CI twin (bugs.md U26/U27, D-g
	// "for both"): fields that never allowed eq/neq/in (the free-text
	// regex fields) stay CS-only.
	withCIOps := func(ops []string) []string {
		out := make([]string, 0, len(ops)+3)
		out = append(out, ops...)
		for _, pair := range [][2]string{{OpEq, OpEqCI}, {OpNeq, OpNeqCI}, {OpIn, OpInCI}} {
			for _, op := range ops {
				if op == pair[0] {
					out = append(out, pair[1])
					break
				}
			}
		}
		return out
	}

	strField := func(key string, ops []string, get func(a *AnimalContext) string) FieldProvider {
		return FieldProvider{Key: key, LabelKey: "careplan.field." + key, Type: TypeString, Ops: withCIOps(ops),
			Resolve: func(a *AnimalContext) ResolvedValue { return strValue(get(a)) }}
	}
	numField := func(key string, get func(a *AnimalContext) float64) FieldProvider {
		return FieldProvider{Key: key, LabelKey: "careplan.field." + key, Type: TypeNumber, Ops: numeric,
			Resolve: func(a *AnimalContext) ResolvedValue { return numValue(get(a)) }}
	}
	boolField := func(key string, get func(a *AnimalContext) bool) FieldProvider {
		return FieldProvider{Key: key, LabelKey: "careplan.field." + key, Type: TypeBool, Ops: boolEq,
			Resolve: func(a *AnimalContext) ResolvedValue { return boolValue(get(a)) }}
	}

	return []FieldProvider{
		strField("species", []string{OpEq, OpNeq, OpIn, OpRegex, OpContains}, func(a *AnimalContext) string { return a.Species }),
		strField("species_class", eqInRegex, func(a *AnimalContext) string { return a.SpeciesClass }),
		strField("species_order", eqInRegex, func(a *AnimalContext) string { return a.SpeciesOrder }),
		strField("species_family", eqInRegex, func(a *AnimalContext) string { return a.SpeciesFamily }),
		strField("species_agw_group", eqInRegex, func(a *AnimalContext) string { return a.SpeciesAGWGroup }),
		strField("species_subside_group", eqInRegex, func(a *AnimalContext) string { return a.SpeciesSubsideGroup }),
		strField("species_native_status", eqInRegex, func(a *AnimalContext) string { return a.SpeciesNativeStatus }),
		boolField("species_game", func(a *AnimalContext) bool { return a.SpeciesGame }),
		boolField("species_huntable", func(a *AnimalContext) bool { return a.SpeciesHuntable }),
		strField("animal_type", eqIn, func(a *AnimalContext) string { return a.AnimalType }),
		strField("animal_age", eqIn, func(a *AnimalContext) string { return a.AnimalAge }),
		strField("gender", eqIn, func(a *AnimalContext) string { return a.Gender }),
		strField("zone", eqInRegex, func(a *AnimalContext) string { return a.Zone }),
		{Key: "cage", LabelKey: "careplan.field.cage", Type: TypeString, Ops: withCIOps(eqInRegex),
			Resolve: func(a *AnimalContext) ResolvedValue { return strValueKeepEmpty(a.Cage) }},
		numField("days_in_care", func(a *AnimalContext) float64 { return float64(a.DaysInCare()) }),
		boolField("has_parasites", func(a *AnimalContext) bool { return a.HasParasites }),
		boolField("has_wounds", func(a *AnimalContext) bool { return a.HasWounds }),
		strField("parasites", text, func(a *AnimalContext) string { return a.Parasites }),
		strField("wounds", text, func(a *AnimalContext) string { return a.Wounds }),
		strField("intake_general", text, func(a *AnimalContext) string { return a.IntakeGeneral }),
		strField("intake_remarks", text, func(a *AnimalContext) string { return a.IntakeRemarks }),
		strField("feeding", text, func(a *AnimalContext) string { return a.Feeding }),
		boolField("force_feed", func(a *AnimalContext) bool { return a.ForceFeed }),
		strField("vet_diagnostic", text, func(a *AnimalContext) string { return a.VetDiagnostic }),
		{Key: "weight_g", LabelKey: "careplan.field.weight_g", Type: TypeNumber, Ops: numeric,
			Resolve: func(a *AnimalContext) ResolvedValue { return numValuePtr(a.LastWeightG) }},
	}
}
