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
	// AppliedAt is an RFC3339 timestamp for client-local display (zero otherwise).
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
// keeps the partial reusable unchanged. R3-7: Items adds the non-
// medication occurrences of the day (observation/care/weighing — compact
// read-only rows) so the Protocol tab can embed the animal's complete
// care plan from the SAME per-day groups; the Treatment tab ignores
// Items and keeps its medication-only focus.
type AnimalTreatmentDay struct {
	Date      time.Time
	DateKey   string // "2006-01-02" (collapse anchor id)
	Current   bool   // today
	Future    bool
	Group     MedGroupView
	Items     []CardView // R3-7: non-medication occurrences (kind-ordered)
	OpenCount int        // open (not done/overridden) slots + open non-medication items of the day
	// MedOpenCount counts ONLY the open medication slots — what the Treatment
	// tab actually renders. R4-7.23: that tab showed OpenCount over a body
	// that carries the medication series alone, so a day whose work is all
	// feeding/care/observation advertised "3" above an EMPTY card (measured
	// on animals/10312: 7 day cards, badge 3, zero rows). The Protocol tab
	// renders both halves and keeps OpenCount.
	MedOpenCount int
	// MissingCount counts the occurrences of the day already past due
	// with nothing recorded (status missing) INSIDE the last 24 h: those
	// rows are no longer actionable, so they leave the list and the
	// caregiver gets ONE pill with their number on the day's last row
	// (R4-7). Older misses stay visible as history; the count stops at
	// missingCountCap — past that the exact number stops being information.
	MissingCount int
}

// missingCountCap bounds the "N missed" pill.
const missingCountCap = 5

// missingWindow bounds which misses the pill summarises: the last 24 h.
// Anything older is history, not "what did I just miss".
const missingWindow = 24 * time.Hour

// dayItemsWithoutMisses splits the day's compact rows: a MISSED
// occurrence (past due, nothing recorded) inside the last 24 h is not
// actionable any more — the apply window has closed — so it leaves the
// list and only raises MissingCount, which the template renders as ONE
// pill on the last remaining row. Every other row (including older
// misses, kept as history) stays.
func dayItemsWithoutMisses(items []CardView, now time.Time, day *AnimalTreatmentDay) []CardView {
	cutoff := now.Add(-missingWindow)
	out := make([]CardView, 0, len(items))
	for _, it := range items {
		if it.Status == string(careplan.StatusMissing) && !it.DueAt.Before(cutoff) && !it.DueAt.After(now) {
			if day.MissingCount < missingCountCap {
				day.MissingCount++
			}
			continue
		}
		out = append(out, it)
	}
	return out
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
	byDayItems := map[string][]CardView{} // R3-7: non-medication occurrences
	dayTime := map[string]time.Time{}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil || it.Occurrence.AnimalID != animal.ID ||
			it.Status == careplan.StatusOverridden {
			continue
		}
		due := it.Occurrence.DueAt
		key := due.Format("2006-01-02")
		if _, ok := dayTime[key]; !ok {
			dayTime[key] = time.Date(due.Year(), due.Month(), due.Day(), 0, 0, 0, 0, due.Location())
		}
		// R3-7: the Protocol tab embeds the animal's COMPLETE care plan —
		// every non-medication occurrence (observation/care/weighing AND
		// the cage-grouped feeding/cleanup — a feeding protocol with no
		// line here would look like "no entries") rides along as a
		// compact CardView row (read-only on the animal page).
		if src.ActionKind() != careplan.KindMedication {
			byDayItems[key] = append(byDayItems[key], animalDayCardFor(plan, it, animal))
			continue
		}
		byDay[key] = append(byDay[key], animalDayMedSlot(plan, it, animal))
	}
	if len(byDay) == 0 && len(byDayItems) == 0 {
		return nil
	}
	keys := make([]string, 0, len(dayTime))
	for k := range dayTime {
		keys = append(keys, k)
	}
	sort.Sort(sort.Reverse(sort.StringSlice(keys))) // newest day first
	today := plan.Now.Format("2006-01-02")
	bucketOrder := map[string]int{"morning": 0, "noon": 1, "evening": 2}
	out := make([]AnimalTreatmentDay, 0, len(keys))
	for _, k := range keys {
		out = append(out, animalTreatmentDayFor(k, today, plan.Now, dayTime[k], byDay[k], byDayItems[k], animal, bucketOrder))
	}
	return out
}

