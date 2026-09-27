//go:build !sqlite
// +build !sqlite

package actions

import (
	"encoding/json"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// MySQL round-trip of the §8.1 startup converter: seed library insertion,
// feeding clustering (threshold ≥5), treatment series → bounded plans,
// marker write + persisted report, and no-op/idempotent re-runs.

type converterFixture struct {
	f         *planFixture
	animalIDs []int
	treatment models.Treatment
}

func setupConverterFixture(t *testing.T) *converterFixture {
	t.Helper()
	requireMySQLTestDB(t)
	db := models.DB

	// Start from a clean converter state: marker + everything tagged.
	cleanupConverterRows(t)

	fx := &converterFixture{f: setupPlanFixture(t)}

	// The §7.4 seeds resolve caretypes by canonical name ("Repas",
	// "Alimentation", "Soin"); ensure they exist so the seed step is
	// deterministic regardless of what earlier test runs left behind.
	for _, name := range []string{"Repas", "Alimentation", "Soin"} {
		var ct models.Caretype
		err := db.Where("name = ?", name).First(&ct)
		if err == nil {
			continue
		}
		ct = models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: name}
		if name == "Repas" || name == "Alimentation" {
			ct.Type = models.CareTypeFeed
		}
		require.NoError(t, db.Create(&ct))
		t.Cleanup(func() {
			models.DB.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec()
		})
	}

	// One feeding caretype is guaranteed by the plan fixture (CPFeed-*).

	// 5 cluster animals: same diet "grains pigeons eau", 08:00–18:00 @120min.
	diet := "Grains pigeons eau"
	for i := 0; i < 5; i++ {
		id := fx.f.mkAnimal(t, db, "Pigeon biset")
		fx.setFeeding(t, db, id, diet, "08:00", "18:00", 120)
		fx.animalIDs = append(fx.animalIDs, id)
	}
	// 2 unique-diet animals → per-animal plans.
	for _, d := range []string{"Mixture A", "Mixture B"} {
		id := fx.f.mkAnimal(t, db, " Tourterelle ")
		fx.setFeeding(t, db, id, d, "09:00", "15:00", 180)
		fx.animalIDs = append(fx.animalIDs, id)
	}
	// 1 fallback animal: empty diet text → converted to a flagged
	// "(à vérifier)" plan instead of being dropped (§1.5 no-skip rule).
	idFallback := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, idFallback, "  ", "08:00", "18:00", 120)
	fx.animalIDs = append(fx.animalIDs, idFallback)
	// 1 same-diet animal on a DIFFERENT schedule (H2: no modal collapse —
	// distinct slot sets must never merge into one cluster rule).
	idVariant := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, idVariant, diet, "07:00", "19:00", 360)
	fx.animalIDs = append(fx.animalIDs, idVariant)

	// One open treatment series (3 future days, morning+noon). The drug is
	// seeded in the drugs table so the series converts as a MEDICATION plan.
	drug := models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "Ivomec 1%"}
	require.NoError(t, db.Create(&drug))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})
	idTr := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	fx.animalIDs = append(fx.animalIDs, idTr)
	base := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 1)
	for i := 0; i < 3; i++ {
		tr := models.Treatment{
			Date:       base.AddDate(0, 0, i),
			AnimalID:   idTr,
			Drug:       "Ivomec 1%",
			Dosage:     "0,1 ml",
			Remarks:    nulls.NewString("contrôle jour 5"),
			Timebitmap: models.Treatement_MORNING | models.Treatement_NOON,
		}
		require.NoError(t, db.Create(&tr))
		if i == 0 {
			fx.treatment = tr
		}
	}

	// M5 routing fixtures — same animal, three edge series:
	// (a) unknown drug (not in drugs table) with dosage → observation plan,
	//     never an invented-dosage medication plan;
	idTr2 := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	fx.animalIDs = append(fx.animalIDs, idTr2)
	trUnknown := models.Treatment{
		Date:       base,
		AnimalID:   idTr2,
		Drug:       "Produit inconnu XYZ",
		Dosage:     "1 cp",
		Timebitmap: models.Treatement_MORNING,
	}
	require.NoError(t, db.Create(&trUnknown))
	// (b) wound-care pseudo-drug → observation plan;
	idTr3 := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	fx.animalIDs = append(fx.animalIDs, idTr3)
	trWound := models.Treatment{
		Date:       base,
		AnimalID:   idTr3,
		Drug:       models.WoundCareDrugName,
		Dosage:     "",
		Timebitmap: models.Treatement_NOON,
	}
	require.NoError(t, db.Create(&trWound))
	// (c) known drug but empty dosage → observation plan (§1.5, M1-adjacent:
	// a medication payload without dosage fails validation).
	idTr4 := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	fx.animalIDs = append(fx.animalIDs, idTr4)
	trNoDose := models.Treatment{
		Date:       base,
		AnimalID:   idTr4,
		Drug:       "Ivomec 1%",
		Dosage:     "",
		Timebitmap: models.Treatement_MORNING,
	}
	require.NoError(t, db.Create(&trNoDose))
	return fx
}

