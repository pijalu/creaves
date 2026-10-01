package actions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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

	// Round-2 §4b-A2 (bugs.md Dash-9): animals outtaken TODAY stay in the
	// day-plan assemblies (same-day work remains visible + recordable via
	// the late path); older outtakes never enter. Strict scopes keep their
	// own rules (CountOpenItems, matcher previews).
	pa, err := loadAnimalContextsIncludingTodayOuttaken(tx, now, nil)
	if err != nil {
		return nil, err
	}
	sources, planAnimal, err := loadPlanSources(tx)
	if err != nil {
		return nil, err
	}
	members, err := buildMemberships(tx, sources, pa, from)
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
// from is the plan window start: the latch query only reads applications
// within the longest possible course horizon before it (M3).
func buildMemberships(tx *pop.Connection, sources []careplan.PlanSource, pa *planAnimals, from time.Time) (map[string]map[int]bool, error) {
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

	// parsed AST cache (§4.4/§7.1-6 fail-closed: a broken — or missing —
	// matcher expression matches NOTHING; only a rule WITHOUT a matcher
	// matches every animal). Parse failures are cached like successes.
	type parsedMatcher struct {
		node careplan.Node
		ok   bool
	}
	astCache := map[uuid.UUID]parsedMatcher{}
	parseExpr := func(id uuid.UUID) parsedMatcher {
		if p, cached := astCache[id]; cached {
			return p
		}
		var p parsedMatcher
		if e, present := exprs[id]; present {
			if parsed, err := careplan.ParseValidatedWith(e, careplan.DefaultRegistry()); err == nil {
				p.node, p.ok = parsed, true
			}
		}
		astCache[id] = p
		return p
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

	// course latch (§5.4): applications prove past membership. The query is
	// bounded by the longest possible course horizon (M3): an application
	// older than from − horizon belongs to a course whose last generated
	// day precedes the window — it cannot produce in-window occurrences.
	maxCourseDays := 0
	for i := range rules {
		if !rules[i].LatchMembership {
			continue
		}
		if d := courseHorizonDays(&rules[i]); d > maxCourseDays {
			maxCourseDays = d
		}
	}
	latched := map[uuid.UUID]map[int]bool{}
	if maxCourseDays > 0 {
		latchFrom := from.AddDate(0, 0, -maxCourseDays)
		var latchApps []models.CarePlanApplication
		if err := tx.Where("source_type = ?", models.ApplicationSourceRule).Where("source_id in (?)", ruleIDs).Where("due_at >= ?", latchFrom).All(&latchApps); err != nil {
			return nil, err
		}
		for _, a := range latchApps {
			if latched[a.SourceID] == nil {
				latched[a.SourceID] = map[int]bool{}
			}
			latched[a.SourceID][a.AnimalID] = true
		}
	}

	reg := careplan.DefaultRegistry()
	for i := range rules {
		r := &rules[i]
		set := map[int]bool{}
		var node careplan.Node
		if r.MatcherID.Valid {
			p := parseExpr(r.MatcherID.UUID)
			if !p.ok {
				// §4.4/§7.1-6 fail-closed: broken matcher matches nothing.
				members[r.ID.String()] = set
				continue
			}
			node = p.node
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

// courseHorizonDays returns the longest calendar span a course of this
// rule can cover, first to last generated day (§10-B3): duration_days
// counts generated days stepping every_days; a weekday filter can stretch
// the calendar span up to 7× (worst case 1 passing day per week). Broken
// or open-ended schedules report 0 (no latch possible, §7.1-6).
func courseHorizonDays(r *models.CareRule) int {
	sched, err := careplan.ParseScheduleJSON(r.Schedule)
	if err != nil || sched.DurationDays <= 0 {
		return 0
	}
	every := sched.EveryDays
	if every < 1 {
		every = 1
	}
	span := (sched.DurationDays - 1) * every
	if len(sched.Weekdays) > 0 {
		span *= 7
	}
	return span + 1
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
		apps[occurrenceKeyParts(a.SourceType, a.SourceID.String(), a.AnimalID, a.DueAt)] = applicationViewOf(a)
	}
	return apps, nil
}

// applicationViewOf projects one stored application row into the engine
// read model (bugs.md U3: fulfillment link included).
func applicationViewOf(a *models.CarePlanApplication) *careplan.ApplicationView {
	v := &careplan.ApplicationView{
		Status:             a.Status,
		AppliedAt:          a.AppliedAt,
		DeferredUntil:      a.DeferredUntil,
		FulfillmentDeleted: a.FulfillmentDeleted,
	}
	if a.FulfillmentID != "" && a.FulfillmentID != planFulfillmentNone {
		v.FulfillmentType = a.FulfillmentType
		v.FulfillmentID = a.FulfillmentID
	}
	return v
}

// occurrenceKeyParts mirrors careplan.OccurrenceKey for stored rows.
func occurrenceKeyParts(sourceType, sourceID string, animalID int, dueAt time.Time) string {
	return fmt.Sprintf("%s|%s|%d|%d", sourceType, sourceID, animalID, dueAt.UnixNano())
}

// CountOpenItems computes the landing badge counters (§7.2): open (due or
// missing) and late items at `now`. It runs the same membership/occurrence
// pipeline as BuildDayPlan but only materializes occurrences up to the
// badge horizon — no occurrence due after now+lookahead can be open or
// late, so the +2 day display window is never generated (M3).
func CountOpenItems(tx *pop.Connection, now time.Time) (open, late int, err error) {
	from, to := DefaultPlanWindow(now)
	pa, err := loadAnimalContexts(tx, now)
	if err != nil {
		return 0, 0, err
	}
	sources, planAnimal, err := loadPlanSources(tx)
	if err != nil {
		return 0, 0, err
	}
	members, err := buildMemberships(tx, sources, pa, from)
	if err != nil {
		return 0, 0, err
	}

	// badge horizon: statuses due/late/missing all require
	// due_at ≤ now + lookahead (§10-CP6a).
	maxLookahead := time.Duration(0)
	for _, src := range sources {
		if l := time.Duration(src.Schedule().LookaheadMinutes) * time.Minute; l > maxLookahead {
			maxLookahead = l
		}
	}
	horizon := now.Add(maxLookahead)
	if to.Before(horizon) {
		horizon = to
	}

	var occs []careplan.Occurrence
	for _, src := range sources {
		for _, animalID := range sourceAnimals(src, planAnimal, members, pa) {
			occs = append(occs, careplan.GenerateOccurrences(src, pa.ctxs[animalID], from, horizon)...)
		}
	}
	apps, err := loadApplications(tx, from, horizon)
	if err != nil {
		return 0, 0, err
	}
	for _, it := range careplan.BuildPlanItems(occs, apps, now, horizon) {
		switch it.Status {
		case careplan.StatusDue, careplan.StatusMissing:
			open++
		case careplan.StatusLate:
			late++
		}
	}
	return open, late, nil
}

// ---------------------------------------------------------------------------
// Single-item re-verification (§6.2 step 1, M3): O(1) sources instead of a
// full plan rebuild on the apply paths.
// ---------------------------------------------------------------------------

// ReverifiedItem is the result of ReverifyItem: the plan item plus the
// animal context/row the apply paths need (defer clamp, cage checks).
type ReverifiedItem struct {
	Item   *careplan.PlanItem
	Ctx    *careplan.AnimalContext
	Animal models.Animal
}

// errItemNotReproduced mirrors findItem's "not found" failure: the
// referenced occurrence is not (or no longer) produced by its source.
var errItemNotReproduced = fmt.Errorf("occurrence not found or no longer produced by its source (§6.2)")

// ReverifyItem recomputes the day-plan item for ONE occurrence reference
// (§6.2): it loads just the referenced source, the referenced animal's
// matcher context, the same-kind animal plans of that animal (override
// resolution, §4.7) and the animal's in-window applications, then runs the
// same engine pipeline BuildDayPlan runs — the result is identical to the
// full-plan item for the reference.
func ReverifyItem(tx *pop.Connection, ref planItemRef, now time.Time) (*ReverifiedItem, error) {
	from, to := DefaultPlanWindow(now)

	// Round-2 §4b-A2: today-outtaken animals stay reproducible (their past
	// occurrences are recordable via the late path, §6.2-2); reproduction
	// is not the apply window.
	pa, err := loadAnimalContextsIncludingTodayOuttaken(tx, now, []int{ref.AnimalID})
	if err != nil {
		return nil, err
	}
	ctx := pa.ctxs[ref.AnimalID]
	if ctx == nil {
		return nil, errItemNotReproduced // animal out of care — nothing to plan
	}

	var occs []careplan.Occurrence
	switch ref.SourceType {
	case models.ApplicationSourceRule:
		var r models.CareRule
		if err := tx.Where("id = ? AND active = ?", ref.SourceID, true).First(&r); err != nil {
			return nil, errItemNotReproduced
		}
		src, err := careRuleSource(&r)
		if err != nil {
			return nil, errItemNotReproduced
		}
		member, err := ruleCoversAnimal(tx, &r, ctx, from)
		if err != nil {
			return nil, err
		}
		if !member {
			return nil, errItemNotReproduced
		}
		occs = careplan.GenerateOccurrences(src, ctx, from, to)
		// §4.7: same-kind animal plans of this animal can suppress the rule
		// occurrences — feed them to the override resolution.
		var plans []models.CareAnimalPlan
		if err := tx.Where("active = ? AND animal_id = ? AND action_kind = ?", true, ref.AnimalID, src.ActionKind()).All(&plans); err != nil {
			return nil, err
		}
		for i := range plans {
			if ps, err := careAnimalPlanSource(&plans[i]); err == nil {
				occs = append(occs, careplan.GenerateOccurrences(ps, ctx, from, to)...)
			}
		}
	case models.ApplicationSourceAnimal:
		var p models.CareAnimalPlan
		if err := tx.Where("id = ? AND active = ?", ref.SourceID, true).First(&p); err != nil {
			return nil, errItemNotReproduced
		}
		if p.AnimalID != ref.AnimalID {
			return nil, errItemNotReproduced
		}
		src, err := careAnimalPlanSource(&p)
		if err != nil {
			return nil, errItemNotReproduced
		}
		// plans never suppress plans and rules never suppress plans (§4.7):
		// no other source can affect this item.
		occs = careplan.GenerateOccurrences(src, ctx, from, to)
	default:
		return nil, errItemNotReproduced
	}

	var appRows []models.CarePlanApplication
	if err := tx.Where("animal_id = ? AND due_at >= ? AND due_at <= ?", ref.AnimalID, from, to).All(&appRows); err != nil {
		return nil, err
	}
	apps := make(map[string]*careplan.ApplicationView, len(appRows))
	for i := range appRows {
		a := &appRows[i]
		apps[occurrenceKeyParts(a.SourceType, a.SourceID.String(), a.AnimalID, a.DueAt)] = applicationViewOf(a)
	}

	items := careplan.BuildPlanItems(occs, apps, now, to)
	for i := range items {
		it := &items[i]
		s := it.Occurrence.Source
		if s == nil {
			continue
		}
		if string(s.SourceType()) == ref.SourceType &&
			s.SourceID() == ref.SourceID &&
			it.Occurrence.AnimalID == ref.AnimalID &&
			it.Occurrence.DueAt.Equal(ref.DueAt) {
			return &ReverifiedItem{Item: it, Ctx: ctx, Animal: pa.rows[ref.AnimalID]}, nil
		}
	}
	return nil, errItemNotReproduced
}

// ruleCoversAnimal reproduces the buildMemberships membership decision of
// one rule for one animal (§5): matcher evaluation (a broken or missing
// matcher expression matches NOTHING, §4.4/§7.1-6, as in buildMemberships;
// only a rule WITHOUT a matcher matches every animal), minus per-animal
// exclusion (§4.1), plus the §5.4 course latch — the latch union runs
// AFTER exclusions, exactly like the bulk path.
func ruleCoversAnimal(tx *pop.Connection, r *models.CareRule, ctx *careplan.AnimalContext, from time.Time) (bool, error) {
	matched := true
	if r.MatcherID.Valid {
		var m models.CareMatcher
		if err := tx.Find(&m, r.MatcherID.UUID); err != nil {
			return false, nil // missing row → fail-closed
		}
		node, perr := careplan.ParseValidatedWith(m.Expression, careplan.DefaultRegistry())
		if perr != nil {
			return false, nil // §4.4/§7.1-6 fail-closed: broken matcher
		}
		matched = careplan.Eval(careplan.DefaultRegistry(), node, ctx)
	}
	if matched {
		var ex []models.CareRuleExclusion
		if err := tx.Where("rule_id = ? AND animal_id = ?", r.ID, ctx.ID).All(&ex); err != nil {
			return false, err
		}
		if len(ex) > 0 {
			matched = false
		}
	}
	if matched {
		return true, nil
	}
	// §5.4 latch union (overrides matcher/exclusion misses).
	if r.LatchMembership && hasDuration(r) {
		latchFrom := from.AddDate(0, 0, -courseHorizonDays(r))
		var apps []models.CarePlanApplication
		if err := tx.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at >= ?",
			models.ApplicationSourceRule, r.ID, ctx.ID, latchFrom).All(&apps); err != nil {
			return false, err
		}
		if len(apps) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// CageCard is one (cleanup source × cage) group of the day plan.
type CageCard struct {
	Source careplan.PlanSource
	Zone   string
	Cage   string
	Items  []careplan.PlanItem
}

// FeedingChip is the per-animal in-card chip of a feeding card
// (bugs.md U1): year-number label plus the item's status (dot) and the
// occurrence reference the apply loop needs.
type FeedingChip struct {
	AnimalID   int
	Label      string // `472/26` (§10.5-N1 year number)
	Status     string
	SourceType string
	SourceID   string
	DueAt      time.Time
	Applicable bool
	AnimalLink string // set by the view-model layer (back-aware)
	// Round-2 §6.2-3/§9 (additive): chip dedupe + supersession info. The
	// chip is the animal's earliest CURRENT occurrence; a fully superseded
	// animal keeps a dimmed info chip pointing at the next due time.
	Superseded            bool
	SupersededBy          string // bare "15:04" of the replacing occurrence
	SupersededByDayKey    string // care_plan.time.yesterday / .tomorrow ("" otherwise)
	SupersededByShortDate string // "02/01" for gaps beyond one day
	OuttakenToday         bool   // §4b-A2: animal outtaken today (depupdate outtaken row)
	// Round-2 §7.1 (CP4 honesty): open CURRENT occurrences of the same
	// (source × animal) beyond the displayed chip — rendered "+N" so the
	// summary strip's occurrence counts stay visible (Zeigarnik).
	Remaining int
}

// FeedingCard is one (cage × normalized food) group of the day plan
// (bugs.md U1): the animals of a cage eat the same ration, so one card
// per diet — possibly spanning several feeding sources (rule + animal
// plans). Batch apply stays source×cage-scoped (§10.5-N2): the client
// loops one batch call per source.
type FeedingCard struct {
	Zone      string
	Cage      string
	Food      string
	ForceFeed bool
	Chips     []FeedingChip
	Items     []careplan.PlanItem

	// chipIndex maps (source × animal) → index into Chips (round-2 §6.2-3
	// dedupe: "per (animal × source) the chip is the earliest current
	// occurrence"); unexported, never rendered.
	chipIndex map[string]int
}

// GroupCards groups the plan items whose kind uses a cage-level grouping
// strategy (§6.2a + bugs.md U1 — registry-driven, GroupingByKind):
// cleanup → one card per (source × cage); feeding → one card per
// (cage × normalized food). Both sorts by zone then cage (round order).
func GroupCards(items []careplan.PlanItem, d *DayPlan) ([]*CageCard, []*FeedingCard) {
	type cageKey struct {
		sourceID, zone, cage string
	}
	type dietKey struct {
		zone, cage, food string
	}
	cageGroups := map[cageKey]*CageCard{}
	dietGroups := map[dietKey]*FeedingCard{}
	var cages []*CageCard
	var feedings []*FeedingCard
	for i := range items {
		it := items[i]
		if it.Occurrence.Source == nil {
			continue
		}
		src := it.Occurrence.Source
		zone, cage, label := "", "", ""
		if a, ok := d.AnimalRow(it.Occurrence.AnimalID); ok {
			zone, cage = a.Zone.String, a.Cage.String
			label = a.YearNumberFormatted()
		}
		if cage == "" {
			cage = "—" // uncaged animals still group (§10.5-N1 dash convention)
		}
		switch careplan.GroupingFor(src.ActionKind()) {
		case careplan.GroupingCage:
			k := cageKey{src.SourceID(), zone, cage}
			card, ok := cageGroups[k]
			if !ok {
				card = &CageCard{Source: src, Zone: zone, Cage: cage}
				cageGroups[k] = card
				cages = append(cages, card)
			}
			card.Items = append(card.Items, it)
		case careplan.GroupingCageDiet:
			p := parsePlanPayload(src)
			food := normalizeFood(p.Food)
			k := dietKey{zone, cage, food}
			card, ok := dietGroups[k]
			if !ok {
				card = &FeedingCard{Zone: zone, Cage: cage, Food: food, ForceFeed: p.ForceFeed}
				dietGroups[k] = card
				feedings = append(feedings, card)
			} else if p.ForceFeed {
				card.ForceFeed = true // any force-fed member flags the card
			}
			card.Items = append(card.Items, it)
			// Round-2 §6.2-3 chip dedupe — see feedingCardChip.
			feedingCardChip(card, &it, src, label, d)
		}
	}
	byZoneCage := func(zoneAt, cageAt func(int) string) func(int, int) bool {
		return func(i, j int) bool {
			if zoneAt(i) != zoneAt(j) {
				return zoneAt(i) < zoneAt(j)
			}
			return cageAt(i) < cageAt(j)
		}
	}
	sort.SliceStable(cages, byZoneCage(
		func(i int) string { return cages[i].Zone },
		func(i int) string { return cages[i].Cage }))
	sort.SliceStable(feedings, byZoneCage(
		func(i int) string { return feedings[i].Zone },
		func(i int) string { return feedings[i].Cage }))
	return cages, feedings
}

// feedingCardChip maintains the round-2 §6.2-3 (fixes CP6) dedupe: ONE chip
// per (animal × source) per card — the earliest CURRENT occurrence. A
// non-current occurrence never takes the chip slot from a current one
// (Jakob's: one next action per animal, like the old /feeding view). A fully
// superseded animal keeps a dimmed fallback chip with the next due label —
// the dead-end red "En retard" with no hint is gone by construction. Items
// arrive chronologically, so the first current occurrence wins and stays.
func feedingCardChip(card *FeedingCard, it *careplan.PlanItem, src careplan.PlanSource, label string, d *DayPlan) {
	if card.chipIndex == nil {
		card.chipIndex = map[string]int{}
	}
	cur := IsCurrent(it, d.Now)
	out := false
	if a, ok := d.AnimalRow(it.Occurrence.AnimalID); ok {
		out = a.Outtake != nil // preloaded only by the §4b-A2 scope
	}
	chipKey := string(src.SourceType()) + "|" + src.SourceID() + "|" + strconv.Itoa(it.Occurrence.AnimalID)
	if idx, seen := card.chipIndex[chipKey]; seen {
		if cur && card.Chips[idx].Superseded {
			card.Chips[idx] = feedingChipOf(it, src, label, out, d.Now)
		}
		return
	}
	card.Chips = append(card.Chips, feedingChipOf(it, src, label, out, d.Now))
	card.chipIndex[chipKey] = len(card.Chips) - 1
}

// feedingChipOf builds one FeedingChip from a plan item, carrying the
// supersession display info (round-2 §6.2-2): why the occurrence left the
// work set and what replaced it (date-aware label parts, §6.2-5 — the
// template localizes the day word).
func feedingChipOf(it *careplan.PlanItem, src careplan.PlanSource, label string, outtakenToday bool, now time.Time) FeedingChip {
	chip := FeedingChip{
		AnimalID:       it.Occurrence.AnimalID,
		Label:          label,
		Status:         string(it.Status),
		SourceType:     string(src.SourceType()),
		SourceID:       src.SourceID(),
		DueAt:          it.Occurrence.DueAt,
		Applicable:     it.Applicable,
		OuttakenToday:  outtakenToday,
	}
	if reason := SupersededReason(it, now); reason != "" {
		chip.Superseded = true
		parts := DueLabelPartsOf(it.NextDue, now)
		chip.SupersededBy = parts.TimeHM
		chip.SupersededByDayKey = parts.DayKey
		chip.SupersededByShortDate = parts.ShortDate
	}
	return chip
}

// normalizeFood is the diet grouping key (bugs.md U1): case- and
// whitespace-insensitive — "Croquettes  + VDF" and "croquettes + vdf"
// are the same ration.
func normalizeFood(food string) string {
	return strings.ToLower(strings.Join(strings.Fields(strings.TrimSpace(food)), " "))
}

// GroupCageCards groups the plan items whose kind uses cage grouping
// (§6.2a — registry-driven, GroupingByKind). Cards sort by zone then cage
// (round order, §6.2a). Kept for callers that only need cleanup cards.
func GroupCageCards(items []careplan.PlanItem, d *DayPlan) []*CageCard {
	cages, _ := GroupCards(items, d)
	return cages
}
