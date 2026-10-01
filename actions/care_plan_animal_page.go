package actions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// R5-4b (bugs.md U21 / D-c): the animal page Protocol tab lists the
// complete protocol and the Treatment tab keeps its original look but is
// fed by the protocol engine — per-time entries (R5-3) + the
// care_plan_applications done-history — for protocol-backed AND manual
// treatments. This file holds the server-side data builders behind both
// tabs; no legacy behavior is removed (entries are additive, R5-3).

// TreatmentProtocolLink is the source-protocol backlink of one treatment
// row on the animal page Treatment tab. Protocol-backed rows link to the
// protocol they came from (same-page anchor on the Protocol tab, or the
// care-rules library for rule occurrences that created a treatment).
type TreatmentProtocolLink struct {
	// Name is the display (marker-stripped) name of the source protocol.
	Name string
	// Href is the link target: "#nav-plan"-anchored animal URL for animal
	// plans, /care_rules for rule-backed occurrences.
	Href string
	// Rule marks a generic care-rule source (opens the rules library).
	Rule bool
}

// TreatmentApplicationIndex is the per-treatment application map of one
// animal plus the distinct protocol/rule source ids behind them.
type treatmentApplicationIndex struct {
	byTreatment map[string]models.CarePlanApplication
	planIDs     []uuid.UUID
	ruleIDs     []uuid.UUID
}

// setTreatmentProtocolLinks resolves, for every treatment of the animal,
// the care-plan application that created it (fulfillment_type=treatment)
// and the protocol it came from. Keyed by treatment UUID string;
// treatments without an application (manual / migrated rows) have no map
// entry and render without a backlink.
func setTreatmentProtocolLinks(tx *pop.Connection, c buffalo.Context, animal *models.Animal) error {
	links := map[string]*TreatmentProtocolLink{}
	c.Set("treatmentProtocolLinks", links)
	if len(animal.Treatments) == 0 {
		return nil
	}

	idx, err := loadTreatmentApplications(tx, animal)
	if err != nil {
		return err
	}
	planNames, err := planNamesByIDs(tx, idx.planIDs)
	if err != nil {
		return err
	}
	ruleNames, err := ruleNamesByIDs(tx, idx.ruleIDs)
	if err != nil {
		return err
	}

	for tid, a := range idx.byTreatment {
		if a.SourceType == models.ApplicationSourceAnimal {
			if name, ok := planNames[a.SourceID.String()]; ok {
				links[tid] = &TreatmentProtocolLink{
					Name: DisplayName(name),
					Href: fmt.Sprintf("/animals/%d#nav-plan", animal.ID),
				}
			}
		} else if name, ok := ruleNames[a.SourceID.String()]; ok {
			links[tid] = &TreatmentProtocolLink{
				Name: DisplayName(name),
				Href: "/care_rules",
				Rule: true,
			}
		}
	}
	return nil
}

// loadTreatmentApplications fetches the treatment-fulfilling applications
// of one animal and indexes them per treatment, collecting the distinct
// source ids per planning level.
func loadTreatmentApplications(tx *pop.Connection, animal *models.Animal) (*treatmentApplicationIndex, error) {
	ids := make([]uuid.UUID, 0, len(animal.Treatments))
	for i := range animal.Treatments {
		ids = append(ids, animal.Treatments[i].ID)
	}
	apps := &models.CarePlanApplications{}
	if err := tx.Where("animal_id = ?", animal.ID).
		Where("fulfillment_type = ?", models.ApplicationFulfillmentTreatment).
		Where("fulfillment_id IN (?)", ids).All(apps); err != nil {
		return nil, err
	}
	idx := &treatmentApplicationIndex{byTreatment: map[string]models.CarePlanApplication{}}
	for _, a := range *apps {
		if a.FulfillmentDeleted {
			continue
		}
		idx.byTreatment[a.FulfillmentID] = a
		if a.SourceType == models.ApplicationSourceAnimal {
			idx.planIDs = append(idx.planIDs, a.SourceID)
		} else {
			idx.ruleIDs = append(idx.ruleIDs, a.SourceID)
		}
	}
	return idx, nil
}

