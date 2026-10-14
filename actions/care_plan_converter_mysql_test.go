//go:build !sqlite
// +build !sqlite

package actions

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

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
	// BUG-10 feeding-completion cases
	nullTimesID int // period > 0, NULL start/end, diet text
	dietOnlyID  int // diet text only
	noFeedingID int // no legacy feeding trace at all
	emptyDietID int // feeding window set but NO diet text → no plan (user ruling)
	// M5 routing animals (named — positional animalIDs indexes shift
	// whenever fixture rows are added)
	trUnknownDrugID int
	trWoundID       int
	trNoDosageID    int
	// BUG-50 start==end (zero-length window) animal
	adLibID int
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
	// 1 empty-diet animal: NO diet text → NO feeding plan (user ruling:
	// an empty feeding means the animal is not on a feeding regimen; the
	// former "(régime non spécifié)" placeholder named nothing).
	idFallback := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, idFallback, "  ", "08:00", "18:00", 120)
	fx.animalIDs = append(fx.animalIDs, idFallback)
	// 1 same-diet animal on a DIFFERENT schedule (H2: no modal collapse —
	// distinct slot sets must never merge into one cluster rule).
	idVariant := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, idVariant, diet, "07:00", "19:00", 360)
	fx.animalIDs = append(fx.animalIDs, idVariant)

	// BUG-50: a zero-length window (start == end) — the legacy "à
	// disposition" grain-cage encoding (production: animal 10231,
	// 10:00/10:00/600, rendered "from 10:00 to 10:00 every 10:00").
	idAdLib := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, idAdLib, diet, "10:00", "10:00", 600)
	fx.animalIDs = append(fx.animalIDs, idAdLib)
	fx.adLibID = idAdLib

	// BUG-10 (2026-10-14): animals the pre-v4 converter silently skipped.
	// (a) period > 0 but NULL start/end + diet text (the animal-8842 case):
	//     must be converted with the fallback slot (single daily 09:00 or
	//     legacy-start slot — definitive, never flagged).
	idNullTimes := fx.f.mkAnimal(t, db, "Renard roux")
	fx.setFeedingNullTimes(t, db, idNullTimes, "15 poussins et 5 souris", 480)
	fx.animalIDs = append(fx.animalIDs, idNullTimes)
	// (b) diet text only (no period, no times): still a legacy feeding
	//     trace — must be converted (fallback slot, definitive).
	idDietOnly := fx.f.mkAnimal(t, db, "Fou de Bassan")
	fx.setFeedingNullTimes(t, db, idDietOnly, "Poisson entier", 0)
	fx.animalIDs = append(fx.animalIDs, idDietOnly)
	// (c) NO legacy feeding trace at all: must NOT be converted.
	idNoFeeding := fx.f.mkAnimal(t, db, "Hérisson")
	fx.animalIDs = append(fx.animalIDs, idNoFeeding)
	fx.nullTimesID = idNullTimes
	fx.dietOnlyID = idDietOnly
	fx.noFeedingID = idNoFeeding
	fx.emptyDietID = idFallback

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
	fx.trUnknownDrugID = idTr2
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
	fx.trWoundID = idTr3
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
	fx.trNoDosageID = idTr4
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

