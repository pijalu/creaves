package actions

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Care plan service (§6 of docs/care-expert.md): bridges the DB-free
// models/careplan engine and the live database. It gathers the planning
// sources (rules + animal plans), resolves rule membership through the
// matcher DSL (§5), generates occurrences over a window (§6.1), joins the
// stored applications and hands pure PlanItems to the handlers.
//
// Fulfillment writers (§6.2, care_plan_fulfillment.go) create the real
// cares/treatments rows per action kind plus the linking
// care_plan_applications row (idempotent via the UNIQUE key). Everything
// here runs on the pop connection the per-request transaction provides.

// §6.1 display window: [today 00:00 − 24h, today + 2 days], hard cap
// 14 days (query-param configurable in the handler).
const (
	planWindowPastDays   = 1
	planWindowFutureDays = 2
	planWindowMaxDays    = 14
)

// followUpPlanPrefix marks the auto-created alert follow-up observation
// plans (§6.2, §10-CP3): such plans self-deactivate once their occurrence
// is applied or skipped.
const followUpPlanPrefix = "Vérifier alerte: "

// defaultFollowUpHours is used when an alert payload omits
// alert_follow_up_hours (validated ≥ 1 when set).
const defaultFollowUpHours = 2

// DefaultPlanWindow returns the §6.1 default display window for `now`.
func DefaultPlanWindow(now time.Time) (time.Time, time.Time) {
	start := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).
		AddDate(0, 0, -planWindowPastDays)
	end := start.AddDate(0, 0, planWindowPastDays+planWindowFutureDays).
		Add(24*time.Hour - time.Nanosecond)
	return start, end
}

// ---------------------------------------------------------------------------
// Planning sources (§6.1 step 0)
// ---------------------------------------------------------------------------

// loadPlanSources loads all active rules and animal plans as engine
// PlanSources. planAnimalsByID maps animal-plan source IDs to their single
// animal (§4.7). Rows whose stored JSON fails to parse cannot exist through
// normal validation; should they, the source is skipped — the plan view
// degrades to fewer items, never a 500.
func loadPlanSources(tx *pop.Connection) ([]careplan.PlanSource, map[string]int, error) {
	var rules []models.CareRule
	if err := tx.Where("active = ?", true).All(&rules); err != nil {
		return nil, nil, err
	}
	var plans []models.CareAnimalPlan
	if err := tx.Where("active = ?", true).All(&plans); err != nil {
		return nil, nil, err
	}
	out := make([]careplan.PlanSource, 0, len(rules)+len(plans))
	planAnimal := make(map[string]int, len(plans))
	for i := range rules {
		if src, err := careRuleSource(&rules[i]); err == nil {
			out = append(out, src)
		}
	}
	for i := range plans {
		if src, err := careAnimalPlanSource(&plans[i]); err == nil {
			out = append(out, src)
			planAnimal[src.SourceID()] = plans[i].AnimalID
		}
	}
	return out, planAnimal, nil
}

// careRuleSource projects a CareRule row onto the engine value type.
func careRuleSource(r *models.CareRule) (careplan.PlanSource, error) {
	sched, err := careplan.ParseScheduleJSON(r.Schedule)
	if err != nil {
		return nil, err
	}
	return &careplan.Source{
		Type: careplan.SourceRule, ID: r.ID.String(), NameStr: r.Name,
		Kind: r.ActionKind, PayloadJSON: r.ActionPayload, Sched: sched,
		ValidFromD: r.ValidFrom, ValidToD: r.ValidTo,
		PriorityVal: r.Priority, IsActive: r.Active,
	}, nil
}

// careAnimalPlanSource projects a CareAnimalPlan row onto the engine value
// type (full action/schedule parity with rules, §4.7).
func careAnimalPlanSource(p *models.CareAnimalPlan) (careplan.PlanSource, error) {
	sched, err := careplan.ParseScheduleJSON(p.Schedule)
	if err != nil {
		return nil, err
	}
	return &careplan.Source{
		Type: careplan.SourceAnimal, ID: p.ID.String(), NameStr: p.Name,
		Kind: p.ActionKind, PayloadJSON: p.ActionPayload, Sched: sched,
		PriorityVal: 100, IsActive: p.Active, Replaces: p.ReplacesKind,
	}, nil
}

// ---------------------------------------------------------------------------
// Animal contexts (§5.1): bulk assembly of the matcher view
// ---------------------------------------------------------------------------

