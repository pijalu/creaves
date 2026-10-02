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

// AnimalPlanTodayRow is ONE plan occurrence in the animal Treatment
// tab's current-date accordion card (bugs.md U26 — fix 7): today's
// medication, observation and care occurrences merge into the legacy
// treatment list with the original look (label + protocol backlink +
// clock badge) — the dedicated Today block is gone, new replaces old
// instead of sitting on top of it.
type AnimalPlanTodayRow struct {
	// Label is the content line: medication "Drug (dosage)", observation
	// prompt, care note (falling back to instructions, then source name).
	Label string
	// DueHM is the due time "15:04" (drives the clock-dot badge).
	DueHM string
	// Status is "done", "skipped" (skipped or deferred) or "pending";
	// open occurrences due earlier today are "missed".
	Status string
	// AppliedAt is the "15:04" apply time of done rows ("" otherwise).
	AppliedAt string
	// Protocol backlink of the source plan / care rule (same fields as
	// TreatmentProtocolLink — Rule opens the care-rules library).
	ProtocolName string
	ProtocolHref string
	ProtocolRule bool
}

// animalTodayPlan builds today's plan (TodayPlanWindow) — the single
// engine pass behind the Treatment tab's Today block AND the ?item=
// deep-link resolution (§8.2/§10 share one assembly).
func animalTodayPlan(tx *pop.Connection) (*DayPlan, error) {
	now := time.Now()
	from, to := TodayPlanWindow(now)
	return BuildDayPlan(tx, now, from, to)
}

// R3-6 treatment-tab window: 14 days of history, 5 days ahead — the
// forward cap bounds open-ended protocols (no end date) so the engine
// never generates an unbounded occurrence list.
const (
	treatmentWindowPastDays   = 14
	treatmentWindowFutureDays = 5
)

// TreatmentPlanWindow returns the animal Treatment tab window (R3-6):
// [now-14d 00:00, now+5d 24:00). The +5d forward cap bounds open-ended
// protocols; history shows the last two weeks of applied/missed work.
func TreatmentPlanWindow(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -treatmentWindowPastDays)
	end := start.AddDate(0, 0, treatmentWindowPastDays+treatmentWindowFutureDays).
		Add(24*time.Hour - time.Nanosecond)
	return start, end
}

// animalTreatmentPlan builds the R3-6 engine plan behind the animal
// Treatment tab (history + future, capped forward).
func animalTreatmentPlan(tx *pop.Connection) (*DayPlan, error) {
	now := time.Now()
	from, to := TreatmentPlanWindow(now)
	return BuildDayPlan(tx, now, from, to)
}

// AnimalTreatmentDay is ONE date group of the R3-6 Treatment tab: the
// animal's medication series for that calendar day, newest day first.
// Group carries the same series in the MedGroupView shape the shared
// `_med_series` partial expects (its `mg` context) — one group per day
// keeps the partial reusable unchanged.
type AnimalTreatmentDay struct {
	Date      time.Time
	DateKey   string // "2006-01-02" (collapse anchor id)
	Current   bool   // today
	Future    bool
	Group     MedGroupView
	OpenCount int // open (not done/overridden) slots of the day
}

