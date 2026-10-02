package actions

import (
	"fmt"
	"sort"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Round-4 R4-6 — applicable-protocol traceability.
//
// The Protocol tab used to list only the animal's own care plans, so a
// caregiver could not answer "where does this due time come from?": the
// global care rules that also apply were invisible. The trace is built
// from the SAME engine pass that renders the plan (DayPlan), filtered to
// the sources that actually produced at least one occurrence for THIS
// animal — a rule that produces nothing never appears.

// ProtocolSourceView is one source (animal plan or global rule) that
// applies to the animal, with what it actually contributed.
type ProtocolSourceView struct {
	SourceType  string // "animal" | "rule"
	SourceID    string
	Name        string // display name (conversion markers stripped)
	Kind        string // action kind
	Content     string // humanised action payload (drug, food, prompt…)
	Schedule    string // raw §4.3 schedule JSON — the template humanises it
	EditURL     string // /animals/{id}#nav-plan or /care_rules/{id}
	Editable    bool   // animal plans: always; global rules: admin only
	Replaces    bool   // the source replaces the kind-level protocol
	Occurrences int    // occurrences this source contributed to the window
}

// ProtocolTraceView is the ordered list of applicable sources: animal
// plans first (they win over global rules), then rules by name.
type ProtocolTraceView struct {
	AnimalID int
	Sources  []ProtocolSourceView
}

// protocolTraceOf projects the applicable sources of one animal from the
// engine's day plan. admin gates the global-rule EDIT LINK only — a
// non-admin still sees the rule's name, content and schedule (never a
// dead link).
func protocolTraceOf(tx *pop.Connection, plan *DayPlan, animal *models.Animal, admin bool) (*ProtocolTraceView, error) {
	trace := &ProtocolTraceView{}
	if plan == nil || animal == nil {
		return trace, nil
	}
	trace.AnimalID = animal.ID
	ruleIDs, planIDs := traceSources(plan, animal, admin, trace)
	schedules, err := protocolSchedules(tx, ruleIDs, planIDs)
	if err != nil {
		return nil, err
	}
	for i := range trace.Sources {
		trace.Sources[i].Schedule = schedules[trace.Sources[i].SourceType+"|"+trace.Sources[i].SourceID]
	}
	sortProtocolSources(trace.Sources)
	return trace, nil
}

// traceSources folds every occurrence of THIS animal into one row per
// contributing source, counting what each produced, and returns the ids
// whose stored schedule document must still be loaded.
func traceSources(plan *DayPlan, animal *models.Animal, admin bool, trace *ProtocolTraceView) (ruleIDs, planIDs []uuid.UUID) {
	idx := map[[2]string]int{}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animal.ID {
			continue
		}
		k := [2]string{string(src.SourceType()), src.SourceID()}
		if at, ok := idx[k]; ok {
			trace.Sources[at].Occurrences++
			continue
		}
		idx[k] = len(trace.Sources)
		trace.Sources = append(trace.Sources, protocolSourceView(src, animal, admin))
		if u, err := uuid.FromString(src.SourceID()); err == nil {
			if k[0] == string(careplan.SourceRule) {
				ruleIDs = append(ruleIDs, u)
			} else {
				planIDs = append(planIDs, u)
			}
		}
	}
	return ruleIDs, planIDs
}

// protocolSourceView projects one source into its trace row (schedule
// filled in by the caller from the stored document).
func protocolSourceView(src careplan.PlanSource, animal *models.Animal, admin bool) ProtocolSourceView {
	view := ProtocolSourceView{
		SourceType:  string(src.SourceType()),
		SourceID:    src.SourceID(),
		Name:        DisplayName(src.Name()),
		Kind:        src.ActionKind(),
		Content:     planDetail(src),
		Replaces:    src.ReplacesKind(),
		Occurrences: 1,
	}
	if view.SourceType == string(careplan.SourceAnimal) {
		// The animal's own protocol is edited on this very page.
		view.EditURL = fmt.Sprintf("/animals/%d#nav-plan", animal.ID)
		view.Editable = true
		return view
	}
	view.EditURL = "/care_rules/" + src.SourceID()
	view.Editable = admin // global rules are admin-only (never a dead link)
	return view
}

// protocolSchedules loads the raw §4.3 schedule documents of the traced
// sources, keyed "type|id" — the raw text is what humanSchedule() reads,
// so the trace shows the same sentence the editors show.
func protocolSchedules(tx *pop.Connection, ruleIDs, planIDs []uuid.UUID) (map[string]string, error) {
	out := map[string]string{}
	if len(ruleIDs) > 0 {
		rules := &models.CareRules{}
		if err := tx.Where("id IN (?)", ruleIDs).All(rules); err != nil {
			return nil, err
		}
		for _, r := range *rules {
			out[string(careplan.SourceRule)+"|"+r.ID.String()] = string(r.Schedule)
		}
	}
	if len(planIDs) > 0 {
		plans := &models.CareAnimalPlans{}
		if err := tx.Where("id IN (?)", planIDs).All(plans); err != nil {
			return nil, err
		}
		for _, p := range *plans {
			out[string(careplan.SourceAnimal)+"|"+p.ID.String()] = string(p.Schedule)
		}
	}
	return out, nil
}

// sortProtocolSources: animal plans first (they override the global
// protocol), then rules, each group alphabetical by name — a stable,
// readable order.
func sortProtocolSources(sources []ProtocolSourceView) {
	sort.SliceStable(sources, func(i, j int) bool {
		ai := sources[i].SourceType == string(careplan.SourceAnimal)
		aj := sources[j].SourceType == string(careplan.SourceAnimal)
		if ai != aj {
			return ai
		}
		return sources[i].Name < sources[j].Name
	})
}
