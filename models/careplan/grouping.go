package careplan

// Per-kind day-plan grouping strategies (§6.2a, §9 OCP): cleanup is a
// cage-sized chore and renders one cage card per (cleanup source × cage);
// feeding is a cage-sized diet and renders one card per (cage × normalized
// food text) — the animals of a cage eat the same ration (bugs.md U1);
// every other kind renders per-animal cards with the cage as its batch
// ceiling. A future kind declares its grouping here instead of the
// day-plan handler growing conditionals.

// Grouping is a day-plan card granularity strategy (§6.2a).
type Grouping string

const (
	// GroupingAnimal renders one card per animal occurrence (default).
	GroupingAnimal Grouping = "animal"
	// GroupingCage renders one card per (source × cage) (§6.2a cleanup).
	GroupingCage Grouping = "cage"
	// GroupingCageDiet renders one card per (cage × normalized food text)
	// (bugs.md U1 feeding) — possibly spanning several feeding sources.
	GroupingCageDiet Grouping = "cage_diet"
)

// GroupingByKind is the append-only kind→grouping registry (§9 OCP).
var GroupingByKind = map[string]Grouping{
	KindCleanup: GroupingCage,
	KindFeeding: GroupingCageDiet,
}

// GroupingFor returns the grouping strategy of an action kind; unknown
// kinds default to per-animal cards (§6.2a).
func GroupingFor(actionKind string) Grouping {
	if g, ok := GroupingByKind[actionKind]; ok {
		return g
	}
	return GroupingAnimal
}
