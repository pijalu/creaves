package actions

import (
	"fmt"
	"sort"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Day plan assembly (§6.1): membership resolution (§5) + occurrence
// generation + applications join. Handlers receive the read model — all
// status math stays in the DB-free careplan engine.

// DayPlan is the assembled §6.1 read model for one window.
type DayPlan struct {
	Items      []careplan.PlanItem
	Animals    *planAnimals
	PlanAnimal map[string]int // animal-plan source id → animal id (§4.7)
	From       time.Time
	To         time.Time
	Now        time.Time
}

// ContextOf returns the assembled matcher context of one animal.
func (d *DayPlan) ContextOf(animalID int) *careplan.AnimalContext {
	return d.Animals.ctxs[animalID]
}

// AnimalRow returns the raw animal row (labels: §10.5-N1).
func (d *DayPlan) AnimalRow(animalID int) (models.Animal, bool) {
	a, ok := d.Animals.rows[animalID]
	return a, ok
}

// BuildDayPlan assembles the day plan for [from, to] (clamped to the §6.1
// 14-day cap). from/to zero means the default window.
func BuildDayPlan(tx *pop.Connection, now, from, to time.Time) (*DayPlan, error) {
	if from.IsZero() || to.IsZero() || to.Before(from) {
		from, to = DefaultPlanWindow(now)
	}
	if d := to.Sub(from); d > planWindowMaxDays*24*time.Hour {
		to = from.Add(planWindowMaxDays * 24 * time.Hour)
	}

	pa, err := loadAnimalContexts(tx, now)
	if err != nil {
		return nil, err
	}
	sources, planAnimal, err := loadPlanSources(tx)
	if err != nil {
		return nil, err
	}
	members, err := buildMemberships(tx, sources, pa)
	if err != nil {
		return nil, err
	}

	var occs []careplan.Occurrence
	for _, src := range sources {
		for _, animalID := range sourceAnimals(src, planAnimal, members, pa) {
			occs = append(occs, careplan.GenerateOccurrences(src, pa.ctxs[animalID], from, to)...)
		}
	}

	apps, err := loadApplications(tx, from, to)
	if err != nil {
		return nil, err
	}

	return &DayPlan{
		Items:      careplan.BuildPlanItems(occs, apps, now, to),
		Animals:    pa,
		PlanAnimal: planAnimal,
		From:       from,
		To:         to,
		Now:        now,
	}, nil
}

// sourceAnimals yields the animals a source currently drives (§6.1 step 1):
// rules iterate their membership; animal plans their single animal.
func sourceAnimals(src careplan.PlanSource, planAnimal map[string]int, members map[string]map[int]bool, pa *planAnimals) []int {
	if src.SourceType() == careplan.SourceAnimal {
		id := planAnimal[src.SourceID()]
		if id == 0 {
			return nil
		}
		if _, ok := pa.ctxs[id]; !ok {
			return nil // animal out of care — nothing to plan
		}
		return []int{id}
	}
	set := members[src.SourceID()]
	if len(set) == 0 {
		return nil
	}
	ids := make([]int, 0, len(set))
	for id := range set {
		if _, ok := pa.ctxs[id]; ok {
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	return ids
}

// buildMemberships resolves which animals each active rule covers (§5):
// matcher DSL evaluation per in-care animal, minus per-animal exclusions,
// plus the §5.4 course-latch (matched once → stays until course end).
func buildMemberships(tx *pop.Connection, sources []careplan.PlanSource, pa *planAnimals) (map[string]map[int]bool, error) {
	members := map[string]map[int]bool{}
	var ruleIDs []uuid.UUID
	for _, src := range sources {
		if src.SourceType() == careplan.SourceRule {
			if id, err := uuid.FromString(src.SourceID()); err == nil {
				ruleIDs = append(ruleIDs, id)
			}
		}
	}
	if len(ruleIDs) == 0 {
		return members, nil
	}

	var rules []models.CareRule
	if err := tx.Where("id in (?)", ruleIDs).All(&rules); err != nil {
		return nil, err
	}

	// matcher expressions referenced by the rules
	matcherIDs := make([]uuid.UUID, 0, len(rules))
	for _, r := range rules {
		if r.MatcherID.Valid {
			matcherIDs = append(matcherIDs, r.MatcherID.UUID)
		}
	}
	exprs := map[uuid.UUID]string{}
	if len(matcherIDs) > 0 {
		var ms []models.CareMatcher
		if err := tx.Where("id in (?)", matcherIDs).All(&ms); err != nil {
			return nil, err
		}
		for _, m := range ms {
			exprs[m.ID] = m.Expression
		}
	}

	// parsed AST cache (§7.1-6: broken matchers match nothing, no plan error)
	astCache := map[uuid.UUID]careplan.Node{}
	parseExpr := func(id uuid.UUID) careplan.Node {
		if n, ok := astCache[id]; ok {
			return n
		}
		var n careplan.Node
		if e, ok := exprs[id]; ok {
			if parsed, err := careplan.ParseValidatedWith(e, careplan.DefaultRegistry()); err == nil {
				n = parsed
			}
		}
		astCache[id] = n
		return n
	}

	// per-animal exclusions (§4.1)
	excluded := map[uuid.UUID]map[int]bool{}
	var exclusions []models.CareRuleExclusion
	if err := tx.Where("rule_id in (?)", ruleIDs).All(&exclusions); err != nil {
		return nil, err
	}
	for _, e := range exclusions {
		if excluded[e.RuleID] == nil {
			excluded[e.RuleID] = map[int]bool{}
		}
		excluded[e.RuleID][e.AnimalID] = true
	}

	// course latch (§5.4): applications prove past membership
	latched := map[uuid.UUID]map[int]bool{}
	var latchApps []models.CarePlanApplication
	if err := tx.Where("source_type = ?", models.ApplicationSourceRule).Where("source_id in (?)", ruleIDs).All(&latchApps); err != nil {
		return nil, err
	}
	for _, a := range latchApps {
		if latched[a.SourceID] == nil {
			latched[a.SourceID] = map[int]bool{}
		}
		latched[a.SourceID][a.AnimalID] = true
	}

	reg := careplan.DefaultRegistry()
	for i := range rules {
		r := &rules[i]
		set := map[int]bool{}
		var node careplan.Node
		if r.MatcherID.Valid {
			node = parseExpr(r.MatcherID.UUID)
		}
		for id, ctx := range pa.ctxs {
			if node != nil && !careplan.Eval(reg, node, ctx) {
				continue
			}
			if excluded[r.ID][id] {
				continue
			}
			set[id] = true
		}
		if r.LatchMembership && hasDuration(r) {
			for id := range latched[r.ID] {
				if _, inCare := pa.ctxs[id]; inCare {
					set[id] = true
				}
			}
		}
		members[r.ID.String()] = set
	}
	return members, nil
}

// hasDuration reports whether the rule's schedule bounds a course (§5.4).
func hasDuration(r *models.CareRule) bool {
	sched, err := careplan.ParseScheduleJSON(r.Schedule)
	return err == nil && sched.DurationDays > 0
}

// loadApplications loads the window's application rows keyed by occurrence
// key (§6.1 step 3 — one query).
func loadApplications(tx *pop.Connection, from, to time.Time) (map[string]*careplan.ApplicationView, error) {
	var rows []models.CarePlanApplication
	if err := tx.Where("due_at >= ? AND due_at <= ?", from, to).All(&rows); err != nil {
		return nil, err
	}
	apps := make(map[string]*careplan.ApplicationView, len(rows))
	for i := range rows {
		a := &rows[i]
		apps[occurrenceKeyParts(a.SourceType, a.SourceID.String(), a.AnimalID, a.DueAt)] = &careplan.ApplicationView{
			Status:             a.Status,
			AppliedAt:          a.AppliedAt,
			DeferredUntil:      a.DeferredUntil,
			FulfillmentDeleted: a.FulfillmentDeleted,
		}
	}
	return apps, nil
}

// occurrenceKeyParts mirrors careplan.OccurrenceKey for stored rows.
func occurrenceKeyParts(sourceType, sourceID string, animalID int, dueAt time.Time) string {
	return fmt.Sprintf("%s|%s|%d|%d", sourceType, sourceID, animalID, dueAt.UnixNano())
}

// ---------------------------------------------------------------------------
// Cage-scoped aggregation (§6.2a): cleanup renders one card per (source × cage)
// ---------------------------------------------------------------------------

// CageCard is one (cleanup source × cage) group of the day plan.
type CageCard struct {
	Source careplan.PlanSource
	Zone   string
	Cage   string
	Items  []careplan.PlanItem
}

// GroupCageCards groups the plan items whose kind uses cage grouping
// (§6.2a — registry-driven, GroupingByKind). Cards sort by zone then cage
// (round order, §6.2a). Items of animal-grouped kinds are returned as-is.
func GroupCageCards(items []careplan.PlanItem, d *DayPlan) []*CageCard {
	type key struct {
		sourceID, zone, cage string
	}
	groups := map[key]*CageCard{}
	var cards []*CageCard
	for i := range items {
		it := items[i]
		if it.Occurrence.Source == nil {
			continue
		}
		if careplan.GroupingFor(it.Occurrence.Source.ActionKind()) != careplan.GroupingCage {
			continue
		}
		zone, cage := "", ""
		if a, ok := d.AnimalRow(it.Occurrence.AnimalID); ok {
			zone, cage = a.Zone.String, a.Cage.String
		}
		k := key{it.Occurrence.Source.SourceID(), zone, cage}
		card, ok := groups[k]
		if !ok {
			card = &CageCard{Source: it.Occurrence.Source, Zone: zone, Cage: cage}
			groups[k] = card
			cards = append(cards, card)
		}
		card.Items = append(card.Items, it)
	}
	sort.SliceStable(cards, func(i, j int) bool {
		if cards[i].Zone != cards[j].Zone {
			return cards[i].Zone < cards[j].Zone
		}
		return cards[i].Cage < cards[j].Cage
	})
	return cards
}