// animalTreatmentDays folds the engine plan into per-day medication
// series for ONE animal (R3-6): every occurrence of the window lands in
// its calendar day, grouped into the shared `_med_series` series so the
// hour buttons stay togglable via `_plan_med_toggle`. Overridden
// occurrences stay out; days are returned newest-first, series in slot
// order. Days with no occurrence are omitted.
func animalTreatmentDays(plan *DayPlan, animal *models.Animal) []AnimalTreatmentDay {
	if plan == nil {
		return nil
	}
	// Per-day slot buckets keyed by date; series merged within the day.
	byDay := map[string][]MedSlotView{}
	dayTime := map[string]time.Time{}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animal.ID ||
			src.ActionKind() != careplan.KindMedication ||
			it.Status == careplan.StatusOverridden {
			continue
		}
		due := it.Occurrence.DueAt
		key := due.Format("2006-01-02")
		slot := MedSlotView{
			Slot:       medSlotOf(due),
			Detail:     planDetail(src),
			SourceName: DisplayName(src.Name()),
			SourceType: string(src.SourceType()),
			SourceID:   src.SourceID(),
			DueAt:      due,
			DueAtRFC:   due.Format("2006-01-02T15:04:05Z07:00"),
			DueAtHM:    due.Format("15:04"),
			Status:     string(it.Status),
			Applied:    it.Status == careplan.StatusApplied,
			Applicable: it.Applicable,
			Overridden: false,
			SourceLink: cardSourceLink(string(src.SourceType()), src.SourceID(), animal.ID, ""),
		}
		slot.Done = slot.Applied || it.Status == careplan.StatusSkipped || it.Status == careplan.StatusDeferred
		slot.LateAllowed = slotLateAllowed(plan.Now, slot, it)
		if app := it.Application; app != nil {
			slot.CanUndo = slot.Applied && app.FulfillmentType == models.ApplicationFulfillmentTreatment &&
				app.FulfillmentID != "" && app.FulfillmentID != planFulfillmentNone && !app.FulfillmentDeleted
			slot.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted, "")
		}
		slot.ViewLink = slot.FulfillmentLink
		if slot.ViewLink == "" {
			slot.ViewLink = animalTreatmentLink(it.Occurrence.AnimalID, "")
		}
		slot.DeepLink = animalItemDeepLink(it.Occurrence.AnimalID, string(src.SourceType()), src.SourceID(), slot.DueAtRFC)
		byDay[key] = append(byDay[key], slot)
		if _, ok := dayTime[key]; !ok {
			dayTime[key] = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
		}
	}
	if len(byDay) == 0 {
		return nil
	}
	keys := make([]string, 0, len(byDay))
	for k := range byDay {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys))) // newest day first
	today := plan.Now.Format("2006-01-02")
	bucketOrder := map[string]int{"morning": 0, "noon": 1, "evening": 2}
	out := make([]AnimalTreatmentDay, 0, len(keys))
	for _, k := range keys {
		slots := byDay[k]
		sort.SliceStable(slots, func(i, j int) bool {
			if slots[i].DueAt.Equal(slots[j].DueAt) {
				return bucketOrder[slots[i].Slot] < bucketOrder[slots[j].Slot]
			}
			return slots[i].DueAt.Before(slots[j].DueAt)
		})
		series := seriesOf(slots, bucketOrder)
		d := AnimalTreatmentDay{
			Date:    dayTime[k],
			DateKey: k,
			Current: k == today,
			Future:  k > today,
			Group: MedGroupView{
				AnimalID:    animal.ID,
				AnimalLabel: animalLabel(*animal),
				AnimalLink:  animalTreatmentLink(animal.ID, ""),
				Series:      series,
			},
		}
		for _, s := range slots {
			if !s.Done && !s.Overridden {
				d.OpenCount++
			}
		}
		out = append(out, d)
	}
	return out
}

// animalPlanTodayItem pairs an accordion row with its due time (sort
// key), its source reference (protocol backlink) and its dedupe key
// ("" = never deduped against legacy treatments).
type animalPlanTodayItem struct {
	due    time.Time
	srcID  string
	rule   bool
	ddedup string
	row    AnimalPlanTodayRow
}

// animalPlanLegacyDrugs indexes the drug labels of the animal's legacy
// treatments dated inside the plan window (today) — the dedupe keys of
// the plan rows (bugs.md U26).
func animalPlanLegacyDrugs(plan *DayPlan, animal *models.Animal) map[string]bool {
	legacy := map[string]bool{}
	for i := range animal.Treatments {
		t := &animal.Treatments[i]
		if t.Date.Before(plan.From) || t.Date.After(plan.To) {
			continue
		}
		legacy[normalizeWorkLabel(t.Drug)] = true
	}
	return legacy
}

// animalPlanTodayKind reports whether the action kind shows on the
// Treatment tab (medication, observation, care — feeding/cleanup/weighing
// belong to the care plan screen).
func animalPlanTodayKind(kind string) bool {
	return kind == careplan.KindMedication || kind == careplan.KindObservation ||
		kind == careplan.KindCare
}

// animalPlanTodayLabel renders the content line of one occurrence:
// medication "Drug (dosage)", observation prompt, care note (falling
// back to instructions). Returns the dedupe key ("" = never deduped
// against legacy treatments — care rows have no drug/prompt identity).
func animalPlanTodayLabel(kind string, src careplan.PlanSource) (label, dedupe string) {
	p := parsePlanPayload(src)
	switch kind {
	case careplan.KindMedication:
		if p.Dosage == "" {
			return p.Drug, p.Drug
		}
		return p.Drug + " (" + p.Dosage + ")", p.Drug
	case careplan.KindObservation:
		return p.Prompt, p.Prompt
	default: // care
		if p.Note != "" {
			return p.Note, ""
		}
		return p.Instructions, ""
	}
}

