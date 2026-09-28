package careplan

import "testing"

// §6.2a: cleanup is cage-sized; feeding is cage×diet-sized (bugs.md U1);
// every other kind is per-animal.
func TestGroupingFor(t *testing.T) {
	if g := GroupingFor(KindCleanup); g != GroupingCage {
		t.Errorf("cleanup grouping = %q, want %q (§6.2a)", g, GroupingCage)
	}
	if g := GroupingFor(KindFeeding); g != GroupingCageDiet {
		t.Errorf("feeding grouping = %q, want %q (bugs.md U1)", g, GroupingCageDiet)
	}
	for _, k := range []string{KindMedication, KindCare, KindWeighing, KindObservation} {
		if g := GroupingFor(k); g != GroupingAnimal {
			t.Errorf("%s grouping = %q, want %q", k, g, GroupingAnimal)
		}
	}
	// Unknown kinds default to per-animal (safe fallback, §6.2a).
	if g := GroupingFor("massage"); g != GroupingAnimal {
		t.Errorf("unknown kind grouping = %q, want %q", g, GroupingAnimal)
	}
	if g := GroupingFor(""); g != GroupingAnimal {
		t.Errorf("empty kind grouping = %q, want %q", g, GroupingAnimal)
	}
}

// §9 OCP: a future kind picks its grouping via the registry without the
// day-plan handler changing.
func TestGroupingRegistryExtension(t *testing.T) {
	restore := GroupingByKind
	defer func() { GroupingByKind = restore }()
	GroupingByKind["grooming"] = GroupingCage
	if g := GroupingFor("grooming"); g != GroupingCage {
		t.Errorf("registered kind grouping = %q, want %q", g, GroupingCage)
	}
}
