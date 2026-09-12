package grifts

import "testing"

func TestCanonicalOuttakeTypes(t *testing.T) {
	if len(canonicalOuttakeTypes) != 7 {
		t.Fatalf("got %d canonical types, want 7", len(canonicalOuttakeTypes))
	}
	want := []struct {
		name           string
		def, dead, err bool
		rating         int
	}{
		{"OT1", true, true, false, -1}, {"OT2", false, false, false, 1},
		{"OT3", false, false, false, 1}, {"OT4", false, true, false, -1},
		{"OT5", false, false, true, -1}, {"OT6", false, false, true, -1},
		{"OT7", false, false, false, 0},
	}
	for i, got := range canonicalOuttakeTypes {
		if got.name != want[i].name || got.def != want[i].def || got.dead != want[i].dead || got.err != want[i].err || got.rating != want[i].rating {
			t.Errorf("%s fields mismatch", got.name)
		}
	}
}
