package careplan

import "time"

// Override resolution (§4.7, §6.1 step 2, §10-A3): within one animal and
// one action_kind, an active animal-plan occurrence suppresses a rule
// occurrence either kind-level (replaces_kind=true: ALL rule occurrences of
// the kind) or slot-level (default: |dueΔ| ≤ the RULE's grace). Different
// kinds never interact; plans never suppress plans.

// ResolveOverrides marks rule occurrences suppressed by an animal plan of
// the same kind with Status=StatusOverridden and OverriddenBy set to the
// plan's name (the why-explanation of §4.7). All other items pass through
// with Status unset — BuildPlanItems computes it. When several plans
// collide with the same rule occurrence, the EARLIEST plan occurrence names
// the override (deterministic; ties by source id).
func ResolveOverrides(occs []Occurrence) []PlanItem {
	items := make([]PlanItem, len(occs))
	for i, o := range occs {
		items[i] = PlanItem{Occurrence: o}
	}

	type groupKey struct {
		animal int
		kind   string
	}
	groups := make(map[groupKey][]int, len(items))
	for i := range items {
		src := items[i].Occurrence.Source
		if src == nil {
			continue
		}
		k := groupKey{items[i].Occurrence.AnimalID, src.ActionKind()}
		groups[k] = append(groups[k], i)
	}

	for _, idxs := range groups {
		var plans, rules []int
		for _, i := range idxs {
			if items[i].Occurrence.Source.SourceType() == SourceAnimal {
				plans = append(plans, i)
			} else {
				rules = append(rules, i)
			}
		}
		if len(plans) == 0 || len(rules) == 0 {
			continue
		}
		for _, r := range rules {
			ruleSrc := items[r].Occurrence.Source
			grace := time.Duration(ruleSrc.Schedule().GraceMinutes) * time.Minute
			var best *Occurrence
			for _, p := range plans {
				po := items[p].Occurrence
				suppresses := po.Source.ReplacesKind() ||
					absDur(po.DueAt.Sub(items[r].Occurrence.DueAt)) <= grace
				if !suppresses {
					continue
				}
				if best == nil || po.DueAt.Before(best.DueAt) ||
					(po.DueAt.Equal(best.DueAt) && po.Source.SourceID() < best.Source.SourceID()) {
					candidate := po
					best = &candidate
				}
			}
			if best != nil {
				items[r].Status = StatusOverridden
				items[r].OverriddenBy = best.Source.Name()
			}
		}
	}
	return items
}

func absDur(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}