// medicationOnlyDays returns the days of a treatment-day list that actually
// carry medication work — the ones the Treatment tab can render. A day whose
// occurrences are all feeding/care/observation produces no series, and a card
// whose body is the series alone would then be an empty box under a count
// (R4-7.23, measured on animals/10312: 7 such cards, each badged "3", each
// with nothing in it). Those days are NOT dropped from the data: the Protocol
// tab renders them, since it lists the non-medication items too.
func medicationOnlyDays(days []AnimalTreatmentDay) []AnimalTreatmentDay {
	out := make([]AnimalTreatmentDay, 0, len(days))
	for _, d := range days {
		if len(d.Group.Series) > 0 {
			out = append(out, d)
		}
	}
	return out
}

// animalTreatmentDayFor assembles one calendar day: medication slots
// sorted by due time then slot bucket, merged into the shared series;
// the non-medication compact items ride along; OpenCount covers both
// open slots and open items so the day badge reflects ALL remaining work.
func animalTreatmentDayFor(key, today string, now, date time.Time, slots []MedSlotView, items []CardView, animal *models.Animal, bucketOrder map[string]int) AnimalTreatmentDay {
	sort.SliceStable(slots, func(i, j int) bool {
		if slots[i].DueAt.Equal(slots[j].DueAt) {
			return bucketOrder[slots[i].Slot] < bucketOrder[slots[j].Slot]
		}
		return slots[i].DueAt.Before(slots[j].DueAt)
	})
	d := AnimalTreatmentDay{
		Date:    date,
		DateKey: key,
		Current: key == today,
		Future:  key > today,
		Group: MedGroupView{
			AnimalID:    animal.ID,
			AnimalLabel: animalLabel(*animal),
			AnimalLink:  animalTreatmentLink(animal.ID, ""),
			Series:      seriesOf(slots, bucketOrder),
		},
		Items: items,
	}
	// R4-7: recent misses leave the list (they can no longer be applied);
	// their number rides on the day's last row as one pill.
	d.Items = dayItemsWithoutMisses(d.Items, now, &d)
	// Phase 5 / D5 (§3.1): repeats of the same requirement merge into ONE
	// line with one tier-coloured toggle per open occurrence.
	d.Items = mergeDayItems(d.Items, now)
	// B10-5: the day's entries render kind-GROUPED under .plan-kind-separator
	// titles — a stable kind sort keeps each group contiguous (the template
	// emits one separator per kind change). Bugs.md second batch #8: the sort
	// follows the SAME importance rank as the day-plan tabs (medication leads
	// via the series block; then care → feeding → observation → weighing →
	// cleanup), not alphabetical order.
	sort.SliceStable(d.Items, func(i, j int) bool {
		return actionKindRank(d.Items[i].ActionKind) < actionKindRank(d.Items[j].ActionKind)
	})
	for _, s := range slots {
		if !s.Done && !s.Overridden {
			d.OpenCount++
			d.MedOpenCount++
		}
	}
	// §1.4 (one unit, one number): the day badge counts open OCCURRENCES —
	// every slot of a merged line, plus every standalone open row.
	for _, item := range d.Items {
		if len(item.Slots) > 0 {
			for _, s := range item.Slots {
				if openStatusAction(careplan.PlanStatus(s.Status)) {
					d.OpenCount++
				}
			}
			continue
		}
		if openStatusAction(careplan.PlanStatus(item.Status)) {
			d.OpenCount++
		}
	}
	return d
}

// animalDayMedSlot projects one medication occurrence of the animal into
// the togglable MedSlotView the shared `_med_series` partial renders —
// the shared medSlotFor with the animal page as its own return path
// (overridden occurrences never reach it: filtered upstream).
func animalDayMedSlot(plan *DayPlan, it *careplan.PlanItem, animal *models.Animal) MedSlotView {
	slot := medSlotFor(plan.Now, it, "")
	slot.Overridden = false // filtered upstream — never rendered as such
	return slot
}

