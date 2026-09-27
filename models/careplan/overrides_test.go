package careplan

import (
	"testing"
)

func planSource(id, kind string, times []TimeOfDay, replaces bool) *Source {
	s := testRule(id, kind, 60, 24, 60)
	s.Type, s.Replaces = SourceAnimal, replaces
	s.Sched.Times = times
	return s
}

func ruleOccs(t *testing.T, src PlanSource, animalID int, dues ...string) []Occurrence {
	t.Helper()
	out := make([]Occurrence, len(dues))
	for i, d := range dues {
		out[i] = occAt(src, animalID, statusInstant(d))
	}
	return out
}

// §10-A3 kind-level: replaces_kind=true suppresses ALL rule occurrences of
// the kind, whatever the slot.
func TestOverrideReplacesKind(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60)
	plan := planSource("ap1", KindFeeding, []TimeOfDay{{12, 0}}, true)

	occs := ruleOccs(t, rule, 42,
		"2026-09-15 08:00:00", "2026-09-15 12:30:00", "2026-09-15 19:00:00")
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 12:00:00")))

	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 09:00:00"), statusInstant("2026-09-15 23:59:00"))
	overridden := map[string]string{}
	for _, it := range items {
		if it.Status == StatusOverridden {
			overridden[it.Occurrence.DueAt.Format("15:04")] = it.OverriddenBy
		}
		if it.Occurrence.Source == plan && it.Status == StatusOverridden {
			t.Errorf("plan occurrence must never be overridden")
		}
	}
	if len(overridden) != 3 {
		t.Fatalf("all 3 rule slots must be suppressed, got %v", overridden)
	}
	for _, due := range []string{"08:00", "12:30", "19:00"} {
		if overridden[due] != "Règle ap1" {
			t.Errorf("%s overridden by %q, want the plan name", due, overridden[due])
		}
	}
}

// §10-A3 slot-level (default): only same-slot collisions (± rule grace)
// are suppressed; distant slots stay actionable.
func TestOverrideSlotLevel(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60) // grace 60 min
	plan := planSource("ap1", KindFeeding, []TimeOfDay{{8, 30}, {15, 0}}, false)

	occs := ruleOccs(t, rule, 42, "2026-09-15 08:00:00", "2026-09-15 19:00:00")
	occs = append(occs,
		occAt(plan, 42, statusInstant("2026-09-15 08:30:00")),
		occAt(plan, 42, statusInstant("2026-09-15 15:00:00")))

	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 09:00:00"), statusInstant("2026-09-15 23:59:00"))
	for _, it := range items {
		due := it.Occurrence.DueAt.Format("15:04")
		switch due {
		case "08:00": // |08:30−08:00| = 30 ≤ grace 60 → suppressed
			if it.Status != StatusOverridden {
				t.Errorf("08:00 within grace must be overridden, got %s", it.Status)
			}
		case "19:00": // no plan occurrence within grace → kept
			if it.Status == StatusOverridden {
				t.Errorf("19:00 has no colliding plan occurrence, must stay actionable")
			}
		}
	}
}

// Slot collision boundary: |Δ| = grace suppresses, grace+1min does not.
func TestOverrideSlotLevelBoundary(t *testing.T) {
	rule := testRule("r1", KindFeeding, 30, 24, 60) // grace 30 min
	plan := planSource("ap1", KindFeeding, []TimeOfDay{{8, 30}}, false)

	occs := ruleOccs(t, rule, 42, "2026-09-15 08:00:00")
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 08:30:00")))
	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 09:00:00"), statusInstant("2026-09-15 23:59:00"))
	if items[0].Status != StatusOverridden {
		t.Errorf("|Δ| = grace must suppress, got %s", items[0].Status)
	}

	occs = ruleOccs(t, rule, 42, "2026-09-15 08:00:00")
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 08:31:00")))
	items = BuildPlanItems(occs, nil, statusInstant("2026-09-15 09:00:00"), statusInstant("2026-09-15 23:59:00"))
	if items[0].Status == StatusOverridden {
		t.Errorf("|Δ| = grace+1min must NOT suppress")
	}
}

