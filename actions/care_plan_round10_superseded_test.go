package actions

// B10-6 — the treatment/protocol duplication. The startup converter turned
// future treatment series into care-animal plans but left the source rows
// live: the Treatment tab rendered the same requirement twice and the legacy
// copy still read as active work. The fix marks today+future treatments
// covered by an active CONVERTER-MADE plan as superseded (display only),
// keeps them out of the today-card dedupe (the PLAN row is the actionable
// one), and carries the legacy dosage into the converted prompt.

import (
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestB10_6TreatmentSupersededByPlan: today+future treatments whose drug
// matches a converter-made plan core (observation prompt / medication drug)
// are marked; past treatments, caretaker plans and inactive plans are not.
func TestB10_6TreatmentSupersededByPlan(t *testing.T) {
	today := time.Date(2026, 10, 6, 0, 0, 0, 0, time.Local)
	authorID := uuid.Must(uuid.NewV4())
	// A minimal valid schedule: careAnimalPlanSource parses it to project
	// the plan onto the engine source type.
	sched := []byte(`{"times":["08:00"],"every_days":1,"anchor":"fixed","anchor_date":"2026-10-06"}`)

	plans := models.CareAnimalPlans{
		// Converter-made observation plan for the drug (created_by NULL).
		{
			ID:            uuid.Must(uuid.NewV4()),
			ActionKind:    "observation",
			ActionPayload: []byte(`{"prompt":"Nettoyage Fistule"}`),
			Schedule:      sched,
			Active:        true,
		},
		// Converter-made medication plan for another drug.
		{
			ID:            uuid.Must(uuid.NewV4()),
			ActionKind:    "medication",
			ActionPayload: []byte(`{"drug":"Citramox L.A.","dosage":"48H"}`),
			Schedule:      sched,
			Active:        true,
		},
		// Caretaker-authored plan with the same core: NEVER auto-dedupes.
		{
			ID:            uuid.Must(uuid.NewV4()),
			ActionKind:    "observation",
			ActionPayload: []byte(`{"prompt":"Soins plaie"}`),
			Schedule:      sched,
			Active:        true,
			CreatedBy:     nulls.UUID{UUID: authorID, Valid: true},
		},
		// Inactive converter plan: no.
		{
			ID:            uuid.Must(uuid.NewV4()),
			ActionKind:    "observation",
			ActionPayload: []byte(`{"prompt":"Vieux protocole"}`),
			Schedule:      sched,
			Active:        false,
		},
	}

	animal := &models.Animal{ID: 1, Treatments: models.Treatments{
		{ID: uuid.Must(uuid.NewV4()), Date: today.AddDate(0, 0, -3), Drug: "Nettoyage Fistule"}, // past: history, untouched
		{ID: uuid.Must(uuid.NewV4()), Date: today, Drug: "nettoyage fistule "},                  // today, case/space-insensitive core
		{ID: uuid.Must(uuid.NewV4()), Date: today.AddDate(0, 0, 4), Drug: "Nettoyage Fistule"},  // future
		{ID: uuid.Must(uuid.NewV4()), Date: today, Drug: "citramox l.a."},                       // medication core
		{ID: uuid.Must(uuid.NewV4()), Date: today, Drug: "Soins plaie"},                         // caretaker core: not marked
		{ID: uuid.Must(uuid.NewV4()), Date: today, Drug: "Vieux protocole"},                     // inactive plan: not marked
		{ID: uuid.Must(uuid.NewV4()), Date: today, Drug: "Autre chose"},                         // no plan at all
	}}

	superseded := treatmentSupersededByPlan(plans, animal, today)
	require.Len(t, superseded, 3, "today + future Nettoyage Fistule and today Citramox marked")

	marked := func(i int) bool { return superseded[animal.Treatments[i].ID.String()] }
	require.False(t, marked(0), "past treatments are history, never hidden")
	require.True(t, marked(1))
	require.True(t, marked(2))
	require.True(t, marked(3))
	require.False(t, marked(4), "caretaker-authored plans never auto-dedupe")
	require.False(t, marked(5), "inactive plans never supersede")
	require.False(t, marked(6))
}

// TestB10_6LegacyDedupeComposite: the today-card dedupe set skips
// superseded treatments and indexes "drug (dosage)" composites, so the
// converted plan row (prompt carrying the dosage) still dedupes against a
// NOT-superseded legacy row, and a superseded one lets the plan row render.
func TestB10_6LegacyDedupeComposite(t *testing.T) {
	now := time.Date(2026, 10, 6, 10, 0, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now

	animal := &models.Animal{ID: 1, Treatments: models.Treatments{
		{ID: uuid.Must(uuid.NewV4()), Date: from, Drug: "Nettoyage Fistule", Dosage: "Dessus oeil droit"},
	}}

	// Superseded: excluded from the dedupe set entirely.
	legacy := animalPlanLegacyDrugs(plan, animal, map[string]bool{animal.Treatments[0].ID.String(): true})
	require.Empty(t, legacy, "a superseded treatment is no dedupe key — the plan row renders")

	// Not superseded: the drug AND the "drug (dosage)" composite are keys.
	legacy = animalPlanLegacyDrugs(plan, animal, nil)
	require.True(t, legacy["nettoyage fistule"])
	require.True(t, legacy["nettoyage fistule (dessus oeil droit)"],
		"the composite key matches the enriched converted prompt")
	require.False(t, legacy["nettoyage fistule (dessus oeil droit) "], "keys are normalized")
}
