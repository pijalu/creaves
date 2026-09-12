package grifts

import "testing"

func TestCanonicalOuttakeTypes(t *testing.T) {
	if len(canonicalOuttakeTypes) != 7 {
		t.Fatalf("got %d canonical types, want 7", len(canonicalOuttakeTypes))
	}
	want := []struct {
		name, description string
		def, dead, err    bool
		rating            int
	}{
		{"OT1", "Animal died", true, true, false, -1},
		{"OT2", "Released to the wild", false, false, false, 1},
		{"OT3", "Transferred to another centre", false, false, false, 1},
		{"OT4", "Euthanized", false, true, false, -1},
		{"OT5", "Lost", false, false, true, -1},
		{"OT6", "Stolen", false, false, true, -1},
		{"OT7", "Other outcome", false, false, false, 0},
	}
	for i, got := range canonicalOuttakeTypes {
		if got.name != want[i].name || got.description != want[i].description || got.def != want[i].def || got.dead != want[i].dead || got.err != want[i].err || got.rating != want[i].rating {
			t.Errorf("%s fields mismatch", got.name)
		}
	}
}