// Different kinds never interact (§4.7): an animal medication plan does not
// supersede a generic feeding rule even at the same instant.
func TestOverrideDifferentKindKept(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60)
	plan := planSource("ap1", KindMedication, []TimeOfDay{{8, 0}}, true) // exact slot + kind-level

	occs := ruleOccs(t, rule, 42, "2026-09-15 08:00:00")
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 08:00:00")))
	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 07:00:00"), statusInstant("2026-09-15 23:59:00"))
	for _, it := range items {
		if it.Status == StatusOverridden {
			t.Errorf("cross-kind override must never happen, %s at %s",
				it.Occurrence.Source.ActionKind(), it.Occurrence.DueAt)
		}
	}
}

// Overrides are per animal: a plan for animal 42 never touches animal 43.
func TestOverridePerAnimal(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60)
	plan := planSource("ap1", KindFeeding, []TimeOfDay{{8, 0}}, true)

	occs := append(ruleOccs(t, rule, 42, "2026-09-15 08:00:00"),
		ruleOccs(t, rule, 43, "2026-09-15 08:00:00")...)
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 08:00:00")))

	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 07:00:00"), statusInstant("2026-09-15 23:59:00"))
	for _, it := range items {
		if it.Occurrence.AnimalID == 43 && it.Status == StatusOverridden {
			t.Errorf("animal 43 must be untouched by animal 42's plan")
		}
		if it.Occurrence.AnimalID == 42 && it.Occurrence.Source == rule && it.Status != StatusOverridden {
			t.Errorf("animal 42's rule occurrence must be overridden, got %s", it.Status)
		}
	}
}

// Two animal plans of the same kind coexist (both explicitly authored) and
// never override each other; the suppression name is deterministic
// (earliest plan occurrence wins).
func TestOverridePlansCoexistDeterministic(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60)
	p1 := planSource("ap1", KindFeeding, []TimeOfDay{{8, 10}}, true)
	p2 := planSource("ap2", KindFeeding, []TimeOfDay{{8, 20}}, true)

	occs := ruleOccs(t, rule, 42, "2026-09-15 08:00:00")
	occs = append(occs,
		occAt(p1, 42, statusInstant("2026-09-15 08:10:00")),
		occAt(p2, 42, statusInstant("2026-09-15 08:20:00")))

	items := BuildPlanItems(occs, nil, statusInstant("2026-09-15 07:00:00"), statusInstant("2026-09-15 23:59:00"))
	plansKept := 0
	for _, it := range items {
		if it.Occurrence.Source == rule {
			if it.Status != StatusOverridden || it.OverriddenBy != "Règle ap1" {
				t.Errorf("rule = %+v, want overridden by the EARLIEST plan (ap1)", it)
			}
		} else if it.Status == StatusOverridden {
			t.Errorf("plan %s overridden — plans coexist", it.Occurrence.Source.SourceID())
		} else {
			plansKept++
		}
	}
	if plansKept != 2 {
		t.Errorf("both plans must remain, kept %d", plansKept)
	}
}

// Historical application beats suppression: an occurrence applied before
// the animal plan was created stays "applied" (§6.1 order), with the
// override kept as an explanation.
func TestOverrideWithApplicationKeepsApplied(t *testing.T) {
	rule := testRule("r1", KindFeeding, 60, 24, 60)
	plan := planSource("ap1", KindFeeding, []TimeOfDay{{12, 0}}, true)

	due := statusInstant("2026-09-15 08:00:00")
	occs := ruleOccs(t, rule, 42, "2026-09-15 08:00:00")
	occs = append(occs, occAt(plan, 42, statusInstant("2026-09-15 12:00:00")))

	appKey := OccurrenceKey(occAt(rule, 42, due))
	items := BuildPlanItems(occs, map[string]*ApplicationView{
		appKey: {Status: "applied"},
	}, statusInstant("2026-09-15 09:00:00"), statusInstant("2026-09-15 23:59:00"))
	for _, it := range items {
		if it.Occurrence.Source == rule {
			if it.Status != StatusApplied || it.OverriddenBy == "" {
				t.Errorf("applied rule occurrence = %+v, want applied + override note", it)
			}
		}
	}
}
