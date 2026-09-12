package actions

import "testing"

func TestReferenceDeleteSpecsCoverAllDependentReferences(t *testing.T) {
	want := map[string]map[string]string{
		"animalages":   {"animals": "animalage_id"},
		"animaltypes":  {"animals": "animaltype_id", "dosages": "animaltype_id", "species": "animaltype_id"},
		"caretypes":    {"cares": "type_id"},
		"discoverers":  {"discoveries": "discoverer_id"},
		"drugs":        {"dosages": "drug_id"},
		"outtaketypes": {"outtakes": "outtaketype_id"},
		"traveltypes":  {"travels": "traveltype_id"},
	}
	if len(referenceDeleteSpecs) != len(want) {
		t.Fatalf("got %d specs, want %d", len(referenceDeleteSpecs), len(want))
	}
	for _, spec := range referenceDeleteSpecs {
		deps, ok := want[spec.Table]
		if !ok {
			t.Fatalf("unexpected spec %q", spec.Table)
		}
		if len(spec.Dependents) != len(deps) {
			t.Errorf("%s: got %d dependents, want %d", spec.Table, len(spec.Dependents), len(deps))
		}
		for table, column := range deps {
			if spec.Dependents[table] != column {
				t.Errorf("%s: %s mapping %q, want %q", spec.Table, table, spec.Dependents[table], column)
			}
		}
	}
}