// setFeedingNullTimes writes the BUG-10 shapes: a legacy feeding row with
// NULL start/end (optionally period-only or diet-only).
func (fx *converterFixture) setFeedingNullTimes(t *testing.T, db *pop.Connection, id int, diet string, period int) {
	t.Helper()
	a := &models.Animal{}
	require.NoError(t, db.Find(a, id))
	a.Feeding = nulls.NewString(diet)
	a.FeedingStart = nulls.Time{}
	a.FeedingEnd = nulls.Time{}
	a.FeedingPeriod = period
	require.NoError(t, db.Update(a))
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
	// Pre-R9-5 rows carry "[source: care_plan_converter]" in the description;
	// post-R9-5 rows stamp DefaultRuleDescription and converter-built rows
	// (cluster rule + matcher, per-animal plans) carry the "(conversion)"
	// name suffix. Match all three so leftover converter rows never leak
	// across tests regardless of the description scheme that wrote them.
	db.RawQuery("DELETE FROM care_rules WHERE description LIKE ? OR description = ? OR name LIKE ?", "%"+ConverterTag+"%", DefaultRuleDescription, "% (conversion)%").Exec()
	db.RawQuery("DELETE FROM care_matchers WHERE description LIKE ? OR description = ? OR name LIKE ?", "%"+ConverterTag+"%", DefaultRuleDescription, "% (conversion)%").Exec()
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

	// Seed library: 13 canonical + SM14 (R8-3) + 4 derived matchers, SR1–SR13.
	require.Equal(t, int64(18), int64(report.Seeds.MatchersInserted), "seed matchers inserted")
	require.Equal(t, 13, report.Seeds.RulesInserted+report.Seeds.RulesSkipped, "SR1–SR13 accounted")
	require.Equal(t, 13, report.Seeds.RulesInserted, "all 13 seeds insert on a clean run")

	// Feeding (v3): NO cluster rules, NO matchers — every feeding animal
	// gets its own per-animal plan (bugs.md t10, 2026-10-27 user ruling:
	// cage names are per centers; in doubt, migrate to the animal). The
	// 11 feeding animals = 5 cluster-diet + 2 unique diets + 1 schedule
	// variant + 1 BUG-50 start==end + 2 BUG-10 cases (NULL times +
	// diet-only). The empty-diet
	// animal (window set but no diet text) is NOT converted — user ruling:
	// an empty feeding means no feeding regimen. The no-feeding animal is
	// NOT considered either.
	require.Equal(t, 0, report.Feeding.RulesCreated,
		"v3: the converter must not create rules — per-animal plans only")
	require.Equal(t, 11, report.Feeding.PlansCreated)
	require.Equal(t, 11, report.Feeding.AnimalsConsidered)
	require.Len(t, report.Feeding.Lines, 11)

	// Reconciliation (§1.6): every converted animal accounted for, nothing
	// uncovered. The empty-diet animal is excluded at selection, so it is
	// out of the no-loss gate scope (not UNCOVERED).
	require.Equal(t, 0, report.Reconciliation.FeedingUncovered,
		"no feeding animal may be left without a covering plan")
	require.Equal(t, 11, report.Reconciliation.FeedingOK+
		report.Reconciliation.FeedingDegraded)
	require.Equal(t, 0, report.Reconciliation.TreatmentUncovered)

	// BUG-10: the NULL-times animal got a fallback plan carrying its diet
	// (single daily slot — user ruling 2026-10-14: definitive, never
	// flagged); the diet-only animal likewise; the no-trace animal got
	// nothing. BUG-36: the fallback form is a plain slot list — NO repeat
	// block (the legacy triple is incomplete).
	assertFeedingPlan := func(animalID int, wantFood string) {
		var plans []models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", animalID, careplan.KindFeeding).All(&plans))
		require.Len(t, plans, 1, "animal %d must have exactly one converted feeding plan", animalID)
		p := plans[0]
		require.NotContains(t, p.Name, "à vérifier", "converted names are definitive — plan name %q", p.Name)
		// the payload food is the NORMALIZED (case-folded) diet text.
		require.Contains(t, string(p.ActionPayload), NormalizeDiet(wantFood))
		sched, err := careplan.ParseScheduleJSON(p.Schedule)
		require.NoError(t, err)
		require.NotEmpty(t, sched.Times, "fallback slot must be present")
		require.Nil(t, sched.Repeat, "fallback plan of animal %d must not carry a repeat block", animalID)
	}
	assertFeedingPlan(fx.nullTimesID, "15 poussins et 5 souris")
	assertFeedingPlan(fx.dietOnlyID, "Poisson entier")
	var none []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ?", fx.noFeedingID).All(&none))
	require.Empty(t, none, "an animal without any legacy feeding trace must not gain a plan")
	// User ruling: an animal with a feeding window but NO diet text gets NO
	// feeding plan either — the empty "(régime non spécifié)" placeholder is
	// gone (animal 10415).
	var emptyDiet []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.emptyDietID, careplan.KindFeeding).All(&emptyDiet))
	require.Empty(t, emptyDiet, "an animal with no diet text must not gain a feeding plan")

	// Treatments: the 3-day series becomes one bounded plan. The test DB
	// may carry unrelated open series from other tests, so only spot-check
	// ours.
	require.GreaterOrEqual(t, report.Treatments.Series, 1)
	require.GreaterOrEqual(t, report.Treatments.PlansCreated, 1)

	// DB state: absolute counts hold only for converter-attributable rows;
	// unrelated fixtures may exist, hence report-driven deltas. v3: the
	// feeding step adds only per-animal plans — no rules, no matchers.
	require.Equal(t, rulesBefore+int64(13), count("SELECT count(*) as c FROM care_rules"))
	require.Equal(t, matchersBefore+int64(18), count("SELECT count(*) as c FROM care_matchers"))
	require.Equal(t, plansBefore+int64(report.Feeding.PlansCreated+report.Treatments.PlansCreated),
		count("SELECT count(*) as c FROM care_animal_plans"))

	// v3: no converter-built rule or matcher rows may exist after the run —
	// in particular nothing carrying the "(conversion)" suffix, and no
	// matcher expression may predicate on cage names.
	require.Equal(t, int64(0), count("SELECT count(*) as c FROM care_rules WHERE name LIKE '% (conversion)'"),
		"v3: no converter rules")
	require.Equal(t, int64(0), count("SELECT count(*) as c FROM care_matchers WHERE name LIKE '% (conversion)'"),
		"v3: no converter matchers")
	var cageMatchers []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_matchers WHERE LOWER(expression) LIKE ?", "%cage %").All(&cageMatchers))
	require.Equal(t, int64(0), cageMatchers[0].C, "no matcher may predicate on cage names")

	// Per-animal plans: the 5 same-diet animals each get their OWN plan
	// with THEIR slots (08:00 + n×120 ≤ 18:00 — B1 legacy semantics).
	// BUG-36: the exact legacy triple converts to the REPEAT form — block
	// matching the legacy columns, expanded times, and NO "à vérifier"
	// flag (the translation is exact).
	dietSlots := []string{"08:00", "10:00", "12:00", "14:00", "16:00", "18:00"}
	for _, id := range fx.animalIDs[:5] {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", id, "Alimentation — grains pigeons eau%").First(&p))
		require.True(t, p.Active)
		require.NotContains(t, p.Name, "à vérifier", "animal %d: exact triple must convert unflagged", id)
		slots, err := parseScheduleForTest(p.Schedule)
		require.NoError(t, err)
		require.Equal(t, dietSlots, slots, "animal %d slots", id)
		sched, err := careplan.ParseScheduleJSON(p.Schedule)
		require.NoError(t, err)
		require.NotNil(t, sched.Repeat, "animal %d: exact triple must carry the repeat block", id)
		require.Equal(t, "08:00", sched.Repeat.From.String(), "animal %d repeat.from", id)
		require.Equal(t, "18:00", sched.Repeat.To.String(), "animal %d repeat.to", id)
		require.Equal(t, 120, sched.Repeat.EveryMinutes, "animal %d repeat.every_minutes", id)
		var payload struct {
			Food string `json:"food"`
		}
		require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
		require.Equal(t, "grains pigeons eau", payload.Food)
	}

	// H2: the 07:00/360min schedule variant got its own plan — the cluster
	// rule must NOT have silently swallowed it via modal slots. BUG-36: it
	// carries its own repeat block (07:00→19:00 every 360).
	var variantPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", fx.animalIDs[8], "%grains pigeons eau%").First(&variantPlan))
	vSched, err := parseScheduleForTest(variantPlan.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"07:00", "13:00", "19:00"}, vSched)
	vParsed, err := careplan.ParseScheduleJSON(variantPlan.Schedule)
	require.NoError(t, err)
	require.NotNil(t, vParsed.Repeat, "schedule variant must carry the repeat block")
	require.Equal(t, "07:00", vParsed.Repeat.From.String())
	require.Equal(t, "19:00", vParsed.Repeat.To.String())
	require.Equal(t, 360, vParsed.Repeat.EveryMinutes)

	// BUG-50: the zero-length window (start == end) converts to the PLAIN
	// single-slot schedule — no repeat block, legacy grace 300 (period/2).
	var adLib models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.adLibID, careplan.KindFeeding).First(&adLib))
	require.NotContains(t, adLib.Name, "à vérifier", "start==end is an exact translation — no flag")
	adParsed, err := careplan.ParseScheduleJSON(adLib.Schedule)
	require.NoError(t, err)
	require.Nil(t, adParsed.Repeat, "start==end must not carry a repeat block")
	adSlots, err := parseScheduleForTest(adLib.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"10:00"}, adSlots)
	require.Equal(t, 300, adParsed.GraceMinutes, "legacy grace = period/2")
	// JSON column: MySQL normalizes the stored doc (key order + spacing),
	// so assert canonical equality, not raw bytes.
	require.Equal(t, canonicalSchedule(`{"times":["10:00"],"every_days":1,"anchor":"intake","grace_minutes":300}`), canonicalSchedule(string(adLib.Schedule)))

	// t5: the empty-diet animal gets NO feeding plan (user ruling: an empty
	// feeding means no feeding regimen — the "(régime non spécifié)"
	// placeholder is gone). fx.animalIDs[7] is the empty-diet animal.
	var fbPlans []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.animalIDs[7], careplan.KindFeeding).All(&fbPlans))
	require.Empty(t, fbPlans, "empty-diet animal must not gain a feeding plan")

	// Treatment plan: 08:00/12:00 × 3 days from the bitmap, fixed anchor.
	var trPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name = ?", fx.treatment.AnimalID, "Traitement — Ivomec 1%").First(&trPlan))
	trSched, err := parseScheduleForTest(trPlan.Schedule)
	require.NoError(t, err)
	require.Equal(t, []string{"08:00", "12:00"}, trSched)

	// M5 routing: the three edge series became definitive OBSERVATION/CARE
	// plans (observation payload requires a "prompt"; medication requires
	// dosage). B10-8: unknown-drug and wound-care series convert to CARE
	// plans typed "Soin" (the legacy drug column carried non-drug entries);
	// a KNOWN drug without posology becomes an observation (dosage is
	// required for a medication payload). User ruling 2026-10-14: no
	// "à vérifier" flag — every conversion is definitive.
	assertConvertedPlan := func(animalID int, nameLike, wantKind string) models.CareAnimalPlan {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", animalID, nameLike).First(&p))
		require.NotContains(t, p.Name, "(à vérifier)", "converted names are definitive")
		require.Equal(t, wantKind, p.ActionKind, "routing kind")
		if wantKind == careplan.KindCare {
			var payload struct {
				CaretypeID   string `json:"caretype_id"`
				Note         string `json:"note"`
				Instructions string `json:"instructions"`
			}
			require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
			require.NotEmpty(t, payload.CaretypeID, "care plan must carry its caretype")
			require.NotEmpty(t, payload.Note, "care plan must carry the content line in note")
		} else {
			var payload struct {
				Prompt string `json:"prompt"`
			}
			require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
			require.NotEmpty(t, payload.Prompt, "observation plan must carry a prompt")
		}
		return p
	}
	assertConvertedPlan(fx.trUnknownDrugID, "Soin — Produit inconnu XYZ%", careplan.KindCare)
	assertConvertedPlan(fx.trWoundID, "Soin de plaie (conversion)%", careplan.KindCare)
	assertConvertedPlan(fx.trNoDosageID, "Traitement — Ivomec 1%", careplan.KindObservation)

	// No medication plan may exist for the unknown drug (invented dosage).
	n, err := db.Where("animal_id = ? AND name = ?", fx.trUnknownDrugID, "Traitement — Produit inconnu XYZ").Count(&models.CareAnimalPlan{})
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

