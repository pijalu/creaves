package careplan

import (
	"testing"
	"time"
)

// testContext builds a representative enriched animal context (§5.1 registry).
func testContext() *AnimalContext {
	intake := time.Date(2026, 9, 15, 11, 0, 0, 0, time.Local)
	weight := 240.0
	return &AnimalContext{
		ID:                  10472,
		Species:             "Hérisson d'Europe",
		SpeciesClass:        "Mammalia",
		SpeciesOrder:        "Eulipotyphla",
		SpeciesFamily:       "Erinaceidae",
		SpeciesAGWGroup:     "Hérissons",
		SpeciesSubsideGroup: "A1",
		SpeciesNativeStatus: "Indigène",
		AnimalType:          "Hérissons / Insectivore",
		AnimalAge:           "bébé",
		Gender:              "F",
		Zone:                "Salle 1",
		Cage:                "A12",
		LastWeightG:         &weight,
		HasParasites:        true,
		Parasites:           "puces ++++ - tiques",
		IntakeGeneral:       "bon état général",
		IntakeRemarks:       "déshydraté",
		Feeding:             "Croquettes + 4 VDF",
		ForceFeed:           true,
		VetDiagnostic:       "gale suspectée",
		IntakeDate:          intake,
		EvalTime:            time.Date(2026, 9, 20, 9, 0, 0, 0, time.Local),
	}
}

func TestRegistryContainsInitialEntries(t *testing.T) {
	// §5.1 initial registry: every documented field key must be registered.
	want := []string{
		"species", "species_class", "species_order", "species_family",
		"species_agw_group", "species_subside_group", "species_native_status",
		"species_game", "species_huntable",
		"animal_type", "animal_age", "gender", "zone", "cage",
		"weight_g", "days_in_care",
		"has_parasites", "has_wounds", "parasites", "wounds",
		"intake_general", "intake_remarks", "feeding", "force_feed",
		"vet_diagnostic",
	}
	for _, key := range want {
		p, ok := DefaultRegistry().Get(key)
		if !ok {
			t.Errorf("field %q missing from default registry", key)
			continue
		}
		if p.Key != key {
			t.Errorf("registry key mismatch: want %q got %q", key, p.Key)
		}
		if p.LabelKey == "" {
			t.Errorf("field %q has empty LabelKey", key)
		}
		if len(p.Ops) == 0 {
			t.Errorf("field %q declares no ops", key)
		}
	}
}

func TestRegistryTypesAndOps(t *testing.T) {
	r := DefaultRegistry()

	cases := []struct {
		key     string
		typ     string
		wantOps []string
		notOps  []string
	}{
		{"species", TypeString, []string{OpEq, OpNeq, OpIn, OpRegex, OpContains}, []string{OpLt}},
		{"animal_type", TypeString, []string{OpEq, OpIn}, []string{OpRegex, OpLt}},
		{"weight_g", TypeNumber, []string{OpLt, OpLte, OpGt, OpGte, OpBetween}, []string{OpEq, OpRegex, OpIn}},
		{"days_in_care", TypeNumber, []string{OpLt, OpGte, OpBetween}, []string{OpRegex, OpEq}},
		{"has_parasites", TypeBool, []string{OpEq}, []string{OpLt, OpRegex, OpIn}},
		{"force_feed", TypeBool, []string{OpEq}, []string{OpIn, OpContains, OpNeq}},
		{"parasites", TypeString, []string{OpRegex, OpContains}, []string{OpLt, OpEq}},
		{"cage", TypeString, []string{OpEq, OpIn, OpRegex}, []string{OpLt}},
		{"feeding", TypeString, []string{OpRegex, OpContains}, []string{OpEq}},
	}
	for _, c := range cases {
		p, ok := r.Get(c.key)
		if !ok {
			t.Fatalf("field %q not registered", c.key)
		}
		if p.Type != c.typ {
			t.Errorf("%s: type = %q, want %q", c.key, p.Type, c.typ)
		}
		for _, op := range c.wantOps {
			if !p.Allows(op) {
				t.Errorf("%s: op %q should be allowed", c.key, op)
			}
		}
		for _, op := range c.notOps {
			if p.Allows(op) {
				t.Errorf("%s: op %q should NOT be allowed", c.key, op)
			}
		}
	}
}

func TestRegistryResolve(t *testing.T) {
	r := DefaultRegistry()
	ctx := testContext()

	// string resolve
	v := r.MustGet("species").Resolve(ctx)
	if s, ok := v.Value.(string); !ok || s != "Hérisson d'Europe" {
		t.Errorf("species resolve = %v (%T)", v.Value, v.Value)
	}
	// number resolve
	v = r.MustGet("weight_g").Resolve(ctx)
	if f, ok := v.Value.(float64); !ok || f != 240 {
		t.Errorf("weight_g resolve = %v (%T)", v.Value, v.Value)
	}
	// bool resolve
	v = r.MustGet("force_feed").Resolve(ctx)
	if b, ok := v.Value.(bool); !ok || !b {
		t.Errorf("force_feed resolve = %v (%T)", v.Value, v.Value)
	}
}

func TestRegistryResolveMissingValues(t *testing.T) {
	r := DefaultRegistry()
	ctx := testContext()
	ctx.LastWeightG = nil  // no weight on record
	ctx.Wounds = ""        // empty free text
	ctx.VetDiagnostic = "" // no veterinary visit on record

	if v := r.MustGet("weight_g").Resolve(ctx); !v.Missing {
		t.Errorf("weight_g with nil pointer should report Missing")
	}
	if v := r.MustGet("wounds").Resolve(ctx); !v.Missing {
		t.Errorf("wounds with empty text should report Missing (fail-closed)")
	}
	if v := r.MustGet("vet_diagnostic").Resolve(ctx); !v.Missing {
		t.Errorf("vet_diagnostic empty should report Missing")
	}
	// §3 decision 5: animals without a cage value still expose "" so
	// `cage = ""` / the « sans cage » bucket stays expressible.
	if v := r.MustGet("cage").Resolve(ctx); v.Missing {
		t.Errorf("cage empty string is a value, not missing")
	}
}

