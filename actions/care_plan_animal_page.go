package actions

import (
	"fmt"
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