// animalDayCardFor projects one NON-medication occurrence of the animal
// into the CardView the shared `_plan_item_line` partial renders on the
// Protocol tab (Phase 5 / D5): label + kind badge + source backlink +
// per-occurrence tier-coloured toggle (open occurrences merge per source
// into ONE line by mergeDayItems; terminal rows keep their per-occurrence
// fulfillment link / state badge).
func animalDayCardFor(plan *DayPlan, it *careplan.PlanItem, animal *models.Animal) CardView {
	src := it.Occurrence.Source
	parts := DueLabelPartsOf(it.Occurrence.DueAt, plan.Now)
	cv := CardView{
		SourceType:   string(src.SourceType()),
		SourceID:     src.SourceID(),
		SourceName:   DisplayName(src.Name()),
		Detail:       planDetail(src),
		ActionKind:   src.ActionKind(),
		AnimalID:     animal.ID,
		AnimalLabel:  animalLabel(*animal),
		AnimalYear:   animal.YearNumberFormatted(),
		DueAt:        it.Occurrence.DueAt,
		DueHM:        parts.TimeHM,
		DueDayKey:    parts.DayKey,
		DueShortDate: parts.ShortDate,
		Status:       string(it.Status),
		Tier:         tierOrder(it.Status),
		TierClass:    slotTierClass(tierOrder(it.Status)),
		Applicable:   it.Applicable,
		NeedsInput:   src.ActionKind() == careplan.KindWeighing || src.ActionKind() == careplan.KindObservation,
		LateAllowed:  it.Applicable && it.Occurrence.DueAt.Before(plan.Now),
		Undoable:     it.Application != nil,
		SourceLink:   cardSourceLink(string(src.SourceType()), src.SourceID(), animal.ID, ""),
	}
	// Phase 5 / D5 (§4.3): the animal's own page omits the animal CELL, but
	// the ℹ detail modal still names the animal as plain text (no link —
	// identity is the page context; the show template hides the anchor).
	cv.AnimalLink = fmt.Sprintf("/animals/%d", animal.ID)
	if app := it.Application; app != nil {
		cv.RecordedLate = app.AppliedAt.After(it.Occurrence.DueAt)
		// R4-7.11b: the row's record of what was actually done. `back` is
		// THIS page's Plan tab, so the record's own back button returns
		// here instead of stranding the caregiver on a dead-end list.
		cv.FulfillmentLink = cardFulfillmentLink(app.FulfillmentType, app.FulfillmentID, app.FulfillmentDeleted,
			fmt.Sprintf("/animals/%d#nav-plan", animal.ID))
	}
	// Rev: feeding source names embed the diet ("Alimentation — <diet>"),
	// so beside the Detail the name reads as the same sentence twice, cut
	// off mid-word. When the name adds nothing the Detail doesn't already
	// say, the row renders kind + nourriture/régime only (the protocol
	// stays reachable from the Details table above).
	cv.SourceNameRedundant = sourceNameRedundantWithDetail(src, cv.SourceName, cv.Detail)
	return cv
}

// sourceNameRedundantWithDetail reports whether a row's source name is just
// the detail text again (possibly truncated): the name core (kind prefix,
// conversion markers and the "…" cut mark stripped) is a prefix of the Detail
// or vice versa. R8-2: applies to feeding ("Alimentation — <diet>") AND the
// title-shaped kinds observation/care/weighing ("Traitement — Sexage" beside
// detail "Sexage" reads the title twice); medication keeps its drug + dosage
// name (real information beyond the prompt).
func sourceNameRedundantWithDetail(src careplan.PlanSource, name, detail string) bool {
	if src == nil {
		return false
	}
	switch src.ActionKind() {
	case careplan.KindFeeding, careplan.KindObservation, careplan.KindCare, careplan.KindWeighing:
	default:
		return false
	}
	detail = strings.TrimSpace(detail)
	if detail == "" {
		return false
	}
	core := strings.TrimSpace(DisplayName(name))
	if core == "" {
		return true // nothing but the kind prefix — the Detail is the content
	}
	// "Alimentation — <diet>": the text after the FIRST dash is the diet the
	// name repeats. (A diet itself can contain dashes, so first only.)
	if i := strings.Index(core, "—"); i >= 0 {
		core = strings.TrimSpace(core[i+len("—"):])
	}
	// truncateWords marks its cut with "…"; the detail continues where the
	// name stopped.
	core = strings.TrimSuffix(core, "…")
	core = strings.TrimSpace(core)
	if core == "" {
		return true
	}
	return strings.HasPrefix(detail, core) || strings.HasPrefix(core, detail) || strings.Contains(detail, core)
}

