package actions

import (
	"encoding/json"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// TestDisplayName strips the conversion markers from display (bugs.md U5):
// the DB keeps them for rollback identification, the UI/JSON never shows
// them.
func TestDisplayName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Alimentation — rouge (conversion)", "Alimentation — rouge"},
		{"Régime « NB » (conversion) (à vérifier)", "Régime « NB »"},
		{"Traitement — Itra (à vérifier)", "Traitement — Itra"},
		{"Soin de plaie (conversion) (à vérifier)", "Soin de plaie"},
		{"Plain name", "Plain name"},
		{"", ""},
		{"  spaced  ", "spaced"},
		// mid-string markers also go (marker is a suffix convention, but a
		// stray one must not leak into display either)
		{"A (conversion) B", "A B"},
	}
	for _, c := range cases {
		require.Equal(t, c.want, DisplayName(c.in), "DisplayName(%q)", c.in)
	}
}

// testSource builds an active feeding/cleanup source with a JSON payload.
func testSource(kind, id, name string, payload map[string]interface{}) *careplan.Source {
	s := careplan.NewSource(careplan.SourceRule, id, name, kind, careplan.Schedule{})
	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			panic(err)
		}
		s.PayloadJSON = raw
	}
	return s
}

// testPlan builds a DayPlan with two animals in one cage.
func testPlan() *DayPlan {
	return &DayPlan{Animals: &planAnimals{
		ctxs: map[int]*careplan.AnimalContext{},
		rows: map[int]models.Animal{
			1: {ID: 1, YearNumber: 11, Year: 2026, Zone: nulls.NewString("Z1"), Cage: nulls.NewString("C1")},
			2: {ID: 2, YearNumber: 12, Year: 2026, Zone: nulls.NewString("Z1"), Cage: nulls.NewString("C1")},
			3: {ID: 3, YearNumber: 13, Year: 2026, Zone: nulls.NewString("Z2"), Cage: nulls.NewString("C9")},
		},
	}}
}

func testItem(src careplan.PlanSource, animalID int, status careplan.PlanStatus) careplan.PlanItem {
	return careplan.PlanItem{
		Occurrence: careplan.Occurrence{Source: src, AnimalID: animalID, DueAt: time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)},
		Status:     status,
		Applicable: true,
	}
}

// TestGroupCardsFeeding: one card per (cage × normalized food) — same cage
// + same diet across DIFFERENT sources merge into one card (bugs.md U1);
// food normalization is case/whitespace-insensitive; cleanup keeps its
// source × cage cards (§6.2a).
func TestGroupCardsFeeding(t *testing.T) {
	plan := testPlan()
	ruleA := testSource(careplan.KindFeeding, "src-A", "Feed A (conversion)", map[string]interface{}{"food": "Croquettes + VDF"})
	planB := testSource(careplan.KindFeeding, "src-B", "Feed B", map[string]interface{}{"food": "  croquettes  + vdf ", "force_feed": true})
	ruleC := testSource(careplan.KindFeeding, "src-C", "Feed C", map[string]interface{}{"food": "poussins"})
	cleanup := testSource(careplan.KindCleanup, "src-D", "Clean", map[string]interface{}{"note": "désinfecter"})

	items := []careplan.PlanItem{
		testItem(ruleA, 1, careplan.StatusDue),
		testItem(planB, 2, careplan.StatusDue),
		testItem(ruleC, 3, careplan.StatusDue),
		testItem(cleanup, 1, careplan.StatusDue),
		testItem(cleanup, 2, careplan.StatusDue),
	}
	cages, feedings := GroupCards(items, plan)

	// Cleanup: one (source × cage) card holding both animals.
	require.Len(t, cages, 1)
	require.Equal(t, "C1", cages[0].Cage)
	require.Len(t, cages[0].Items, 2)

	// Feeding: C1 diet merges across sources (2 chips), C9 gets its own card.
	require.Len(t, feedings, 2)
	require.Equal(t, "C1", feedings[0].Cage)
	require.Equal(t, "croquettes + vdf", feedings[0].Food)
	require.True(t, feedings[0].ForceFeed)
	require.Len(t, feedings[0].Chips, 2)
	require.Equal(t, "11/26", feedings[0].Chips[0].Label)
	require.Equal(t, "12/26", feedings[0].Chips[1].Label)
	require.Equal(t, "C9", feedings[1].Cage)
	require.Equal(t, "poussins", feedings[1].Food)
	require.False(t, feedings[1].ForceFeed)
	require.Len(t, feedings[1].Chips, 1)
}