// countRows is the shared counting helper for converter/migration tests.
func countRows(t *testing.T, db *pop.Connection, query string, args ...interface{}) int64 {
	t.Helper()
	var rows []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery(query, args...).All(&rows))
	require.Len(t, rows, 1)
	return rows[0].C
}

// TestCarePlanMigrationV3MigratesRulesToAnimals pins the startup_v3
// one-shot migration (bugs.md t10, 2026-10-27 user ruling: "cage names
// are per centers — eliminate the cage-name matchers; in doubt, migrate
// to the animal; this general rule must apply to all centers").
//
// Simulated v2-era state (as found in the 2026-10-27 clone audit, where
// the R4-7.24 name fix made v2 re-create renamed duplicates of v1 rules
// and left species-only v1 zombies sweeping whole species):
//   - a converter-owned ACTIVE feeding rule + matcher whose expression
//     narrows a species sweep with a cage clause;
//   - a hand-edited converter rule that must never be touched.
//
// The migration must: create per-animal plans (rule payload + schedule
// VERBATIM) for the animals the rule correctly covered — their own diet
// equals the rule's payload food; DROP coverage for over-swept animals
// (same species, different diet); retire the rule (active=0, row kept —
// care_plan_applications references its id); delete the matcher when no
// rule references it anymore; keep the hand-edited rule and its matcher;
// write the startup_v3 marker; be write-idempotent afterwards.
func TestCarePlanMigrationV3MigratesRulesToAnimals(t *testing.T) {
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()
	db := models.DB

	// v2-era simulation, part 1: run the (new) converter for seeds +
	// per-animal plans, then remove the plans of the 5 same-diet animals —
	// a real v2 database has a cluster RULE for them, not plans.
	_, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	diet := "grains pigeons eau"
	ruleName := convertedFeedingName(diet)
	require.NoError(t, db.RawQuery(
		"DELETE FROM care_animal_plans WHERE animal_id IN (?, ?, ?, ?, ?)",
		fx.animalIDs[0], fx.animalIDs[1], fx.animalIDs[2], fx.animalIDs[3], fx.animalIDs[4]).Exec())

	// v2-era simulation, part 2: the cage-scoped cluster rule + matcher.
	// The schedule slots are deliberately NOT the animals' legacy slots —
	// the migration must copy the rule verbatim, proving it migrates the
	// rule (not a re-derivation).
	m1 := &models.CareMatcher{
		Name:       convertedMatcherName(diet),
		Expression: fmt.Sprintf(`species IN ("Pigeon biset") AND cage =* "%s"`, fx.f.cage),
	}
	verrs, err := db.ValidateAndCreate(m1)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "matcher fixture invalid: %v", verrs)
	r1 := &models.CareRule{
		Name:          ruleName,
		Description:   nulls.NewString(fmt.Sprintf("Cluster alimentation ×5 [source: %s]", ConverterTag)),
		ActionKind:    careplan.KindFeeding,
		ActionPayload: []byte(`{"caretype_id":"fixture-feed","food":"grains pigeons eau","force_feed":false}`),
		Schedule:      []byte(`{"times":["09:00","15:00"],"every_days":1,"anchor":"intake"}`),
		MatcherID:     uuid.NullUUID{UUID: m1.ID, Valid: true},
		Active:        true,
		StopOnOuttake: true,
	}
	verrs, err = db.ValidateAndCreate(r1)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "rule fixture invalid: %v", verrs)

	// A hand-edited converter rule: still converter-owned (name suffix),
	// but touched after creation — the migration must leave it alone.
	m2 := &models.CareMatcher{
		Name:       convertedMatcherName("Mixture A") + " admin",
		Expression: `species IN ("Pigeon biset")`,
	}
	verrs, err = db.ValidateAndCreate(m2)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "matcher2 fixture invalid: %v", verrs)
	r2 := &models.CareRule{
		Name:          convertedFeedingName("Mixture A"),
		Description:   nulls.NewString(fmt.Sprintf("Cluster alimentation ×2 [source: %s]", ConverterTag)),
		ActionKind:    careplan.KindFeeding,
		ActionPayload: []byte(`{"caretype_id":"fixture-feed","food":"Mixture A","force_feed":false}`),
		Schedule:      []byte(`{"times":["06:00"],"every_days":1,"anchor":"intake"}`),
		MatcherID:     uuid.NullUUID{UUID: m2.ID, Valid: true},
		Active:        true,
		StopOnOuttake: true,
	}
	verrs, err = db.ValidateAndCreate(r2)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "rule2 fixture invalid: %v", verrs)
	require.NoError(t, db.RawQuery(
		"UPDATE care_rules SET updated_at = DATE_ADD(updated_at, INTERVAL 1 HOUR) WHERE id = ?", r2.ID).Exec())

	// --- run the migration -------------------------------------------------
	mig, err := RunCarePlanMigration(db)
	require.NoError(t, err)
	require.NotNil(t, mig)
	require.Equal(t, 2, mig.Migration.RulesConsidered)
	require.Equal(t, 1, mig.Migration.RulesRetired)
	require.Equal(t, 1, mig.Migration.RulesHandEditedSkipped)
	require.Equal(t, 5, mig.Migration.PlansCreated)
	require.Equal(t, 1, mig.Migration.OverSweepDropped,
		"only the empty-diet fallback animal is over-swept (different diet)")
	require.Equal(t, 1, mig.Migration.MatchersDeleted)
	require.Equal(t, 1, mig.Migration.MatchersKept)

	// Rule 1 retired but NOT deleted (applications reference its id), and
	// its matcher link is released; rule 2 untouched and still active.
	var r1b models.CareRule
	require.NoError(t, db.Find(&r1b, r1.ID))
	require.False(t, r1b.Active, "migrated rule must be retired (active=0), never deleted")
	require.False(t, r1b.MatcherID.Valid, "retired rule releases its matcher link (FK ON DELETE RESTRICT)")
	var r2b models.CareRule
	require.NoError(t, db.Find(&r2b, r2.ID))
	require.True(t, r2b.Active, "hand-edited rule must stay active")
	require.True(t, r2b.MatcherID.Valid, "hand-edited rule keeps its matcher")

	// Matcher 1 unreferenced → gone; matcher 2 still referenced → kept.
	var n []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_matchers WHERE id = ?", m1.ID).All(&n))
	require.Equal(t, int64(0), n[0].C, "unreferenced cage matcher must be deleted")
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_matchers WHERE id = ?", m2.ID).All(&n))
	require.Equal(t, int64(1), n[0].C, "matcher of the kept rule must survive")

	// The 5 covered animals each got an per-animal plan with the RULE's
	// payload and schedule, verbatim.
	for _, id := range fx.animalIDs[:5] {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name = ?", id, ruleName).First(&p),
			"animal %d must have the migrated plan", id)
		require.True(t, p.Active)
		slots, err := parseScheduleForTest(p.Schedule)
		require.NoError(t, err)
		require.Equal(t, []string{"09:00", "15:00"}, slots, "animal %d: rule schedule copied verbatim", id)
		var payload struct {
			Food       string `json:"food"`
			CaretypeID string `json:"caretype_id"`
			ForceFeed  bool   `json:"force_feed"`
		}
		require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
		require.Equal(t, "grains pigeons eau", payload.Food)
		require.Equal(t, "fixture-feed", payload.CaretypeID)
		require.False(t, payload.ForceFeed)
	}

	// Same-diet/same-name plans are NOT duplicated: the schedule-variant
	// animal (own plan from the converter, 07:00/13:00/19:00) keeps exactly
	// its one plan — the name guard prefers the animal's own slots.
	var vCount []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ?", fx.animalIDs[8]).All(&vCount))
	require.Equal(t, int64(1), vCount[0].C, "variant animal keeps exactly its own plan")

	// The over-swept empty-diet animal got no plan from the rule.
	var oCount []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ?", fx.animalIDs[7], ruleName).All(&oCount))
	require.Equal(t, int64(0), oCount[0].C, "over-swept animal must not receive the rule's plan")

	// Marker written with the persisted migration report.
	var marker models.CarePlanConversion
	require.NoError(t, db.Where("`key` = ?", ConversionMarkerKeyV3).First(&marker))
	require.True(t, marker.Report.Valid)
	var persisted ConversionReport
	require.NoError(t, json.Unmarshal([]byte(marker.Report.String), &persisted))
	require.Equal(t, 1, persisted.Migration.RulesRetired)

	// --- second call: marker present → no-op -------------------------------
	mig2, err := RunCarePlanMigration(db)
	require.NoError(t, err)
	require.Nil(t, mig2)

	// --- marker deleted → re-runs write-neutrally --------------------------
	// Plus the v3.1 completion pass: simulate the FIRST v3 build's residue
	// — a rule already retired but STILL carrying its matcher link (that
	// build never released it, so the cage matcher survived behind the
	// written marker). The re-run must unlink + delete it.
	m3 := &models.CareMatcher{
		Name:       convertedMatcherName("residue diet"),
		Expression: fmt.Sprintf(`species IN ("Pigeon biset") AND cage =* "%s"`, fx.f.cage),
	}
	verrs, err = db.ValidateAndCreate(m3)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "matcher3 fixture invalid: %v", verrs)
	r3 := &models.CareRule{
		Name:          convertedFeedingName("residue diet"),
		Description:   nulls.NewString(fmt.Sprintf("Cluster alimentation ×1 [source: %s]", ConverterTag)),
		ActionKind:    careplan.KindFeeding,
		ActionPayload: []byte(`{"caretype_id":"fixture-feed","food":"residue diet","force_feed":false}`),
		Schedule:      []byte(`{"times":["10:00"],"every_days":1,"anchor":"intake"}`),
		MatcherID:     uuid.NullUUID{UUID: m3.ID, Valid: true},
		Active:        false, // retired by the buggy first build, matcher link left behind
		StopOnOuttake: true,
	}
	verrs, err = db.ValidateAndCreate(r3)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "rule3 fixture invalid: %v", verrs)

	plansTotal := countRows(t, db, "SELECT count(*) as c FROM care_animal_plans")
	require.NoError(t, db.RawQuery("DELETE FROM care_plan_conversion WHERE `key` = ?", ConversionMarkerKeyV3).Exec())
	mig3, err := RunCarePlanMigration(db)
	require.NoError(t, err)
	require.NotNil(t, mig3)
	require.Equal(t, 0, mig3.Migration.RulesRetired, "retired rules are not revisited")
	require.Equal(t, 1, mig3.Migration.RulesHandEditedSkipped, "hand-edited rule still skipped")
	require.Equal(t, 0, mig3.Migration.PlansCreated, "no duplicate plans")
	require.Equal(t, 1, mig3.Migration.MatchersDeleted, "completion pass cleans the retired-but-linked residue")
	require.Equal(t, 1, mig3.Migration.MatchersKept, "the hand-edited active rule's matcher is still kept")
	require.Equal(t, plansTotal, countRows(t, db, "SELECT count(*) as c FROM care_animal_plans"))
	var r3b models.CareRule
	require.NoError(t, db.Find(&r3b, r3.ID))
	require.False(t, r3b.MatcherID.Valid, "residue rule is unlinked")
	var n3 []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery("SELECT count(*) as c FROM care_matchers WHERE id = ?", m3.ID).All(&n3))
	require.Equal(t, int64(0), n3[0].C, "residue cage matcher deleted")
}