// mergeDayItems folds a day's non-medication rows per (source × animal)
// group (Phase 5 / D5, guideline §3.1 — the same merge the day plan's
// tierRows applies, here fed from the already-projected CardViews):
// every OPEN occurrence of the group becomes one tier-coloured toggle
// (ItemSlotView) on ONE representative line, due-time ordered; the line's
// own tier/colour/due label is its most urgent slot's (late beats now
// beats later, earliest due wins on a tie). Terminal occurrences
// (applied/skipped/deferred) and non-applicable misses kept as history
// stay SINGLE-occurrence lines — their fulfillment link / record state
// is per occurrence (R4-7.11b). Rows are keyed by source across the
// WHOLE day list, not adjacent runs; survivors keep their original
// relative order (the representative sits at its first open occurrence).
func mergeDayItems(items []CardView, now time.Time) []CardView {
	type srcKey struct{ typ, id string }
	reps := map[srcKey]int{} // source → index in out of the representative row
	out := make([]CardView, 0, len(items))
	for i := range items {
		it := items[i]
		if !openStatusAction(careplan.PlanStatus(it.Status)) {
			out = append(out, it) // terminal / locked history row — stands alone
			continue
		}
		k := srcKey{it.SourceType, it.SourceID}
		slot := ItemSlotView{
			DueAt:        it.DueAt,
			DueAtRFC:     it.DueAt.Format("2006-01-02T15:04:05Z07:00"),
			DueHM:        it.DueHM,
			DueDayKey:    it.DueDayKey,
			DueShortDate: it.DueShortDate,
			AnimalID:     it.AnimalID,
			Status:       it.Status,
			Tier:         it.Tier,
			TierClass:    it.TierClass,
			Applicable:   it.Applicable,
			LateAllowed:  it.Applicable && it.DueAt.Before(now),
			NeedsInput:   it.ActionKind == careplan.KindWeighing || it.ActionKind == careplan.KindObservation,
		}
		j, seen := reps[k]
		if !seen {
			it.Slots = []ItemSlotView{slot}
			reps[k] = len(out)
			out = append(out, it)
			continue
		}
		rep := &out[j]
		rep.Slots = append(rep.Slots, slot)
		// The line's own tier/colour/due label is its MOST URGENT slot's
		// (the work the caregiver must see first); on a tie the earliest
		// due time wins.
		if slot.Tier < rep.Tier || (slot.Tier == rep.Tier && slot.DueAt.Before(rep.DueAt)) {
			rep.Tier, rep.TierClass = slot.Tier, slot.TierClass
			rep.DueAt, rep.DueHM = slot.DueAt, slot.DueHM
			rep.DueDayKey, rep.DueShortDate = slot.DueDayKey, slot.DueShortDate
			rep.Status = slot.Status
		}
	}
	// Due-time order each line's slots and count the fold ("+N" badge).
	for _, j := range reps {
		rep := &out[j]
		sort.SliceStable(rep.Slots, func(a, b int) bool {
			return rep.Slots[a].DueAt.Before(rep.Slots[b].DueAt)
		})
		rep.Remaining = len(rep.Slots) - 1
		rep.RemainingCap = BadgeCap(rep.Remaining)
		// A merged line is actionable when ANY of its slots is.
		rep.Applicable = false
		for _, s := range rep.Slots {
			if s.Applicable {
				rep.Applicable = true
				break
			}
		}
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
// the plan rows (bugs.md U26). B10-6: treatments the protocol already
// covers (superseded) are NOT dedupe keys — the plan row is the
// actionable one; and each label also indexes its "drug (dosage)"
// composite so a converted plan whose prompt carries the dosage still
// dedupes against the legacy row it came from.
func animalPlanLegacyDrugs(plan *DayPlan, animal *models.Animal, superseded map[string]bool) map[string]bool {
	legacy := map[string]bool{}
	for i := range animal.Treatments {
		t := &animal.Treatments[i]
		if t.Date.Before(plan.From) || t.Date.After(plan.To) {
			continue
		}
		if superseded[t.ID.String()] {
			continue
		}
		legacy[normalizeWorkLabel(t.Drug)] = true
		if strings.TrimSpace(t.Dosage) != "" {
			legacy[normalizeWorkLabel(t.Drug+" ("+t.Dosage+")")] = true
		}
	}
	return legacy
}

// treatmentSupersededByPlan marks the animal's legacy treatments (TODAY and
// FUTURE) already covered by an active CONVERTER-MADE plan (B10-6): the
// startup converter turned future treatment series into care-animal plans
// but left the source rows live, so the Treatment tab showed the same
// requirement twice and the legacy copy still read as active work.
// Matching is by normalized content core — the converter's plans carry the
// legacy drug line in the payload per kind (medication drug, observation
// prompt, care note — B10-8), matched against the treatment's bare drug OR
// its "drug (dosage)" composite. Past treatments stay untouched (they are
// history, never hidden), and caretaker-authored plans never auto-dedupe
// anything.
func treatmentSupersededByPlan(plans models.CareAnimalPlans, animal *models.Animal, today time.Time) map[string]bool {
	superseded := map[string]bool{}
	cores := convertedPlanCores(plans)
	if len(cores) == 0 {
		return superseded
	}
	for i := range animal.Treatments {
		t := &animal.Treatments[i]
		if t.Date.Before(today) {
			continue
		}
		// The converted plan core may carry the application site — the
		// converted content line after the dosage enrichment is "Drug
		// (site)" while the treatment stores the site in the dosage
		// column. Match the bare drug OR the "drug (dosage)" composite
		// (same keys the today-card dedupe indexes).
		if cores[normalizeWorkLabel(t.Drug)] ||
			(strings.TrimSpace(t.Dosage) != "" &&
				cores[normalizeWorkLabel(t.Drug+" ("+t.Dosage+")")]) {
			superseded[t.ID.String()] = true
		}
	}
	return superseded
}

// convertedPlanCores collects the identifying content core of every ACTIVE
// converter-made plan (B10-6), per kind: medication drug, observation
// prompt, care note (falling back to instructions — B10-8 converted cares
// carry the legacy drug line there).
func convertedPlanCores(plans models.CareAnimalPlans) map[string]bool {
	cores := map[string]bool{}
	for i := range plans {
		p := &plans[i]
		if !p.Active || p.CreatedBy.Valid {
			continue
		}
		src, err := careAnimalPlanSource(p)
		if err != nil {
			continue
		}
		payload := parsePlanPayload(src)
		core := payload.Drug
		switch p.ActionKind {
		case careplan.KindObservation:
			core = payload.Prompt
		case careplan.KindCare:
			core = payload.Note
			if strings.TrimSpace(core) == "" {
				core = payload.Instructions
			}
		}
		if core = normalizeWorkLabel(core); core != "" {
			cores[core] = true
		}
	}
	return cores
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
// open occurrences due earlier today are "missed". appliedAt is an RFC3339
// timestamp for client-local display on the browser.
func animalPlanTodayStatus(it *careplan.PlanItem, now time.Time) (status, appliedAt string) {
	switch it.Status {
	case careplan.StatusApplied:
		if it.Application != nil {
			return "done", it.Application.AppliedAt.Format(time.RFC3339)
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
// superseded marks the treatments the protocol already covers (B10-6) —
// they stay out of the dedupe set so the PLAN row renders on the today
// card (the legacy row shows superseded instead).
func animalPlanTodayRows(tx *pop.Connection, plan *DayPlan, animal *models.Animal, superseded map[string]bool) ([]AnimalPlanTodayRow, error) {
	if plan == nil {
		return nil, nil
	}
	items, planIDs, ruleIDs := collectAnimalPlanTodayItems(plan, animal, animalPlanLegacyDrugs(plan, animal, superseded))
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