// planNamesByIDs loads the names of the animal plans behind the given ids.
func planNamesByIDs(tx *pop.Connection, ids []uuid.UUID) (map[string]string, error) {
	names := map[string]string{}
	if len(ids) == 0 {
		return names, nil
	}
	plans := &models.CareAnimalPlans{}
	if err := tx.Where("id IN (?)", ids).All(plans); err != nil {
		return nil, err
	}
	for _, p := range *plans {
		names[p.ID.String()] = p.Name
	}
	return names, nil
}

// ruleNamesByIDs loads the names of the care rules behind the given ids.
func ruleNamesByIDs(tx *pop.Connection, ids []uuid.UUID) (map[string]string, error) {
	names := map[string]string{}
	if len(ids) == 0 {
		return names, nil
	}
	rules := &models.CareRules{}
	if err := tx.Where("id IN (?)", ids).All(rules); err != nil {
		return nil, err
	}
	for _, r := range *rules {
		names[r.ID.String()] = r.Name
	}
	return names, nil
}

// entryClockSVG maps a per-time entry label ("11:30") to the accordion
// clock-dot SVG name (R5-4b): morning < 11h, noon < 15h, evening otherwise.
func entryClockSVG(label string) string {
	h, _ := strconv.Atoi(strings.SplitN(label, ":", 2)[0])
	switch {
	case h < 11:
		return "morningSVG"
	case h < 15:
		return "noonSVG"
	default:
		return "eveningSVG"
	}
}

// PlanItemDetail carries the server-resolved display fields of one plan
// occurrence for the shared detail modal (round-2 §8.2, Dash-7): the
// dashboard eye deep-links /animals/{id}?item=…&due=…#nav-treatment; the
// Show handler resolves the occurrence server-side and the show template
// renders the modal open — no fetch, deep-links work from bookmarks.
type PlanItemDetail struct {
	AnimalLabel string
	AnimalLink  string
	Detail      string
	SourceName  string
	SourceLink  string
	Kind        string // localized action kind
	Due         string // "15:04" — deep links target today's occurrences
	Status      string // localized occurrence status
}

// AnimalTodayBlock is the protocol-driven TODAY block of the animal
// Treatment tab (round-2 §10, T1/T2): today's plan occurrences of the
// medication + care kinds for THIS animal as drug/care series with
// hour-labeled toggle buttons — the same component the dashboard and
// the care plan render (Similarity: one visual language).
type AnimalTodayBlock struct {
	// Label is the animal header ("472/26 · Hérisson · A12").
	Label string
	// Group carries the combined series — the template binds it to `mg`
	// so the shared _med_series partial renders it unchanged.
	Group MedGroupView
	// Empty reports "no protocol work today" (block not rendered).
	Empty bool
}

// animalTodayPlan builds today's plan (TodayPlanWindow) — the single
// engine pass behind the Treatment tab's Today block AND the ?item=
// deep-link resolution (§8.2/§10 share one assembly).
func animalTodayPlan(tx *pop.Connection) (*DayPlan, error) {
	now := time.Now()
	from, to := TodayPlanWindow(now)
	return BuildDayPlan(tx, now, from, to)
}