// TestCarePlanMigrationV3CompletionPassWithoutActiveRules pins the exact
// corner the first production v3 run left behind (2026-10-27): every
// converter rule ALREADY retired — rules_considered == 0 — while the
// retired rows still carry their matcher links. The first shipped build
// early-returned on len(rules)==0, skipped the completion pass, wrote the
// marker and froze the cage matchers into /care_matchers. The completion
// pass must run even with zero active converter rules.
func TestCarePlanMigrationV3CompletionPassWithoutActiveRules(t *testing.T) {
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()
	db := models.DB

	m := &models.CareMatcher{
		Name:       convertedMatcherName("residue only"),
		Expression: fmt.Sprintf(`species IN ("Pigeon biset") AND cage =* "%s"`, fx.f.cage),
	}
	verrs, err := db.ValidateAndCreate(m)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "matcher fixture invalid: %v", verrs)
	r := &models.CareRule{
		Name:          convertedFeedingName("residue only"),
		Description:   nulls.NewString(fmt.Sprintf("Cluster alimentation ×1 [source: %s]", ConverterTag)),
		ActionKind:    careplan.KindFeeding,
		ActionPayload: []byte(`{"caretype_id":"fixture-feed","food":"residue only","force_feed":false}`),
		Schedule:      []byte(`{"times":["10:00"],"every_days":1,"anchor":"intake"}`),
		MatcherID:     uuid.NullUUID{UUID: m.ID, Valid: true},
		Active:        false, // already retired, matcher link left behind
		StopOnOuttake: true,
	}
	verrs, err = db.ValidateAndCreate(r)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "rule fixture invalid: %v", verrs)

	mig, err := RunCarePlanMigration(db)
	require.NoError(t, err)
	require.NotNil(t, mig)
	require.Equal(t, 0, mig.Migration.RulesConsidered, "no active converter rules in the residue corner")
	require.Equal(t, 0, mig.Migration.RulesRetired)
	require.Equal(t, 1, mig.Migration.MatchersDeleted, "completion pass must run when rules_considered == 0")
	require.Equal(t, 0, mig.Migration.MatchersKept)

	var rb models.CareRule
	require.NoError(t, db.Find(&rb, r.ID))
	require.False(t, rb.MatcherID.Valid, "residue rule unlinked")
	require.Equal(t, int64(0), countRows(t, db, "SELECT count(*) as c FROM care_matchers WHERE id = ?", m.ID),
		"cage matcher deleted even with zero active converter rules")
}

