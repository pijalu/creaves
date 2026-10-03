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
	// R4-7.22: Active is the stored soft switch. The trace now also lists
	// protocols that produced NOTHING (appendIdleAnimalPlans), so an
	// inactive protocol is listed — and without this flag it would be
	// indistinguishable from a live one. Measured on animals/10221: one
	// definition was already inactive.
	Active bool
	// R4-7.13: per-animal opt-out (§4.1). A rule excluded for THIS animal
	// produces no occurrence, so under the "evidence, not catalogue" rule it
	// would simply vanish from the trace and become impossible to undo. The
	// excluded rows are therefore listed as SUPPRESSED: they keep their
	// evidence row, flagged, with the reason and the id needed to restore.
	Excluded        bool
	ExclusionID     string
	ExclusionReason string
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
	// R4-7.22: the animal's OWN protocols are listed whether or not they
	// produced anything today. They are this animal's care plan, and the
	// caregiver must be able to find, edit or delete one that has gone quiet
	// — measured on animals/10221, two protocols lived ONLY in the duplicate
	// definitions table, and dropping that table would have made them
	// unreachable. Global rules keep the stricter rule (evidence, not
	// catalogue): only the ones that actually apply are listed.
	if err := appendIdleAnimalPlans(tx, animal, trace); err != nil {
		return nil, err
	}
	if err := appendExcludedRules(tx, animal, trace, admin); err != nil {
		return nil, err
	}
	schedules, err := protocolSchedules(tx, ruleIDs, planIDs)
	if err != nil {
		return nil, err
	}
	for i := range trace.Sources {
		// An appended idle plan already carries its own schedule; only fill in
		// the ones the occurrence pass asked for.
		if raw, ok := schedules[trace.Sources[i].SourceType+"|"+trace.Sources[i].SourceID]; ok {
			trace.Sources[i].Schedule = raw
		}
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
		Active:      src.Active(),
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

// appendIdleAnimalPlans adds every animal protocol that produced no
// occurrence to the trace, so merging the trace with the old definitions table
// cannot make a protocol unreachable.
//
// The occurrence pass above returns the ids whose schedule still needs loading;
// an appended row has to contribute its own.
func appendIdleAnimalPlans(tx *pop.Connection, animal *models.Animal, trace *ProtocolTraceView) error {
	if animal == nil {
		return nil
	}
	plans := &models.CareAnimalPlans{}
	if err := tx.Where("animal_id = ?", animal.ID).All(plans); err != nil {
		return err
	}
	seen := make(map[string]bool, len(trace.Sources))
	for _, s := range trace.Sources {
		if s.SourceType == string(careplan.SourceAnimal) {
			seen[s.SourceID] = true
		}
	}
	for _, p := range *plans {
		id := p.ID.String()
		if seen[id] {
			continue
		}
		seen[id] = true
		var payload planPayload
		_ = jsonUnmarshalStrictish(p.ActionPayload, &payload)
		trace.Sources = append(trace.Sources, ProtocolSourceView{
			SourceType:  string(careplan.SourceAnimal),
			SourceID:    id,
			Name:        DisplayName(p.Name),
			Kind:        p.ActionKind,
			Content:     planDetailOf(p.ActionKind, payload),
			Schedule:    string(p.Schedule),
			EditURL:     fmt.Sprintf("/animals/%d#nav-plan", animal.ID),
			Editable:    true,
			Replaces:    p.ReplacesKind,
			Occurrences: 0,
			Active:      p.Active,
		})
	}
	return nil
}

// appendExcludedRules marks (or adds) the care rules this animal is opted out
// of (§4.1).
//
// The engine checks an exclusion BEFORE the §5.4 course latch, so a rule that
// still has a recent application history can produce occurrences despite the
// opt-out. Such a row is already in the trace from its occurrences and only
// needs the flag; a rule with no occurrence at all is APPENDED, otherwise the
// caregiver would have no way to remove the exception they added.
func appendExcludedRules(tx *pop.Connection, animal *models.Animal, trace *ProtocolTraceView, admin bool) error {
	if animal == nil {
		return nil
	}
	var exclusions []models.CareRuleExclusion
	if err := tx.Where("animal_id = ?", animal.ID).All(&exclusions); err != nil {
		return err
	}
	missing := markExcludedSources(exclusions, trace)
	return appendMissingExcludedRules(tx, missing, exclusions, trace, admin)
}

// markExcludedSources flags every excluded rule that already has a trace row
// and returns the ids of those that do not.
func markExcludedSources(exclusions []models.CareRuleExclusion, trace *ProtocolTraceView) []uuid.UUID {
	present := traceRuleRows(trace)
	var missing []uuid.UUID
	seen := map[uuid.UUID]struct{}{}
	for _, e := range exclusions {
		if i, ok := present[e.RuleID]; ok {
			trace.Sources[i].Excluded = true
			trace.Sources[i].ExclusionID = e.ID.String()
			trace.Sources[i].ExclusionReason = e.Reason.String
			continue
		}
		// Two exclusion rows for the same rule would append it twice; the
		// first one wins and keeps the trace idempotent.
		if _, dup := seen[e.RuleID]; dup {
			continue
		}
		seen[e.RuleID] = struct{}{}
		missing = append(missing, e.RuleID)
	}
	return missing
}

// traceRuleRows indexes the trace's GLOBAL-RULE rows by rule id — the only
// ones an exclusion can apply to.
func traceRuleRows(trace *ProtocolTraceView) map[uuid.UUID]int {
	present := map[uuid.UUID]int{}
	for i := range trace.Sources {
		if trace.Sources[i].SourceType != string(careplan.SourceRule) {
			continue
		}
		if u, err := uuid.FromString(trace.Sources[i].SourceID); err == nil {
			present[u] = i
		}
	}
	return present
}

// appendMissingExcludedRules adds a row per excluded rule the engine emitted
// nothing for. The engine no longer produces these sources, so there is no
// PlanSource to project from — the row is built from the rule row itself.
func appendMissingExcludedRules(tx *pop.Connection, missing []uuid.UUID,
	exclusions []models.CareRuleExclusion, trace *ProtocolTraceView, admin bool) error {
	if len(missing) == 0 {
		return nil
	}
	rules := &models.CareRules{}
	if err := tx.Where("id IN (?)", missing).All(rules); err != nil {
		return err
	}
	reason := map[uuid.UUID]models.CareRuleExclusion{}
	for _, e := range exclusions {
		reason[e.RuleID] = e
	}
	for _, r := range *rules {
		trace.Sources = append(trace.Sources, suppressedRuleView(r, reason[r.ID], admin))
	}
	return nil
}

// suppressedRuleView projects one suppressed rule into its trace row.
func suppressedRuleView(r models.CareRule, e models.CareRuleExclusion, admin bool) ProtocolSourceView {
	var payload planPayload
	_ = jsonUnmarshalStrictish(r.ActionPayload, &payload)
	return ProtocolSourceView{
		SourceType:      string(careplan.SourceRule),
		SourceID:        r.ID.String(),
		Name:            DisplayName(r.Name),
		Kind:            r.ActionKind,
		Content:         planDetailOf(r.ActionKind, payload),
		Schedule:        string(r.Schedule),
		EditURL:         "/care_rules/" + r.ID.String(),
		Editable:        admin,
		Active:          r.Active,
		Excluded:        true,
		ExclusionID:     e.ID.String(),
		ExclusionReason: e.Reason.String,
	}
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