// TestGroupCardsSorting: cards sort by zone then cage (round order §6.2a).
func TestGroupCardsSorting(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindCleanup, "src-A", "Clean", nil)
	items := []careplan.PlanItem{
		testItem(src, 3, careplan.StatusDue), // Z2/C9
		testItem(src, 1, careplan.StatusDue), // Z1/C1
	}
	cages, _ := GroupCards(items, plan)
	require.Len(t, cages, 2)
	require.Equal(t, "C1", cages[0].Cage)
	require.Equal(t, "C9", cages[1].Cage)
}

// TestBuildDayPlanViewFeedingCards: compact view diverts feeding items out
// of the per-animal tiers into one cage × diet card; overridden chips stay
// off the work screen; applicable refs feed the apply-group button.
func TestBuildDayPlanViewFeedingCards(t *testing.T) {
	plan := testPlan()
	ruleA := testSource(careplan.KindFeeding, "src-A", "Feed A (conversion)", map[string]interface{}{"food": "grenouilles"})
	planB := testSource(careplan.KindFeeding, "src-B", "Feed B", map[string]interface{}{"food": "Grenouilles"})
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	items := []careplan.PlanItem{
		testItem(ruleA, 1, careplan.StatusDue),
		testItem(planB, 2, careplan.StatusDue),
	}
	over := testItem(planB, 2, careplan.StatusOverridden)
	over.Applicable = false
	items = append(items, over)
	plan.Items = items

	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	for i, tier := range v.Tiers {
		require.Empty(t, tier.Cards, "tier %d must not carry feeding rows (compact)", i)
	}
	require.Len(t, v.Feedings, 1)
	fc := v.Feedings[0]
	require.Equal(t, "grenouilles", fc.Food)
	require.Len(t, fc.Chips, 2) // overridden chip filtered out
	require.Equal(t, 2, fc.ApplicableCount)
	require.Contains(t, fc.ChipRefsJSON, `"source_id":"src-A"`)
	require.Contains(t, fc.ChipRefsJSON, `"source_id":"src-B"`)

	// zone filter narrows feeding cards
	v = BuildDayPlanView(plan, ViewCompact, "Z2", "", now)
	require.Empty(t, v.Feedings)
	// kind filter ≠ feeding hides feeding cards
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindCare, now)
	require.Empty(t, v.Feedings)
}

// TestPlanDetail per-kind content lines (bugs.md U5).
func TestPlanDetail(t *testing.T) {
	require.Equal(t, "grenouilles", planDetail(testSource(careplan.KindFeeding, "1", "n", map[string]interface{}{"food": "grenouilles"})))
	require.Equal(t, "NB 🍼", planDetail(testSource(careplan.KindFeeding, "2", "n", map[string]interface{}{"food": "NB", "force_feed": true})))
	require.Equal(t, "Itra — 0.1 ml", planDetail(testSource(careplan.KindMedication, "3", "n", map[string]interface{}{"drug": "Itra", "dosage": "0.1 ml"})))
	require.Equal(t, "Itra — ⧗ auto", planDetail(testSource(careplan.KindMedication, "4", "n", map[string]interface{}{"drug": "Itra", "dosage_from_dosages_table": true})))
	require.Equal(t, "désinfecter", planDetail(testSource(careplan.KindCleanup, "5", "n", map[string]interface{}{"note": "désinfecter"})))
	require.Equal(t, "check respiration", planDetail(testSource(careplan.KindObservation, "6", "n", map[string]interface{}{"prompt": "check respiration"})))
	require.Equal(t, "", planDetail(testSource(careplan.KindFeeding, "7", "n", nil)))
}

// TestPlanJSONRowsDetailAndDisplayName: Detail is additive in the JSON read
// model (backward compatible) and source_name is marker-stripped (U5).
func TestPlanJSONRowsDetailAndDisplayName(t *testing.T) {
	plan := testPlan()
	src := testSource(careplan.KindFeeding, "src-A", "Alimentation — X (conversion)", map[string]interface{}{"food": "grenouilles"})
	plan.Items = []careplan.PlanItem{testItem(src, 1, careplan.StatusDue)}
	rows := planJSONRows(plan)
	require.Len(t, rows, 1)
	require.Equal(t, "Alimentation — X", rows[0].SourceName)
	require.Equal(t, "grenouilles", rows[0].Detail)

	raw, err := json.Marshal(rows[0])
	require.NoError(t, err)
	require.Contains(t, string(raw), `"detail":"grenouilles"`)
	// empty detail → key omitted (additive contract)
	rows[0].Detail = ""
	raw, err = json.Marshal(rows[0])
	require.NoError(t, err)
	require.NotContains(t, string(raw), "detail")
}