// TestCarePlanConverterCaseVariantSeries guards the no-loss mandate (§8.1)
// against collation traps: prod tables use utf8mb4_0900_ai_ci, so a
// case/accent-insensitive guard would treat "ProdiplasT-T" and
// "Prodiplast-T" as the same plan name and silently skip the second
// series. Found during the 2026-09-27 production cutover test on animal
// 10053 (3 future dates lost). Both series must be converted.
func TestCarePlanConverterCaseVariantSeries(t *testing.T) {
	requireMySQLTestDB(t)
	db := models.DB
	cleanupConverterRows(t)

	fx := setupPlanFixture(t)
	defer fx.cleanup()
	defer cleanupConverterRows(t)

	animalID := fx.mkAnimal(t, db, "Hérisson")
	base := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 2)

	// Two series, same animal/dosage/bitmap, drug names differing only by
	// case — two DISTINCT groups in the converter, two DISTINCT plan names
	// that utf8mb4_0900_ai_ci would wrongly consider equal.
	for i, drug := range []string{"ProdiplasT-T", "Prodiplast-T"} {
		for j := 0; j < 2; j++ {
			tr := models.Treatment{
				Date:       base.AddDate(0, 0, i*7+j),
				AnimalID:   animalID,
				Drug:       drug,
				Dosage:     "oreille",
				Timebitmap: models.Treatement_MORNING,
			}
			require.NoError(t, db.Create(&tr))
		}
	}
	defer func() {
		db.RawQuery("DELETE FROM treatments WHERE animal_id = ? AND drug LIKE 'Prodiplas%'", animalID).Exec()
	}()

	report, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	require.NotNil(t, report)

	var names []struct {
		Name string `db:"name"`
	}
	require.NoError(t, db.RawQuery(
		"SELECT name FROM care_animal_plans WHERE animal_id = ? AND created_by IS NULL AND name LIKE 'Soin — Prodiplas%'",
		animalID).All(&names))
	require.Len(t, names, 2,
		"both case-variant series must convert to distinct plans (utf8mb4_0900_ai_ci collision)")
	got := map[string]bool{}
	for _, n := range names {
		got[n.Name] = true
	}
	// B10-6: the series dosage ("oreille") rides in the converted content
	// line; B10-8: unknown drugs convert to CARE plans ("Soin — …").
	require.True(t, got["Soin — ProdiplasT-T (oreille)"])
	require.True(t, got["Soin — Prodiplast-T (oreille)"])

	// Re-run after marker wipe: still exactly two plans (idempotency holds
	// with the binary collation too).
	require.NoError(t, db.RawQuery("DELETE FROM care_plan_conversion").Exec())
	report2, err := RunCarePlanConverter(db)
	require.NoError(t, err)
	require.NotNil(t, report2)
	var c []struct {
		C int64 `db:"c"`
	}
	require.NoError(t, db.RawQuery(
		"SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND created_by IS NULL AND name LIKE 'Soin — Prodiplas%'",
		animalID).All(&c))
	require.Equal(t, int64(2), c[0].C, "idempotent re-run must not duplicate either plan")
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

// TestConvertedPlanNameIsStoredWholeWord (R4-7.24): end-to-end through the
// real writer against the disposable test DB. The unit tests prove the helper
// and the call sites; this proves what actually LANDS in the varchar(200)
// column — where the old byte slice produced a name ending mid-word.
//
// Accented text is deliberate: "é" is 2 bytes, so a byte budget at 60 cuts
// after ~30 characters of accented diet text, well inside a word.
func TestConvertedPlanNameIsStoredWholeWord(t *testing.T) {
	// models.DB points at the disposable creaves_test database.
	tx := models.DB

	fx := setupConverterFixture(t)
	defer fx.f.cleanup()

	marker := uuid.Must(uuid.NewV4()).String()[:8]
	// The marker goes IN THE DIET because that is what composes the name — a
	// unique diet is what makes this plan's name unique, so the idempotency
	// guard inserts instead of short-circuiting on an earlier run.
	diet := "nourriture " + marker + " spéciale pour animaux malades avec un régime très long"

	feedCareID, err := resolveFeedingCaretype(tx)
	require.NoError(t, err)
	require.NotEmpty(t, feedCareID, "the test DB must carry a feeding caretype")

	report := &ConversionReport{coverage: map[int][]careplan.TimeOfDay{}}
	e := feedingEntry{
		AnimalID: fx.animalIDs[0],
		Label:    "cut-" + marker,
		Times:    []careplan.TimeOfDay{{Hour: 8, Minute: 0}},
		Diet:     diet,
		// The marker keeps this name unique, so the idempotency guard inserts
		// rather than short-circuiting on an existing plan.
		Fallback: false,
	}
	require.NoError(t, createConvertedFeedingPlan(tx, report, e, feedCareID))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM care_animal_plans WHERE name LIKE ?", "%"+marker+"%").Exec()
		tx.RawQuery("DELETE FROM care_rules WHERE name LIKE ?", "%"+marker+"%").Exec()
	})

	var rows []struct {
		Name string `db:"name"`
	}
	require.NoError(t, tx.RawQuery(
		"SELECT name FROM care_animal_plans WHERE name LIKE ?", "%"+marker+"%").All(&rows))
	require.NotEmpty(t, rows, "the converted plan must have been stored")

	for _, r := range rows {
		require.True(t, utf8.ValidString(r.Name), "stored name is invalid UTF-8: %q", r.Name)
		require.NotContains(t, r.Name, "�",
			"a split rune reached the database: %q", r.Name)
		require.LessOrEqual(t, utf8.RuneCountInString(r.Name), 200,
			"stored name must fit varchar(200): %q", r.Name)
		// The cut must be MARKED, and marked at a word edge.
		if strings.Contains(r.Name, "…") {
			inner := strings.TrimSuffix(strings.TrimPrefix(r.Name, "Alimentation — "), " (conversion)")
			body := strings.TrimSuffix(inner, "…")
			require.False(t, strings.HasSuffix(body, " "),
				"a space was left dangling before the ellipsis: %q", r.Name)
			require.Equal(t, ' ', []rune(diet)[len([]rune(body))],
				"the stored name was cut mid-word: %q", r.Name)
		} else {
			// Not cut: then the diet must be present WHOLE.
			require.Contains(t, r.Name, diet,
				"an uncut name must carry the complete diet: %q", r.Name)
		}
	}
}