// animalTodayBlock folds today's plan into the ONE animal's Today block:
// its medication series (dashboard todayOnly semantics — overridden
// suppressed) plus its care occurrences as series lines.
func animalTodayBlock(plan *DayPlan, animal *models.Animal) AnimalTodayBlock {
	b := AnimalTodayBlock{Label: animalLabel(*animal)}
	v := &DayPlanView{View: ViewCompact, SelfPath: "/"}
	for _, mg := range v.buildMedGroups(plan, "", true) {
		if mg.AnimalID == animal.ID {
			b.Group = mg
			break
		}
	}
	b.Group.AnimalID = animal.ID
	b.Group.AnimalLabel = b.Label
	// Self link WITHOUT the back chain and without a tab hash: the modal's
	// animal link must stay on the current tab.
	b.Group.AnimalLink = fmt.Sprintf("/animals/%d", animal.ID)
	b.Group.Series = append(b.Group.Series, careSeriesOf(plan, animal.ID)...)
	b.Empty = len(b.Group.Series) == 0
	return b
}

// todaySlotView projects ONE today-window occurrence into the shared slot
// view (same fields the dashboard/care-plan pipeline fills). buildMedGroups
// keeps its inline copy — that function is a known pre-existing complexity
// offender and stays untouched.
func todaySlotView(plan *DayPlan, it *careplan.PlanItem, back string) MedSlotView {
	src := it.Occurrence.Source
	slot := MedSlotView{
		Slot:       medSlotOf(it.Occurrence.DueAt),
		Detail:     planDetail(src),
		SourceName: DisplayName(src.Name()),
		SourceType: string(src.SourceType()),
		SourceID:   src.SourceID(),
		DueAt:      it.Occurrence.DueAt,
		DueAtRFC:   it.Occurrence.DueAt.Format("2006-01-02T15:04:05Z07:00"),
		DueAtHM:    it.Occurrence.DueAt.Format("15:04"),
		Status:     string(it.Status),
		Applied:    it.Status == careplan.StatusApplied,
		Applicable: it.Applicable,
		Overridden: it.Status == careplan.StatusOverridden,
		SourceLink: cardSourceLink(string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID, back),
	}
	slot.Done = slot.Applied || it.Status == careplan.StatusSkipped || it.Status == careplan.StatusDeferred
	slot.LateAllowed = slotLateAllowed(plan.Now, slot, it)
	if app := it.Application; app != nil {
		slot.CanUndo = slot.Applied && app.FulfillmentType == models.ApplicationFulfillmentTreatment &&
			app.FulfillmentID != "" && app.FulfillmentID != planFulfillmentNone && !app.FulfillmentDeleted
		slot.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted, back)
	}
	slot.ViewLink = slot.FulfillmentLink
	if slot.ViewLink == "" {
		slot.ViewLink = fmt.Sprintf("/animals/%d#nav-treatment", it.Occurrence.AnimalID)
	}
	slot.DeepLink = animalItemDeepLink(it.Occurrence.AnimalID, string(src.SourceType()), src.SourceID(), slot.DueAtRFC)
	return slot
}

// careSeriesOf folds today's CARE occurrences of one animal into series
// (one per source) so the shared _med_series partial renders them next to
// the medication series (§10 T1: the Today block lists the protocol's
// medications AND cares). Overridden occurrences are suppressed (same as
// the dashboard todayOnly mode).
func careSeriesOf(plan *DayPlan, animalID int) []MedSeriesView {
	order := map[string]int{"morning": 0, "noon": 1, "evening": 2}
	var slots []MedSlotView
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animalID ||
			src.ActionKind() != careplan.KindCare || it.Status == careplan.StatusOverridden {
			continue
		}
		if it.Occurrence.DueAt.Before(plan.From) || it.Occurrence.DueAt.After(plan.To) {
			continue
		}
		slots = append(slots, todaySlotView(plan, it, ""))
	}
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].DueAt.Equal(slots[j].DueAt) {
			return order[slots[i].Slot] < order[slots[j].Slot]
		}
		return slots[i].DueAt.Before(slots[j].DueAt)
	})
	return seriesOf(slots, order)
}

