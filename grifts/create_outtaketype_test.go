package grifts

import (
	"testing"

	"github.com/gobuffalo/nulls"
)

func TestCanonicalOuttakeTypes(t *testing.T) {
	if len(canonicalOuttakeTypes) != 7 {
		t.Fatalf("got %d canonical types, want 7", len(canonicalOuttakeTypes))
	}
	want := []struct {
		code, name     string
		def, dead, err bool
		rating         nulls.Int
	}{
		{"OT1", "Relacher", false, false, false, nulls.NewInt(1)},
		{"OT2", "DCD", true, true, false, nulls.NewInt(-1)},
		{"OT3", "Euthanasier", false, true, false, nulls.NewInt(-1)},
		{"OT4", "Transferer", false, false, false, nulls.NewInt(1)},
		{"OT5", "Mort à l'arrivée avant l'encodage", false, true, false, nulls.NewInt(-1)},
		{"OT6", "Adoption", false, false, false, nulls.NewInt(0)},
		// OT7 carries no rating (#199-11): NULL, not a numeric outcome.
		{"OT7", "Doublon", false, false, true, nulls.Int{}},
	}
	for i, got := range canonicalOuttakeTypes {
		if got.code != want[i].code || got.name != want[i].name || got.def != want[i].def || got.dead != want[i].dead || got.err != want[i].err || got.rating.Interface() != want[i].rating.Interface() {
			t.Errorf("%s fields mismatch", got.code)
		}
	}
}