// TestCarePlanFeedingRepairConvergesToLegacyColumns exercises the
// startup_v5 pass (bugs.md BUG-50): a database converted by the PRE-BUG-50
// converter carries (a) stale repeat-form schedules on start==end rows,
// (b) grace-less window rows, (c) a deleted plan (§8.1 hole), (d) zombie
// plans on outtaken animals. The repair must converge every converter-
// owned plan to the legacy columns, create the missing plan, retire the
// zombie, leave human-authored plans untouched — and the read-only
// validation must then report PASS. Idempotent: a second repair is a
// no-op.
func TestCarePlanFeedingRepairConvergesToLegacyColumns(t *testing.T) {
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()
	db := models.DB

	// Convert fresh (current semantics), then SIMULATE the legacy state:
	_, err := RunCarePlanConverter(db)
	require.NoError(t, err)

	// (d) zombie feeder: a converter feeding plan on an animal that later
	// left the §8.1 scope — here: diet emptied (same out-of-scope arm as
	// outtaking; no outtake FK gymnastics needed). The converter is
	// marker-guarded, so wipe the marker to actually convert the new
	// animal (the other animals skip on their name guard).
	zAnimal := fx.f.mkAnimal(t, db, "Pigeon biset")
	fx.setFeeding(t, db, zAnimal, "grains zombies", "09:00", "17:00", 240)
	require.NoError(t, db.RawQuery("DELETE FROM care_plan_conversion WHERE `key` = ?", ConversionMarkerKey).Exec())
	_, err = RunCarePlanConverter(db)
	require.NoError(t, err)
	require.NoError(t, db.RawQuery("UPDATE animals SET feeding = '' WHERE id = ?", zAnimal).Exec())
	var zombiePlans []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", zAnimal, careplan.KindFeeding).All(&zombiePlans))
	require.Len(t, zombiePlans, 1)

	// (a) stale pre-BUG-50 schedule on the start==end animal: the old
	// repeat{from X, to X, every P} form without grace.
	stale := `{"times":["10:00"],"every_days":1,"anchor":"intake","repeat":{"from":"10:00","to":"10:00","every_minutes":600}}`
	var adLib models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.adLibID, careplan.KindFeeding).First(&adLib))
	adLib.Schedule = []byte(stale)
	require.NoError(t, db.Save(&adLib))

	// (b) grace dropped from a window row (pre-BUG-50 JSON).
	var variant models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", fx.animalIDs[8], "%grains pigeons eau%").First(&variant))
	variant.Schedule = []byte(`{"times":["07:00","13:00","19:00"],"every_days":1,"anchor":"intake","repeat":{"from":"07:00","to":"19:00","every_minutes":360}}`)
	require.NoError(t, db.Save(&variant))

	// (c) §8.1 hole: the NULL-times animal loses its plan entirely.
	require.NoError(t, db.RawQuery("DELETE FROM care_animal_plans WHERE animal_id = ?", fx.nullTimesID).Exec())

	// (e) a HUMAN-authored plan must never be touched by the repair. The
	// animal's converter plan is removed so the human plan is the ONLY
	// coverage (HUMAN_ONLY arm).
	require.NoError(t, db.RawQuery("DELETE FROM care_animal_plans WHERE animal_id = ? AND created_by IS NULL", fx.dietOnlyID).Exec())
	human := &models.CareAnimalPlan{
		AnimalID:      fx.dietOnlyID,
		Name:          "Nourrissage gavage main",
		ActionKind:    careplan.KindFeeding,
		ActionPayload: []byte(`{"food":"main d'oeuvre"}`),
		Schedule:      []byte(`{"times":["06:13"],"every_days":1,"anchor":"intake"}`),
		Active:        true,
		CreatedBy:     nulls.NewUUID(fx.f.userID), // a REAL user row (FK) — “a caregiver authored this”
	}
	require.NoError(t, db.Create(human))

	// Read-only validation MUST see the damage (the user gate: failures are
	// reported, not silently "fixed").
	var validation *FeedingValidationReport
	require.NoError(t, db.Transaction(func(tx *pop.Connection) error {
		var err error
		validation, err = ValidateLegacyFeedingConversion(tx, false)
		return err
	}))
	require.Equal(t, 11, validation.AnimalsConsidered, "the zombie feeder is out of scope, not considered")
	require.False(t, validation.Healthy(), "the deleted plan is a MISSING violation")
	// BUG-51: stale alone must also fail the gate (wrong feeding info on
	// the day plan), not just MISSING.
	require.GreaterOrEqual(t, validation.Stale, 2, "adLib + variant schedules are stale vs the columns")
	foundStale, foundMissing, foundZombie := false, false, false
	for _, l := range validation.Lines {
		switch l.Status {
		case fvStale:
			if l.AnimalID == fx.adLibID || l.AnimalID == fx.animalIDs[8] {
				foundStale = true
			}
		case fvMissing:
			if l.AnimalID == fx.nullTimesID {
				foundMissing = true
			}
		case fvZombie:
			if l.AnimalID == zAnimal {
				foundZombie = true
			}
		case fvHumanOnly:
			require.Equal(t, fx.dietOnlyID, l.AnimalID, "human-covered animal: %d", l.AnimalID)
		}
	}
	require.True(t, foundStale, "stale schedules must be reported")
	require.True(t, foundMissing, "the §8.1 hole must be reported")
	require.True(t, foundZombie, "the zombie plan must be reported")

	// The repair converges everything.
	repair, err := RunCarePlanFeedingRepair(db)
	require.NoError(t, err)
	require.NotNil(t, repair)
	require.True(t, repair.Healthy(), "after the repair every legacy trace is covered")
	require.Equal(t, 0, repair.Stale, "write mode reports the POST state: no residual stale")
	require.GreaterOrEqual(t, repair.Repaired, 2, "the two stale schedules are rewritten")
	require.Equal(t, 1, repair.Created, "exactly one plan was created")
	require.Equal(t, 1, repair.ZombiesRetired, "the zombie plan is retired")
	require.Equal(t, 1, repair.HumanOnly, "the human plan counts as covered (converter leaves it alone)")

	// Post-repair state, read back from the DB:
	// (a) the start==end plan carries the plain schedule + grace 300;
	var adLibAfter models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.adLibID, careplan.KindFeeding).First(&adLibAfter))
	// MySQL JSON normalization: compare canonically (see round-trip note).
	require.Equal(t, canonicalSchedule(`{"times":["10:00"],"every_days":1,"anchor":"intake","grace_minutes":300}`), canonicalSchedule(string(adLibAfter.Schedule)))
	require.True(t, adLibAfter.Active)
	// (b) the window row carries its grace again;
	var variantAfter models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", fx.animalIDs[8], "%grains pigeons eau%").First(&variantAfter))
	variantParsed, err := careplan.ParseScheduleJSON(variantAfter.Schedule)
	require.NoError(t, err)
	require.Equal(t, 180, variantParsed.GraceMinutes, "window row grace = 360/2")
	require.NotNil(t, variantParsed.Repeat)
	// (c) the deleted plan is back (fallback row, definitive single daily
	// slot — user ruling 2026-10-14: no "à vérifier" flag);
	var recreated []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.nullTimesID, careplan.KindFeeding).All(&recreated))
	require.Len(t, recreated, 1, "the §8.1 hole is closed")
	require.NotContains(t, recreated[0].Name, "à vérifier")
	// (d) the zombie is inactive;
	var zombieAfter models.CareAnimalPlan
	require.NoError(t, db.Where("id = ?", zombiePlans[0].ID).First(&zombieAfter))
	require.False(t, zombieAfter.Active)
	// (e) the human plan is untouched (canonical equality — the JSON column
	// normalizes bytes on insert; "untouched" means same DOC, not same bytes).
	var humanAfter models.CareAnimalPlan
	require.NoError(t, db.Where("id = ?", human.ID).First(&humanAfter))
	require.Equal(t, canonicalSchedule(string(human.Schedule)), canonicalSchedule(string(humanAfter.Schedule)))
	require.True(t, humanAfter.Active)

	// Idempotency: second repair = no-op (marker).
	require.NoError(t, db.RawQuery("DELETE FROM care_plan_conversion WHERE `key` = ?", ConversionMarkerKeyV5).Exec())
	second, err := RunCarePlanFeedingRepair(db)
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, 0, second.Repaired, "no residual drift")
	require.Equal(t, 0, second.Missing)
	require.Equal(t, 0, second.ZombiesRetired, "the zombie is already inactive")
	require.Equal(t, 1, second.HumanOnly, "the human plan is still left alone")

	// The read-only validation now PASSES (missing == 0, no stale lines).
	var final *FeedingValidationReport
	require.NoError(t, db.Transaction(func(tx *pop.Connection) error {
		var err error
		final, err = ValidateLegacyFeedingConversion(tx, false)
		return err
	}))
	require.True(t, final.Healthy())
	require.Equal(t, 0, final.Stale, "BUG-51: the read-only gate passes only with zero stale schedules")
	require.Equal(t, 0, final.Missing)
	for _, l := range final.Lines {
		require.NotEqual(t, fvStale, l.Status, "no stale converter schedule may survive: %+v", l)
		require.NotEqual(t, fvMissing, l.Status, "no §8.1 hole may survive: %+v", l)
	}
}

