package careplan

import (
	"encoding/json"
	"time"
)

// PlanSource abstracts one planning source — a care_rule (§4.1) or a
// care_animal_plan (§4.7) — as a pure value view (§9 DIP): the occurrence
// generator, the override resolver and the day-plan assembly work on this
// interface, never on Pop models, so the engine stays testable without a
// database. Rules and animal plans share full action/schedule parity (§4.7).
type PlanSource interface {
	// SourceType discriminates the planning level (§4.5 source_type).
	SourceType() SourceType
	// SourceID is the care_rules / care_animal_plans uuid (§4.5 source_id).
	SourceID() string
	// Name is the display name ("Hérisson bébé — gavage"), used by override
	// explanations ("overridden by animal plan …", §4.7) and snapshots.
	Name() string
	// ActionKind is one of the six validated kinds (§4.2).
	ActionKind() string
	// Payload is the §4.2 action_payload JSON (validated per kind by
	// ValidateActionPayload — schema parity rules and animal plans, §4.7).
	Payload() []byte
	// Schedule is the validated §4.3 schedule value object.
	Schedule() Schedule
	// ValidFrom/ValidTo are the rule-level activation window (§4.1), day
	// granularity, nil = unbounded. Animal plans return nil/nil.
	ValidFrom() *time.Time
	ValidTo() *time.Time
	// Priority orders the day plan (§4.1: lower first; default 100).
	Priority() int
	// Active is the soft switch (§4.1/§4.7). Inactive sources generate
	// nothing — deactivating an animal plan re-opens the overridden rule
	// occurrences automatically (§4.7, §5.4).
	Active() bool
	// ReplacesKind is the §10-A3 kind-level override flag: when true, an
	// animal plan suppresses ALL generic-rule occurrences of the same
	// action_kind for its animal (§4.7). Rules always return false.
	ReplacesKind() bool
}

// SourceType enumerates the two planning levels (§4.5 source_type).
type SourceType string

const (
	// SourceRule is a care_rules row (admin-authored, matcher-selected
	// animals, §4.1).
	SourceRule SourceType = "rule"
	// SourceAnimal is a care_animal_plans row (caretaker-authored, one
	// animal, no matcher, §4.7).
	SourceAnimal SourceType = "animal"
)

// Source is the plain-value PlanSource: unit tests build it directly and the
// models layer copies its Pop rows into it. The zero value is inactive.
type Source struct {
	Type        SourceType
	ID          string
	NameStr     string
	Kind        string
	PayloadJSON []byte
	Sched       Schedule
	ValidFromD  *time.Time
	ValidToD    *time.Time
	PriorityVal int
	IsActive    bool
	Replaces    bool // §4.7 replaces_kind (animal plans)
}

// NewSource builds an active rule-typed Source — the common test/model shape.
func NewSource(typ SourceType, id, name, kind string, sched Schedule) *Source {
	return &Source{Type: typ, ID: id, NameStr: name, Kind: kind, Sched: sched, IsActive: true}
}

func (s *Source) SourceType() SourceType { return s.Type }
func (s *Source) SourceID() string       { return s.ID }
func (s *Source) Name() string           { return s.NameStr }
func (s *Source) ActionKind() string     { return s.Kind }
func (s *Source) Payload() []byte        { return s.PayloadJSON }
func (s *Source) Schedule() Schedule     { return s.Sched }
func (s *Source) ValidFrom() *time.Time  { return s.ValidFromD }
func (s *Source) ValidTo() *time.Time    { return s.ValidToD }
func (s *Source) Priority() int          { return s.PriorityVal }
func (s *Source) Active() bool           { return s.IsActive }
func (s *Source) ReplacesKind() bool     { return s.Replaces }

// SnapshotJSON renders the §4.5 source_snapshot audit document (name +
// payload captured at application time, immune to later edits).
func (s *Source) SnapshotJSON() []byte {
	b, _ := json.Marshal(struct {
		Name    string          `json:"name"`
		Kind    string          `json:"action_kind"`
		Payload json.RawMessage `json:"action_payload,omitempty"`
	}{Name: s.NameStr, Kind: s.Kind, Payload: s.PayloadJSON})
	return b
}