// resolvePlanItemDetail handles the ?item=&due= deep link on the animal
// page (§8.2): find the occurrence in today's plan for THIS animal and
// pass its display fields plus openPlanDetail to the template. An
// unparseable ref or a stale occurrence (plan gone, already purged)
// resolves to nothing — the page renders normally without a popup.
func resolvePlanItemDetail(c buffalo.Context, animal *models.Animal, plan *DayPlan, ref, dueRaw string) error {
	srcType, srcID, ok := parseItemRef(ref)
	if !ok {
		return nil
	}
	var due time.Time
	if dueRaw != "" {
		if d, err := time.Parse(time.RFC3339, dueRaw); err == nil {
			due = d
		}
	}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animal.ID ||
			string(src.SourceType()) != srcType || src.SourceID() != srcID {
			continue
		}
		if !due.IsZero() && !it.Occurrence.DueAt.Equal(due) {
			continue
		}
		c.Set("planItemDetail", PlanItemDetail{
			AnimalLabel: animalLabel(*animal),
			AnimalLink:  fmt.Sprintf("/animals/%d", animal.ID),
			Detail:      planDetail(src),
			SourceName:  DisplayName(src.Name()),
			SourceLink:  cardSourceLink(string(src.SourceType()), src.SourceID(), animal.ID, ""),
			Kind:        T.Translate(c, "care_plan.kind."+string(src.ActionKind())),
			Due:         it.Occurrence.DueAt.Format("15:04"),
			Status:      T.Translate(c, "care_plan.status."+string(it.Status)),
		})
		c.Set("openPlanDetail", true)
		return nil
	}
	return nil
}

// parseItemRef splits the ?item= reference "<source_type>:<source_id>".
func parseItemRef(ref string) (string, string, bool) {
	i := strings.Index(ref, ":")
	if i <= 0 || i == len(ref)-1 {
		return "", "", false
	}
	return ref[:i], ref[i+1:], true
}

// planWindow renders the active window of a plan schedule as ISO dates
// "YYYY-MM-DD → YYYY-MM-DD" (or "YYYY-MM-DD → ∞" when open-ended) —
// R5-4b (U21): the Protocol tab shows the complete protocol incl. when it
// runs. Resolves the intake-anchored start from the animal's intake date;
// returns "—" when the schedule is unparseable (legacy display fallback)
// or the intake date is unknown.
func planWindow(a *models.Animal, raw string) string {
	start, end, ok := planWindowBounds(a, raw)
	if !ok {
		return "—"
	}
	if end == nil {
		return start.Format("2006-01-02") + " → ∞"
	}
	return start.Format("2006-01-02") + " → " + end.Format("2006-01-02")
}

// planExpired reports whether a plan's window has already ended
// (duration-bounded schedule whose last day is in the past).
func planExpired(a *models.Animal, raw string) bool {
	_, end, ok := planWindowBounds(a, raw)
	return ok && end != nil && end.Before(time.Now())
}

// planWindowBounds resolves the [start, end] window of a §4.3 schedule:
// fixed anchor → AnchorDate + FromOffsetDays; intake anchor → the
// animal's intake date + FromOffsetDays. end is nil for open-ended
// schedules; ok is false when the schedule cannot be resolved.
func planWindowBounds(a *models.Animal, raw string) (time.Time, *time.Time, bool) {
	s, err := careplan.ParseScheduleJSON([]byte(raw))
	if err != nil {
		return time.Time{}, nil, false
	}
	var start time.Time
	switch s.Anchor {
	case careplan.AnchorIntake:
		if a == nil || a.Intake.Date.IsZero() {
			return time.Time{}, nil, false
		}
		start = a.Intake.Date
	case careplan.AnchorFixed:
		if s.AnchorDate == nil {
			return time.Time{}, nil, false
		}
		start = *s.AnchorDate
	default:
		return time.Time{}, nil, false
	}
	start = time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, start.Location()).
		AddDate(0, 0, s.FromOffsetDays)
	if s.DurationDays <= 0 {
		return start, nil, true
	}
	end := start.AddDate(0, 0, s.DurationDays-1)
	return start, &end, true
}