// TestCarePlanTreatmentValidationAndRefresh (bugs.md BUG-52): the
// treatment conversion is one-shot at startup_v2 time — a series
// prescribed AFTER conversion silently had no plan (miscare risk: the
// medication never shows on the day plan). Pin the coverage gate
// (ValidateTreatmentConversion) and the marker-free boot refresh.
func TestCarePlanTreatmentValidationAndRefresh(t *testing.T) {
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()
	db := models.DB

	// Fresh conversion — the fixture carries four treatment series
	// (medication, unknown-drug observation, wound-care observation,
	// no-dosage observation).
	_, err := RunCarePlanConverter(db)
	require.NoError(t, err)

	// TP-2a: a freshly converted DB is healthy — every series covered by
	// an ACTIVE converter plan.
	validate := func() *TreatmentValidationReport {
		var r *TreatmentValidationReport
		require.NoError(t, db.Transaction(func(tx *pop.Connection) error {
			var err error
			r, err = ValidateTreatmentConversion(tx)
			return err
		}))
		return r
	}
	base := validate()
	require.True(t, base.Healthy(), "fresh conversion: %+v", base.Lines)
	require.GreaterOrEqual(t, base.Series, 4)
	require.Equal(t, 0, base.Missing)
	require.Equal(t, 0, base.Inactive)
	require.Equal(t, 0, base.Orphans)
	require.Equal(t, base.Series, base.OK, "no dedupe surprises between series and plans")

	// TP-2b: a DELETED converter plan is a MISSING violation — the gate
	// must catch it (this is the miscare case).
	var medPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.animalIDs[len(fx.animalIDs)-4], careplan.KindMedication).First(&medPlan))
	require.NoError(t, db.RawQuery("DELETE FROM care_animal_plans WHERE id = ?", medPlan.ID).Exec())
	gone := validate()
	require.False(t, gone.Healthy(), "a deleted medication plan is a coverage hole")
	require.Equal(t, 1, gone.Missing)

	// TP-2c: the boot refresh (marker-free re-run of the idempotent
	// conversion) recreates it; a second refresh adds nothing.
	created, err := RunCarePlanTreatmentRefresh(db)
	require.NoError(t, err)
	require.Equal(t, 1, created)
	afterRefresh := validate()
	require.True(t, afterRefresh.Healthy(), "%+v", afterRefresh.Lines)
	created, err = RunCarePlanTreatmentRefresh(db)
	require.NoError(t, err)
	require.Equal(t, 0, created, "idempotent: no duplicate series conversion")
	require.True(t, validate().Healthy())

	// TP-2d: THE regression — a treatment prescribed AFTER conversion
	// (post-conversion row) must be picked up by the refresh, not
	// silently ignored until the end of time. A NEW drug (distinct
	// routed name) keeps the series a distinct coverage unit; same
	// drug+animal would legitimately dedupe onto the existing plan name.
	drug2 := models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "Metacam 1,5 mg/ml"}
	require.NoError(t, db.Create(&drug2))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug2.ID).Exec()
	})
	tomorrow := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 1)
	newSeries := models.Treatment{
		Date:       tomorrow,
		AnimalID:   fx.animalIDs[len(fx.animalIDs)-4],
		Drug:       "Metacam 1,5 mg/ml",
		Dosage:     "0,2 ml",
		Timebitmap: models.Treatement_MORNING,
	}
	require.NoError(t, db.Create(&newSeries))
	late := validate()
	require.False(t, late.Healthy(), "a post-conversion prescription is uncovered until refreshed")
	require.Equal(t, 1, late.Missing)
	created, err = RunCarePlanTreatmentRefresh(db)
	require.NoError(t, err)
	require.Equal(t, 1, created, "the refresh converts the new series")
	require.True(t, validate().Healthy())

	// TP-2e: a DEACTIVATED converter plan (vet stopped the treatment) is
	// reported as INACTIVE — visible, but a deliberate human decision,
	// so it does not flip Healthy.
	var stopped models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.animalIDs[len(fx.animalIDs)-4], careplan.KindMedication).First(&stopped))
	require.True(t, stopped.Active)
	stopped.Active = false
	require.NoError(t, db.Save(&stopped))
	inactive := validate()
	require.True(t, inactive.Healthy(), "deactivation is a human stop decision, not a conversion hole")
	require.Equal(t, 1, inactive.Inactive)
	// The refresh must NOT re-activate it (it respects the stop).
	created, err = RunCarePlanTreatmentRefresh(db)
	require.NoError(t, err)
	require.Equal(t, 0, created)
	var stillStopped models.CareAnimalPlan
	require.NoError(t, db.Where("id = ?", stopped.ID).First(&stillStopped))
	require.False(t, stillStopped.Active, "the refresh never overrides a deliberate deactivation")

	// TP-2f: orphan plans (series aged out / edited away) are
	// informational only. Payload cloned from the real observation plan
	// (known-drug-no-dosage arm) so model validation passes.
	var obsPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.trNoDosageID, careplan.KindObservation).First(&obsPlan))
	var woundPlan models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND action_kind = ?", fx.trWoundID, careplan.KindCare).First(&woundPlan))
	orphan := &models.CareAnimalPlan{
		AnimalID:      fx.trWoundID,
		Name:          "Traitement — fantôme",
		ActionKind:    careplan.KindObservation,
		ActionPayload: obsPlan.ActionPayload,
		Schedule:      woundPlan.Schedule,
		Active:        true,
	}
	require.NoError(t, db.Create(orphan))
	w := validate()
	require.Equal(t, 1, w.Orphans)
	require.True(t, w.Healthy(), "orphans never flip the gate")
	require.NoError(t, db.RawQuery("DELETE FROM care_animal_plans WHERE id = ?", orphan.ID).Exec())
}