func (fx *converterFixture) setFeeding(t *testing.T, db *pop.Connection, id int, diet, start, end string, period int) {
	t.Helper()
	startT, err := time.Parse("15:04", start)
	require.NoError(t, err)
	endT, err := time.Parse("15:04", end)
	require.NoError(t, err)
	// time.Parse yields UTC wall-clock — same as the production write path
	// (animals.go timeToNullTime); the DATETIME round-trip must not shift.
	base := time.Date(2026, 9, 20, 0, 0, 0, 0, time.UTC)
	a := &models.Animal{}
	require.NoError(t, db.Find(a, id))
	a.Feeding = nulls.NewString(diet)
	a.FeedingStart = nulls.NewTime(base.Add(time.Duration(startT.Hour())*time.Hour + time.Duration(startT.Minute())*time.Minute))
	a.FeedingEnd = nulls.NewTime(base.Add(time.Duration(endT.Hour())*time.Hour + time.Duration(endT.Minute())*time.Minute))
	a.FeedingPeriod = period
	require.NoError(t, db.Update(a))
}

func cleanupConverterRows(t *testing.T) {
	t.Helper()
	db := models.DB
	db.RawQuery("DELETE FROM care_plan_conversion").Exec()
	db.RawQuery("DELETE FROM care_animal_plans WHERE created_by IS NULL").Exec()
	db.RawQuery("DELETE FROM care_rules WHERE description LIKE ?", "%"+ConverterTag+"%").Exec()
	db.RawQuery("DELETE FROM care_matchers WHERE description LIKE ?", "%"+ConverterTag+"%").Exec()
}