// planAnimals carries the assembled matcher contexts together with the raw
// animal rows (FK resolution: animaltype/animalage/intake).
type planAnimals struct {
	ctxs map[int]*careplan.AnimalContext
	rows map[int]models.Animal
}

// loadAnimalContexts builds the enriched AnimalContext view for every
// in-care animal (§5: rules match against in-care animals). Missing
// enrichments stay empty and make predicates fail closed (§5.2).
func loadAnimalContexts(tx *pop.Connection, now time.Time) (*planAnimals, error) {
	return loadAnimalContextsScoped(tx, now, nil)
}

// loadAnimalContextsScoped is loadAnimalContexts restricted to the given
// animal ids (nil = all in-care animals). Same fill pipeline — contexts
// are identical for the scoped ids (ReverifyItem, §6.2).
func loadAnimalContextsScoped(tx *pop.Connection, now time.Time, ids []int) (*planAnimals, error) {
	var animals []models.Animal
	q := tx.Where("outtake_id IS NULL")
	if ids != nil {
		q = q.Where("id in (?)", ids)
	}
	if err := q.All(&animals); err != nil {
		return nil, err
	}
	pa := &planAnimals{
		ctxs: make(map[int]*careplan.AnimalContext, len(animals)),
		rows: make(map[int]models.Animal, len(animals)),
	}
	for i := range animals {
		a := animals[i]
		pa.rows[a.ID] = a
		pa.ctxs[a.ID] = &careplan.AnimalContext{
			ID:         a.ID,
			Species:    a.Species,
			Gender:     a.Gender.String,
			Zone:       a.Zone.String,
			Cage:       a.Cage.String,
			Feeding:    a.Feeding.String,
			ForceFeed:  a.ForceFeed,
			IntakeDate: a.IntakeDate,
			EvalTime:   now,
		}
	}
	if len(pa.ctxs) == 0 {
		return pa, nil
	}
	if err := pa.fillReferenceNames(tx); err != nil {
		return nil, err
	}
	if err := pa.fillSpeciesFields(tx); err != nil {
		return nil, err
	}
	if err := pa.fillIntakeCondition(tx); err != nil {
		return nil, err
	}
	if err := pa.fillLastWeights(tx); err != nil {
		return nil, err
	}
	if err := pa.fillVetDiagnostics(tx); err != nil {
		return nil, err
	}
	return pa, nil
}

// fillReferenceNames resolves animaltypes.name / animalages.name.
func (pa *planAnimals) fillReferenceNames(tx *pop.Connection) error {
	var types []models.Animaltype
	if err := tx.All(&types); err != nil {
		return err
	}
	typeNames := make(map[uuid.UUID]string, len(types))
	for _, t := range types {
		typeNames[t.ID] = t.Name
	}
	var ages []models.Animalage
	if err := tx.All(&ages); err != nil {
		return err
	}
	ageNames := make(map[uuid.UUID]string, len(ages))
	for _, a := range ages {
		ageNames[a.ID] = a.Name
	}
	for id, row := range pa.rows {
		ctx := pa.ctxs[id]
		ctx.AnimalType = typeNames[row.AnimaltypeID]
		ctx.AnimalAge = ageNames[row.AnimalageID]
	}
	return nil
}

// fillSpeciesFields resolves the species-table columns by name.
func (pa *planAnimals) fillSpeciesFields(tx *pop.Connection) error {
	var species []models.Species
	if err := tx.All(&species); err != nil {
		return err
	}
	byName := make(map[string]models.Species, len(species))
	for _, s := range species {
		byName[s.Species] = s
	}
	for _, ctx := range pa.ctxs {
		s, ok := byName[ctx.Species]
		if !ok {
			continue
		}
		ctx.SpeciesClass = s.Class
		ctx.SpeciesOrder = s.Order
		ctx.SpeciesFamily = s.Family
		ctx.SpeciesAGWGroup = s.AgwGroup
		ctx.SpeciesSubsideGroup = s.SubsideGroup
		ctx.SpeciesNativeStatus = s.NativeStatus
		ctx.SpeciesGame = s.Game
		ctx.SpeciesHuntable = s.Huntable
	}
	return nil
}