// TestCarePlanTreatmentCadence pins the BUG-52 exact-conversion contract
// (user ruling 2026-10-14: a conversion is always correct, never flagged
// "à vérifier"): a regularly spaced series (constant gap G days) converts
// to every_days:G with duration_days = date count; a weekly weekday
// pattern converts to a weekdays schedule; an irregular series (mixed
// gaps) becomes ONE exact single-day plan per legacy date; a single-date
// series is unchanged.
func TestCarePlanTreatmentCadence(t *testing.T) {
	requireMySQLTestDB(t)
	db := models.DB
	cleanupConverterRows(t)
	fx := setupConverterFixture(t)
	defer fx.f.cleanup()

	drug := models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "Metacam 1,5 mg/ml"}
	require.NoError(t, db.Create(&drug))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})

	base := time.Now().Truncate(24*time.Hour).AddDate(0, 0, 1)
	mkSeries := func(animalID int, offsets ...int) {
		for _, off := range offsets {
			tr := models.Treatment{
				Date:       base.AddDate(0, 0, off),
				AnimalID:   animalID,
				Drug:       "Metacam 1,5 mg/ml",
				Dosage:     "0,2 ml",
				Timebitmap: models.Treatement_MORNING,
			}
			require.NoError(t, db.Create(&tr))
		}
	}

	// (a) regular every-3-days series (the animal-9577 bandage shape).
	idRegular := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	mkSeries(idRegular, 0, 3, 6, 9)
	// (b) irregular series: mixed gaps (1 and 8) — no single step fits.
	idIrregular := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	mkSeries(idIrregular, 0, 1, 9)
	// (c) single-date series — unchanged behavior.
	idSingle := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	mkSeries(idSingle, 2)
	// (d) weekly weekday pattern: Mon/Wed/Fri for two weeks — converts to
	// ONE plan restricted to weekdays [1,3,5], generating exactly the 6
	// legacy dates.
	idWeekdays := fx.f.mkAnimal(t, db, "Faucon crécerelle")
	daysToMonday := (8 - int(base.Weekday())) % 7
	mon1 := base.AddDate(0, 0, daysToMonday)
	mkSeries(idWeekdays, daysToMonday, daysToMonday+2, daysToMonday+4, daysToMonday+7, daysToMonday+9, daysToMonday+11)
	wantMWF := []string{}
	for _, off := range []int{0, 2, 4, 7, 9, 11} {
		wantMWF = append(wantMWF, mon1.AddDate(0, 0, off).Format("2006-01-02"))
	}

	created, err := RunCarePlanTreatmentRefresh(db)
	require.NoError(t, err)
	require.GreaterOrEqual(t, created, 3)

	read := func(animalID int) models.CareAnimalPlan {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND created_by IS NULL AND action_kind = ?",
			animalID, careplan.KindMedication).First(&p))
		return p
	}

	// (a) regular: every_days 3, duration 3 (generated days), occurrences
	// land exactly on the legacy dates.
	pRegular := read(idRegular)
	require.Equal(t, "Traitement — Metacam 1,5 mg/ml", pRegular.Name)
	sched, err := careplan.ParseScheduleJSON(pRegular.Schedule)
	require.NoError(t, err)
	require.Equal(t, 3, sched.EveryDays)
	require.Equal(t, 4, sched.DurationDays, "generated-day count = the 4 legacy dates")
	occs := careplan.GenerateOccurrences(careplan.NewSource(careplan.SourceAnimal, pRegular.ID.String(), pRegular.Name, pRegular.ActionKind, sched),
		&careplan.AnimalContext{ID: idRegular}, base.AddDate(0, 0, -1), base.AddDate(0, 0, 40))
	gotDays := []string{}
	for _, o := range occs {
		gotDays = append(gotDays, o.DueAt.Format("2006-01-02"))
	}
	wantDays := []string{
		base.Format("2006-01-02"), base.AddDate(0, 0, 3).Format("2006-01-02"),
		base.AddDate(0, 0, 6).Format("2006-01-02"), base.AddDate(0, 0, 9).Format("2006-01-02"),
	}
	require.Equal(t, wantDays, gotDays, "regular series must produce EXACTLY the legacy dates (BUG-52: no daily over-schedule)")

	// (b) irregular: ONE exact single-day plan per legacy date — the
	// legacy dates ARE the truth; no flag, no over-schedule, no hidden dose.
	var pIrregulars []models.CareAnimalPlan
	require.NoError(t, db.Where("animal_id = ? AND created_by IS NULL AND action_kind = ?",
		idIrregular, careplan.KindMedication).Order("name").All(&pIrregulars))
	require.Len(t, pIrregulars, 3, "mixed-gap series → one exact plan per legacy date")
	wantIrregularDates := []string{
		base.Format("2006-01-02"), base.AddDate(0, 0, 1).Format("2006-01-02"),
		base.AddDate(0, 0, 9).Format("2006-01-02"),
	}
	for i, p := range pIrregulars {
		require.Equal(t, "Traitement — Metacam 1,5 mg/ml — "+wantIrregularDates[i], p.Name,
			"per-date plan name carries the exact legacy date, never a flag")
		schedB, err := careplan.ParseScheduleJSON(p.Schedule)
		require.NoError(t, err)
		require.Equal(t, 1, schedB.EveryDays)
		require.Equal(t, 1, schedB.DurationDays)
		require.NotNil(t, schedB.AnchorDate)
		require.Equal(t, wantIrregularDates[i], schedB.AnchorDate.Format("2006-01-02"))
		occsB := careplan.GenerateOccurrences(careplan.NewSource(careplan.SourceAnimal, p.ID.String(), p.Name, p.ActionKind, schedB),
			&careplan.AnimalContext{ID: idIrregular}, base.AddDate(0, 0, -1), base.AddDate(0, 0, 40))
		require.Len(t, occsB, 1, "single-day plan generates exactly one occurrence")
		require.Equal(t, wantIrregularDates[i], occsB[0].DueAt.Format("2006-01-02"))
	}

	// (d) weekday pattern: single plan, weekdays [1,3,5], duration 6 —
	// occurrences land exactly on the legacy dates.
	pWeekdays := read(idWeekdays)
	require.Equal(t, "Traitement — Metacam 1,5 mg/ml", pWeekdays.Name)
	schedD, err := careplan.ParseScheduleJSON(pWeekdays.Schedule)
	require.NoError(t, err)
	require.Equal(t, 1, schedD.EveryDays)
	require.Equal(t, []int{1, 3, 5}, schedD.Weekdays)
	require.Equal(t, 6, schedD.DurationDays)
	require.NotNil(t, schedD.AnchorDate)
	require.Equal(t, mon1.Format("2006-01-02"), schedD.AnchorDate.Format("2006-01-02"))
	occsD := careplan.GenerateOccurrences(careplan.NewSource(careplan.SourceAnimal, pWeekdays.ID.String(), pWeekdays.Name, pWeekdays.ActionKind, schedD),
		&careplan.AnimalContext{ID: idWeekdays}, base.AddDate(0, 0, -1), base.AddDate(0, 0, 40))
	gotMWFDays := []string{}
	for _, o := range occsD {
		gotMWFDays = append(gotMWFDays, o.DueAt.Format("2006-01-02"))
	}
	require.Equal(t, wantMWF, gotMWFDays, "weekday pattern must generate EXACTLY the legacy dates")

	// (c) single date: plain daily one-day plan, no flag.
	pSingle := read(idSingle)
	require.NotContains(t, pSingle.Name, "(à vérifier)")
	schedC, err := careplan.ParseScheduleJSON(pSingle.Schedule)
	require.NoError(t, err)
	require.Equal(t, 1, schedC.EveryDays)
	require.Equal(t, 1, schedC.DurationDays)

	// Validator stays healthy on all three (coverage is cadence-agnostic).
	report, err := ValidateTreatmentConversion(db)
	require.NoError(t, err)
	require.True(t, report.Healthy(), "lines: %+v", report.Lines)

	// Reconvert (BUG-52 one-shot repair path): deletes converter plans and
	// rebuilds them — same names, same schedules, human rows untouched.
	require.NoError(t, db.Transaction(func(tx *pop.Connection) error {
		if err := tx.RawQuery("DELETE FROM care_animal_plans WHERE created_by IS NULL AND action_kind <> ?", careplan.KindFeeding).Exec(); err != nil {
			return err
		}
		return ReconvertTreatmentSeries(tx, &ConversionReport{})
	}))
	pAgain := read(idRegular)
	require.Equal(t, string(pRegular.Schedule), string(pAgain.Schedule), "reconvert must rebuild the identical cadence")
}