func TestRegistryDaysInCareReflectsEvalTime(t *testing.T) {
	r := DefaultRegistry()
	ctx := testContext()
	// EvalTime 2026-09-20 09:00, intake 2026-09-15 11:00 → 4 full days (floor of 4d22h).
	days := r.MustGet("days_in_care").Resolve(ctx)
	if f, ok := days.Value.(float64); !ok || f != 4 {
		t.Errorf("days_in_care = %v, want 4 (floor of 4d22h)", days.Value)
	}
}

func TestRegistryIsAppendOnlyAndIndependent(t *testing.T) {
	// OCP: callers may register extra providers without mutating the default.
	r := NewRegistry()
	if err := r.Register(FieldProvider{Key: "parasite_type", LabelKey: "careplan.field.parasite_type", Type: TypeString, Ops: []string{OpEq, OpIn}}); err != nil {
		t.Fatalf("register: %v", err)
	}
	if _, ok := r.Get("parasite_type"); !ok {
		t.Fatal("custom field not registered")
	}
	// default registry untouched
	if _, ok := DefaultRegistry().Get("parasite_type"); ok {
		t.Error("default registry must not leak custom registrations")
	}
}

func TestRegistryRejectsDuplicateAndInvalid(t *testing.T) {
	r := NewRegistry()
	if err := r.Register(FieldProvider{Key: "species", Type: TypeString, Ops: []string{OpEq}}); err == nil {
		t.Error("duplicate key must be rejected")
	}
	if err := r.Register(FieldProvider{Key: "", Type: TypeString, Ops: []string{OpEq}}); err == nil {
		t.Error("empty key must be rejected")
	}
	if err := r.Register(FieldProvider{Key: "x", Type: "blob", Ops: []string{OpEq}}); err == nil {
		t.Error("unknown type must be rejected")
	}
	if err := r.Register(FieldProvider{Key: "y", Type: TypeNumber, Ops: []string{OpRegex}}); err == nil {
		t.Error("number field must not allow regex op")
	}
	// §5.1: bool fields declare eq only (NOT covers negation).
	if err := r.Register(FieldProvider{Key: "z", Type: TypeBool, Ops: []string{OpEq, OpNeq}}); err == nil {
		t.Error("bool fields must not allow neq")
	}
}

// TestRegistryCaseInsensitiveOps (bugs.md U26/U27 R5-1a): every string field
// that allows eq/neq/in also allows its CI twins; fields without eq/neq/in
// (free-text regex fields) and non-string types reject them.
func TestRegistryCaseInsensitiveOps(t *testing.T) {
	r := DefaultRegistry()

	// eq/in-bearing string fields gain the CI eq/in twins.
	for _, key := range []string{"cage", "species", "zone", "animal_type", "gender",
		"species_class", "species_order", "species_family", "species_agw_group",
		"species_subside_group", "species_native_status"} {
		p, ok := r.Get(key)
		if !ok {
			t.Fatalf("field %q not registered", key)
		}
		for _, op := range []string{OpEqCI, OpInCI} {
			if !p.Allows(op) {
				t.Errorf("%s: op %q should be allowed", key, op)
			}
		}
	}
	// CI neq only mirrors plain neq — species is the only default field with !=.
	sp, _ := r.Get("species")
	if !sp.Allows(OpNeqCI) {
		t.Errorf("species: op %q should be allowed", OpNeqCI)
	}
	for _, key := range []string{"cage", "animal_type", "gender", "zone"} {
		p, _ := r.Get(key)
		if p.Allows(OpNeqCI) {
			t.Errorf("%s: op %q should NOT be allowed (no plain !=)", key, OpNeqCI)
		}
	}
	for _, key := range []string{"parasites", "wounds", "feeding", "intake_general",
		"intake_remarks", "vet_diagnostic"} {
		p, _ := r.Get(key)
		for _, op := range []string{OpEqCI, OpNeqCI, OpInCI} {
			if p.Allows(op) {
				t.Errorf("%s: op %q should NOT be allowed (no eq/in)", key, op)
			}
		}
	}
	for _, key := range []string{"weight_g", "days_in_care", "has_parasites", "has_wounds", "force_feed"} {
		p, _ := r.Get(key)
		for _, op := range []string{OpEqCI, OpNeqCI, OpInCI} {
			if p.Allows(op) {
				t.Errorf("%s: op %q should NOT be allowed (non-string field)", key, op)
			}
		}
	}

	// Register validates CI ops like any other op: legal on strings only.
	r2 := NewRegistry()
	if err := r2.Register(FieldProvider{Key: "s", LabelKey: "s", Type: TypeString, Ops: []string{OpEqCI}}); err != nil {
		t.Errorf("string field with =* rejected: %v", err)
	}
	if err := r2.Register(FieldProvider{Key: "n", LabelKey: "n", Type: TypeNumber, Ops: []string{OpEqCI}}); err == nil {
		t.Error("number field with =* must be rejected")
	}
	if err := r2.Register(FieldProvider{Key: "b", LabelKey: "b", Type: TypeBool, Ops: []string{OpInCI}}); err == nil {
		t.Error("bool field with INCI must be rejected")
	}
}