// TestCarePlanConverterRoundTrip runs the whole §8.1 sequence twice:
// first boot converts; second boot is a no-op; marker deletion re-runs
// without duplicating anything.
func TestCarePlanConverterRoundTrip(t *testing.T) {
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()
	db := models.DB

	count := func(query string, args ...interface{}) int64 {
		var rows []struct {
			C int64 `db:"c"`
		}
		require.NoError(t, db.RawQuery(query, args...).All(&rows))
		require.Len(t, rows, 1)
		return rows[0].C
	}

	rulesBefore := count("SELECT count(*) as c FROM care_rules")
	matchersBefore := count("SELECT count(*) as c FROM care_matchers")
	plansBefore := count("SELECT count(*) as c FROM care_animal_plans")

	// --- first boot -------------------------------------------------------
	report, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	require.NotNil(t, report)

	// Seed library: 13 canonical + 4 derived matchers, SR1–SR12.
	require.Equal(t, int64(17), int64(report.Seeds.MatchersInserted), "seed matchers inserted")
	require.Equal(t, 12, report.Seeds.RulesInserted+report.Seeds.RulesSkipped, "SR1–SR12 accounted")
	require.Equal(t, 12, report.Seeds.RulesInserted, "all 12 seeds insert on a clean run")

	// Feeding: one cluster rule (5 identical-schedule animals), three
	// per-animal plans (2 unique diets + 1 empty-diet fallback + 1
	// schedule variant — variants must NOT collapse into the cluster, H2).
	// The empty-diet fallback also lands as a plan (no-skip §1.5), so:
	// plans = Mixture A, Mixture B, fallback, variant = 4.
	require.Equal(t, 1, report.Feeding.RulesCreated)
	require.Equal(t, 4, report.Feeding.PlansCreated)
	require.Equal(t, 9, report.Feeding.AnimalsConsidered)
	require.Len(t, report.Feeding.Lines, 9)

	// Reconciliation (§1.6): every animal accounted for, nothing uncovered.
	require.Equal(t, 0, report.Reconciliation.FeedingUncovered,
		"no feeding animal may be left without a covering plan")
	require.Equal(t, 9, report.Reconciliation.FeedingOK+
		report.Reconciliation.FeedingDegraded)
	require.Equal(t, 0, report.Reconciliation.TreatmentUncovered)

	// Treatments: the 3-day series becomes one bounded plan. The test DB
	// may carry unrelated open series from other tests, so only spot-check
	// ours.
	require.GreaterOrEqual(t, report.Treatments.Series, 1)
	require.GreaterOrEqual(t, report.Treatments.PlansCreated, 1)

	// DB state: absolute counts hold only for converter-attributable rows;
	// unrelated fixtures may exist, hence report-driven deltas.
	require.Equal(t, rulesBefore+int64(12+report.Feeding.RulesCreated), count("SELECT count(*) as c FROM care_rules"))
	require.Equal(t, matchersBefore+int64(17+report.Feeding.RulesCreated), count("SELECT count(*) as c FROM care_matchers"))
	require.Equal(t, plansBefore+int64(report.Feeding.PlansCreated+report.Treatments.PlansCreated),
		count("SELECT count(*) as c FROM care_animal_plans"))

	// Cluster rule: species IN matcher + slots INCLUDING the start slot
	// (B1 fix: legacy semantics + spec §11.7, 08:00 + n×120 ≤ 18:00).
	var clusterRule models.CareRule
	require.NoError(t, db.Where("name LIKE ?", "Alimentation — grains pigeons eau%").First(&clusterRule))
	require.True(t, clusterRule.Active, "converted cluster rule must be active immediately")
	var clusterMatcher models.CareMatcher
	require.NoError(t, db.Find(&clusterMatcher, clusterRule.MatcherID.UUID))
	require.Contains(t, clusterMatcher.Expression, `species IN ("Pigeon biset")`)
	sched, err := parseScheduleForTest(clusterRule.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"08:00", "10:00", "12:00", "14:00", "16:00", "18:00"}, sched)

	// H2: the 07:00/360min schedule variant got its own plan — the cluster
	// rule must NOT have silently swallowed it via modal slots.
	var variantPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", fx.animalIDs[8], "%grains pigeons eau%").First(&variantPlan))
	vSched, err := parseScheduleForTest(variantPlan.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"07:00", "13:00", "19:00"}, vSched)

	// t5: the empty-diet animal got a flagged fallback plan, not a skip.
	var fbPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", fx.animalIDs[7], "%(à vérifier)").First(&fbPlan))
	require.Contains(t, fbPlan.Name, "(conversion)")

	// Treatment plan: 08:00/12:00 × 3 days from the bitmap, fixed anchor.
	var trPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name = ?", fx.treatment.AnimalID, "Traitement — Ivomec 1%").First(&trPlan))
	trSched, err := parseScheduleForTest(trPlan.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"08:00", "12:00"}, trSched)

	// M5 routing: the three edge series became flagged OBSERVATION plans
	// (observation payload requires a "prompt"; medication requires dosage).
	assertObservationPlan := func(animalID int, nameLike string) models.CareAnimalPlan {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", animalID, nameLike).First(&p))
		require.Contains(t, p.Name, "(à vérifier)")
		var payload struct {
			Prompt string `json:"prompt"`
		}
		require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
		require.NotEmpty(t, payload.Prompt, "observation plan must carry a prompt")
		return p
	}
	assertObservationPlan(fx.animalIDs[10], "Traitement — Produit inconnu XYZ%")
	assertObservationPlan(fx.animalIDs[11], "Soin de plaie (conversion)%")
	assertObservationPlan(fx.animalIDs[12], "Traitement — Ivomec 1% (à vérifier)")

	// No medication plan may exist for the unknown drug (invented dosage).
	n, err := db.Where("animal_id = ? AND name = ?", fx.animalIDs[10], "Traitement — Produit inconnu XYZ").Count(&models.CareAnimalPlan{})
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// Marker + persisted report.
	var marker models.CarePlanConversion
	require.NoError(t, db.Where("`key` = ?", ConversionMarkerKey).First(&marker))
	require.True(t, marker.Report.Valid)
	var persisted ConversionReport
	require.NoError(t, json.Unmarshal([]byte(marker.Report.String), &persisted))
	require.Equal(t, report.Treatments.PlansCreated, persisted.Treatments.PlansCreated)

	// --- second boot: marker present → strict no-op -----------------------
	rulesAfter := count("SELECT count(*) as c FROM care_rules")
	plansAfter := count("SELECT count(*) as c FROM care_animal_plans")
	report2, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	require.Nil(t, report2)
	require.NoError(t, RunCarePlanConverterAtBoot())
	require.Equal(t, rulesAfter, count("SELECT count(*) as c FROM care_rules"))
	require.Equal(t, plansAfter, count("SELECT count(*) as c FROM care_animal_plans"))

	// --- marker deleted → re-runs, but inserts nothing new ----------------
	require.NoError(t, db.RawQuery("DELETE FROM care_plan_conversion").Exec())
	report3, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	require.NotNil(t, report3)
	require.Equal(t, 0, report3.Seeds.MatchersInserted, "seed matchers deduped by name")
	require.Equal(t, 0, report3.Seeds.RulesInserted, "seed rules deduped by name")
	require.Equal(t, 0, report3.Feeding.RulesCreated, "cluster rule deduped")
	require.Equal(t, 0, report3.Feeding.PlansCreated, "per-animal plans deduped")
	require.Equal(t, 0, report3.Treatments.PlansCreated, "treatment plans deduped")
	require.Equal(t, plansAfter, count("SELECT count(*) as c FROM care_animal_plans"))
}

// parseScheduleForTest extracts the validated times list of a schedule doc.
func parseScheduleForTest(raw []byte) ([]string, error) {
	s, err := careplan.ParseScheduleJSON(raw)
	if err != nil {
		return nil, err
	}
	out := make([]string, len(s.Times))
	for i, t := range s.Times {
		out[i] = t.String()
	}
	return out, nil
}