// fillIntakeCondition resolves the intake condition fields
// (parasites/wounds/general) through animals.intake_id — one bulk query
// over the referenced intakes (§9: no N+1).
func (pa *planAnimals) fillIntakeCondition(tx *pop.Connection) error {
	ids := make([]uuid.UUID, 0, len(pa.rows))
	seen := make(map[uuid.UUID]bool, len(pa.rows))
	for _, row := range pa.rows {
		if row.IntakeID == (uuid.UUID{}) || seen[row.IntakeID] {
			continue
		}
		seen[row.IntakeID] = true
		ids = append(ids, row.IntakeID)
	}
	if len(ids) == 0 {
		return nil
	}
	var rows []models.Intake
	if err := tx.Where("id in (?)", ids).All(&rows); err != nil {
		return err
	}
	intakes := make(map[uuid.UUID]models.Intake, len(rows))
	for _, in := range rows {
		intakes[in.ID] = in
	}
	for id, row := range pa.rows {
		in, ok := intakes[row.IntakeID]
		if !ok {
			continue
		}
		ctx := pa.ctxs[id]
		ctx.HasParasites = in.HasParasites
		ctx.Parasites = in.Parasites.String
		ctx.HasWounds = in.HasWounds
		ctx.Wounds = in.Wounds.String
		ctx.IntakeGeneral = in.General.String
		ctx.IntakeRemarks = in.Remarks.String
	}
	return nil
}

// animalIDSet returns the in-care animal ids (sorted) for IN-set queries.
func (pa *planAnimals) animalIDSet() []int {
	ids := make([]int, 0, len(pa.ctxs))
	for id := range pa.ctxs {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

// inSet renders a (?,?,…) placeholder list and the matching args for an
// IN-set raw query. Callers must no-op when ids is empty.
func inSet(ids []int) (string, []interface{}) {
	ph := make([]string, len(ids))
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return "(" + strings.Join(ph, ",") + ")", args
}

// fillLastWeights resolves the latest recorded weight (grams) per animal —
// one JOIN/MAX query over the in-care IN-set (M3: no full-history scan).
func (pa *planAnimals) fillLastWeights(tx *pop.Connection) error {
	ids := pa.animalIDSet()
	if len(ids) == 0 {
		return nil
	}
	set, args := inSet(ids)
	var rows []struct {
		AnimalID int          `db:"animal_id"`
		Weight   nulls.String `db:"weight"`
	}
	q := `SELECT c.animal_id AS animal_id, c.weight AS weight
	      FROM cares c
	      JOIN (SELECT animal_id, MAX(date) AS md FROM cares
	            WHERE animal_id IN ` + set + ` AND weight IS NOT NULL AND weight <> '' GROUP BY animal_id) x
	            ON x.animal_id = c.animal_id AND x.md = c.date
	      WHERE c.weight IS NOT NULL AND c.weight <> ''`
	if err := tx.RawQuery(q, args...).All(&rows); err != nil {
		return err
	}
	for _, r := range rows {
		ctx, ok := pa.ctxs[r.AnimalID]
		if !ok {
			continue
		}
		if g, err := strconv.ParseFloat(r.Weight.String, 64); err == nil {
			gg := g
			ctx.LastWeightG = &gg
		}
	}
	return nil
}

// fillVetDiagnostics resolves the latest veterinaryvisit diagnostic — one
// JOIN/MAX query over the in-care IN-set (M3).
func (pa *planAnimals) fillVetDiagnostics(tx *pop.Connection) error {
	ids := pa.animalIDSet()
	if len(ids) == 0 {
		return nil
	}
	set, args := inSet(ids)
	var rows []struct {
		AnimalID   int          `db:"animal_id"`
		Diagnostic nulls.String `db:"diagnostic"`
	}
	q := `SELECT v.animal_id AS animal_id, v.diagnostic AS diagnostic
	      FROM veterinaryvisits v
	      JOIN (SELECT animal_id, MAX(date) AS md FROM veterinaryvisits
	            WHERE animal_id IN ` + set + ` GROUP BY animal_id) x
	            ON x.animal_id = v.animal_id AND x.md = v.date`
	if err := tx.RawQuery(q, args...).All(&rows); err != nil {
		return err
	}
	for _, r := range rows {
		if ctx, ok := pa.ctxs[r.AnimalID]; ok {
			ctx.VetDiagnostic = r.Diagnostic.String
		}
	}
	return nil
}

// animalLabel renders the §10.5-N1 convention: `472/26 · Hérisson · A12`.
func animalLabel(a models.Animal) string {
	cage := a.Cage.String
	if cage == "" {
		cage = "—"
	}
	return fmt.Sprintf("%s · %s · %s", a.YearNumberFormatted(), a.Species, cage)
}
