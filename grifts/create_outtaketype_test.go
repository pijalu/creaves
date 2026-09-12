package grifts

import "testing"

func TestCanonicalOuttakeTypes(t *testing.T) {
	if len(canonicalOuttakeTypes) != 7 {
		t.Fatalf("got %d canonical types, want 7", len(canonicalOuttakeTypes))
	}
	want := []struct {
		code, name     string
		def, dead, err bool
		rating         int
	}{
		{"OT1", "Relacher", false, false, false, 1},
		{"OT2", "DCD", true, true, false, -1},
		{"OT3", "Euthanasier", false, true, false, -1},
		{"OT4", "Transferer", false, false, false, 1},
		{"OT5", "Mort à l'arrivée avant l'encodage", false, true, false, -1},
		{"OT6", "Adoption", false, false, false, 0},
		{"OT7", "Doublon", false, false, true, -1},
	}
	for i, got := range canonicalOuttakeTypes {
		if got.code != want[i].code || got.name != want[i].name || got.def != want[i].def || got.dead != want[i].dead || got.err != want[i].err || got.rating != want[i].rating {
			t.Errorf("%s fields mismatch", got.code)
		}
	}
}
