package actions

import (
	"encoding/json"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// ReverifyItem equivalence (bugs.md M3, Phase 5 item 5): the single-item
// recompute must yield the SAME item the full day plan produces, for the
// due / hors-délai / overridden / terminal / latched cases.
func TestReverifyItemMatchesFullPlan(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()

	// feeding rule with 3 slots today: one long past (late / hors-délai
	// once a later slot is due), one due soon, one future (scheduled).
	past := now.Add(-4 * time.Hour)
	soon := now.Add(30 * time.Minute)
	future := now.Add(4 * time.Hour)
	rule := f.feedRule(t, tx, now, past, soon, future)

	// kind-level replacing plan on animal 1 → every rule feeding
	// occurrence of animal 1 renders overridden (§4.7).
	planPayload, err := json.Marshal(map[string]interface{}{
		"caretype_id": f.feedCare.String(),
		"food":        "vers",
	})
	require.NoError(t, err)
	planSched, err := json.Marshal(map[string]interface{}{
		"times":       []string{soon.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": now.Format("2006-01-02"),
	})
	require.NoError(t, err)
	ap := &models.CareAnimalPlan{
		ID:            uuid.Must(uuid.NewV4()),
		AnimalID:      f.animalIDs[0],
		Name:          "P-" + f.marker,
		ActionKind:    "feeding",
		ActionPayload: planPayload,
		Schedule:      planSched,
		ReplacesKind:  true,
		Active:        true,
	}
	require.NoError(t, tx.Create(ap))

	plan, err := BuildDayPlan(tx, now, time.Time{}, time.Time{})
	require.NoError(t, err)

	// index full-plan items by (source, animal, slot)
	type key struct {
		srcType, srcID string
		animal         int
		due            time.Time
	}
	fullItems := map[key]*careplan.PlanItem{}
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil {
			continue
		}
		fullItems[key{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID, it.Occurrence.DueAt}] = it
	}

	assertEquiv := func(label, srcType, srcID string, animal int, due time.Time) {
		t.Helper()
		k := key{srcType, srcID, animal, due}
		full, ok := fullItems[k]
		require.True(t, ok, "%s: item missing from full plan", label)
		rev, err := ReverifyItem(tx, planItemRef{
			SourceType: srcType, SourceID: srcID, AnimalID: animal, DueAt: due,
		}, now)
		require.NoError(t, err, "%s: ReverifyItem", label)
		require.Equal(t, full.Status, rev.Item.Status, "%s: status", label)
		require.Equal(t, full.Applicable, rev.Item.Applicable, "%s: applicable", label)
		require.Equal(t, full.OverriddenBy, rev.Item.OverriddenBy, "%s: overridden_by", label)
		require.True(t, full.Occurrence.DueAt.Equal(rev.Item.Occurrence.DueAt), "%s: due_at", label)
		require.NotNil(t, rev.Ctx, "%s: ctx", label)
	}

	ruleID := rule.ID.String()
	planID := ap.ID.String()

	// animal 2 (no plan): plain rule occurrences
	assertEquiv("due", models.ApplicationSourceRule, ruleID, f.animalIDs[1], slotOn(now, soon))
	assertEquiv("late/hors-delai", models.ApplicationSourceRule, ruleID, f.animalIDs[1], slotOn(now, past))
	assertEquiv("scheduled", models.ApplicationSourceRule, ruleID, f.animalIDs[1], slotOn(now, future))

	// animal 1: rule occurrences overridden by the kind-replacing plan
	assertEquiv("overridden-due", models.ApplicationSourceRule, ruleID, f.animalIDs[0], slotOn(now, soon))
	// the plan's own occurrence
	assertEquiv("plan-item", models.ApplicationSourceAnimal, planID, f.animalIDs[0], slotOn(now, soon))

	// terminal state: apply the plan item, then both views must say applied
	app := &models.CarePlanApplication{
		ID:         uuid.Must(uuid.NewV4()),
		SourceType: models.ApplicationSourceAnimal,
		SourceID:   ap.ID,
		AnimalID:   f.animalIDs[0],
		DueAt:      slotOn(now, soon),
		Status:     models.ApplicationStatusApplied,
		AppliedAt:  now,
		UserID:     f.userID,
	}
	require.NoError(t, snapshotApplication(tx, app))
	plan2, err := BuildDayPlan(tx, now, time.Time{}, time.Time{})
	require.NoError(t, err)
	var fullApplied *careplan.PlanItem
	for i := range plan2.Items {
		it := &plan2.Items[i]
		src := it.Occurrence.Source
		if src != nil && src.SourceID() == planID && it.Occurrence.AnimalID == f.animalIDs[0] &&
			it.Occurrence.DueAt.Equal(slotOn(now, soon)) {
			fullApplied = it
		}
	}
	require.NotNil(t, fullApplied)
	require.Equal(t, careplan.StatusApplied, fullApplied.Status)
	rev, err := ReverifyItem(tx, planItemRef{
		SourceType: models.ApplicationSourceAnimal, SourceID: planID,
		AnimalID: f.animalIDs[0], DueAt: slotOn(now, soon),
	}, now)
	require.NoError(t, err)
	require.Equal(t, fullApplied.Status, rev.Item.Status, "terminal: status")
	require.Equal(t, fullApplied.Applicable, rev.Item.Applicable, "terminal: applicable")

	// negative: an occurrence the source does not produce
	_, err = ReverifyItem(tx, planItemRef{
		SourceType: models.ApplicationSourceRule, SourceID: ruleID,
		AnimalID: f.animalIDs[1], DueAt: now.Add(97 * time.Hour),
	}, now)
	require.ErrorIs(t, err, errItemNotReproduced)
}

// ReverifyItem latch case (§5.4 + M3 bound): a latch rule whose matcher no
// longer matches stays a member through a recent application — identical
// in the full plan and in the single-item recompute.
func TestReverifyItemLatchEquivalence(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()

	matcher := &models.CareMatcher{
		ID:         uuid.Must(uuid.NewV4()),
		Name:       "MLatch-" + f.marker,
		Expression: `species = "NO-SUCH-SPECIES"`,
	}
	require.NoError(t, tx.Create(matcher))

	sched, err := json.Marshal(map[string]interface{}{
		"times":         []string{now.Format("15:04")},
		"anchor":        "fixed",
		"anchor_date":   now.AddDate(0, 0, -2).Format("2006-01-02"),
		"duration_days": 7,
	})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]interface{}{
		"caretype_id": f.feedCare.String(),
		"food":        "grenouilles",
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID:              uuid.Must(uuid.NewV4()),
		Name:            "RLatch-" + f.marker,
		ActionKind:      "feeding",
		ActionPayload:   payload,
		Schedule:        sched,
		Active:          true,
		MatcherID:       uuid.NullUUID{UUID: matcher.ID, Valid: true},
		LatchMembership: true,
	}
	require.NoError(t, tx.Create(rule))

	// application yesterday (inside the 6-day course horizon) → latch holds
	appDue := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local).AddDate(0, 0, -1)
	app := &models.CarePlanApplication{
		ID:         uuid.Must(uuid.NewV4()),
		SourceType: models.ApplicationSourceRule,
		SourceID:   rule.ID,
		AnimalID:   f.animalIDs[0],
		DueAt:      appDue,
		Status:     models.ApplicationStatusApplied,
		AppliedAt:  appDue,
		UserID:     f.userID,
	}
	require.NoError(t, snapshotApplication(tx, app))

	plan, err := BuildDayPlan(tx, now, time.Time{}, time.Time{})
	require.NoError(t, err)
	due := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), 0, 0, time.Local)
	var full *careplan.PlanItem
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src != nil && src.SourceID() == rule.ID.String() &&
			it.Occurrence.AnimalID == f.animalIDs[0] && it.Occurrence.DueAt.Equal(due) {
			full = it
		}
	}
	require.NotNil(t, full, "latched occurrence must exist in the full plan")

	rev, err := ReverifyItem(tx, planItemRef{
		SourceType: models.ApplicationSourceRule, SourceID: rule.ID.String(),
		AnimalID: f.animalIDs[0], DueAt: due,
	}, now)
	require.NoError(t, err, "latched occurrence must reproduce")
	require.Equal(t, full.Status, rev.Item.Status)
	require.Equal(t, full.Applicable, rev.Item.Applicable)

	// animal 2 has no application → matcher misses → not a member anywhere
	_, err = ReverifyItem(tx, planItemRef{
		SourceType: models.ApplicationSourceRule, SourceID: rule.ID.String(),
		AnimalID: f.animalIDs[1], DueAt: due,
	}, now)
	require.ErrorIs(t, err, errItemNotReproduced)
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src != nil && src.SourceID() == rule.ID.String() {
			require.NotEqual(t, f.animalIDs[1], it.Occurrence.AnimalID, "animal 2 must not be latched in the full plan")
		}
	}
}

// slotOn renders the occurrence instant of a same-day slot (engine times
// carry zero seconds, local zone).
func slotOn(day, slot time.Time) time.Time {
	return time.Date(day.Year(), day.Month(), day.Day(), slot.Hour(), slot.Minute(), 0, 0, time.Local)
}

// snapshotApplication fills the mandatory source_snapshot column (§4.5)
// and creates the application row.
func snapshotApplication(tx *pop.Connection, app *models.CarePlanApplication) error {
	app.SourceSnapshot = json.RawMessage(`{"test": true}`)
	return tx.Create(app)
}