// animalPlanTodayStatus folds the occurrence status into the accordion
// badge kind: "done", "skipped" (skipped or deferred) or "pending";
// open occurrences due earlier today are "missed". appliedAt is the
// "15:04" apply time of done rows.
func animalPlanTodayStatus(it *careplan.PlanItem, now time.Time) (status, appliedAt string) {
	switch it.Status {
	case careplan.StatusApplied:
		if it.Application != nil {
			return "done", it.Application.AppliedAt.Format("15:04")
		}
		return "done", ""
	case careplan.StatusSkipped, careplan.StatusDeferred:
		return "skipped", ""
	default:
		if it.Occurrence.DueAt.Before(now) {
			return "missed", ""
		}
		return "pending", ""
	}
}

// collectAnimalPlanTodayItems projects the today-window occurrences of
// ONE animal into accordion items: overridden occurrences stay out, and
// occurrences whose drug/prompt already exists as a legacy treatment of
// the same day are deduped away — the legacy row keeps showing that work
// (new must not stack on top of old, bugs.md U26). Also returns the
// distinct source ids per planning level for the protocol backlinks.
func collectAnimalPlanTodayItems(plan *DayPlan, animal *models.Animal, legacy map[string]bool) (items []animalPlanTodayItem, planIDs, ruleIDs []uuid.UUID) {
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animal.ID ||
			it.Status == careplan.StatusOverridden ||
			!animalPlanTodayKind(src.ActionKind()) {
			continue
		}
		if it.Occurrence.DueAt.Before(plan.From) || it.Occurrence.DueAt.After(plan.To) {
			continue
		}
		entry := animalPlanTodayItem{due: it.Occurrence.DueAt, srcID: src.SourceID()}
		entry.row.DueHM = it.Occurrence.DueAt.Format("15:04")
		entry.row.Label, entry.ddedup = animalPlanTodayLabel(src.ActionKind(), src)
		if entry.row.Label == "" {
			entry.row.Label = DisplayName(src.Name())
		}
		entry.row.Status, entry.row.AppliedAt = animalPlanTodayStatus(it, plan.Now)
		if entry.ddedup != "" && legacy[normalizeWorkLabel(entry.ddedup)] {
			continue
		}
		if src.SourceType() == careplan.SourceAnimal {
			planIDs = append(planIDs, uuid.FromStringOrNil(src.SourceID()))
		} else {
			entry.rule = true
			ruleIDs = append(ruleIDs, uuid.FromStringOrNil(src.SourceID()))
		}
		items = append(items, entry)
	}
	return items, planIDs, ruleIDs
}

// animalPlanTodayRows folds today's plan occurrences of ONE animal
// (medication + observation + care, TodayPlanWindow) into Treatment-tab
// accordion rows, sorted by due time, with the protocol backlink of
// their source (animal plans anchor #nav-plan on this page, rules open
// the care-rules library — same convention as treatmentProtocolLinks).
func animalPlanTodayRows(tx *pop.Connection, plan *DayPlan, animal *models.Animal) ([]AnimalPlanTodayRow, error) {
	if plan == nil {
		return nil, nil
	}
	items, planIDs, ruleIDs := collectAnimalPlanTodayItems(plan, animal, animalPlanLegacyDrugs(plan, animal))
	if len(items) == 0 {
		return nil, nil
	}
	planNames, err := planNamesByIDs(tx, planIDs)
	if err != nil {
		return nil, err
	}
	ruleNames, err := ruleNamesByIDs(tx, ruleIDs)
	if err != nil {
		return nil, err
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].due.Before(items[j].due) })
	rows := make([]AnimalPlanTodayRow, 0, len(items))
	for _, entry := range items {
		if entry.rule {
			entry.row.ProtocolRule = true
			entry.row.ProtocolHref = "/care_rules"
			entry.row.ProtocolName = ruleNames[entry.srcID]
		} else {
			entry.row.ProtocolHref = fmt.Sprintf("/animals/%d#nav-plan", animal.ID)
			entry.row.ProtocolName = planNames[entry.srcID]
		}
		rows = append(rows, entry.row)
	}
	return rows, nil
}

// normalizeWorkLabel folds a drug/prompt label for the plan-vs-legacy
// dedupe (case/whitespace-insensitive compare).
func normalizeWorkLabel(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
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
