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
	// 9 feeding animals = 5 cluster-diet + 2 unique diets + 1 empty-diet
	// fallback + 1 schedule variant.
	require.Equal(t, 0, report.Feeding.RulesCreated,
		"v3: the converter must not create rules — per-animal plans only")
	require.Equal(t, 9, report.Feeding.PlansCreated)
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
	dietSlots := []string{"08:00", "10:00", "12:00", "14:00", "16:00", "18:00"}
	for _, id := range fx.animalIDs[:5] {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", id, "Alimentation — grains pigeons eau%").First(&p))
		require.True(t, p.Active)
		slots, err := parseScheduleForTest(p.Schedule)
		require.NoError(t, err)
		require.Equal(t, dietSlots, slots, "animal %d slots", id)
		var payload struct {
			Food string `json:"food"`
		}
		require.NoError(t, json.Unmarshal(p.ActionPayload, &payload))
		require.Equal(t, "grains pigeons eau", payload.Food)
	}

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
	// B10-8: unknown-drug and wound-care series convert to CARE plans
	// typed "Soin" (the legacy drug column carried non-drug entries); a
	// KNOWN drug without posology stays a flagged observation.
	assertConvertedPlan := func(animalID int, nameLike, wantKind string) models.CareAnimalPlan {
		var p models.CareAnimalPlan
		require.NoError(t, db.Where("animal_id = ? AND name LIKE ?", animalID, nameLike).First(&p))
		require.Contains(t, p.Name, "(à vérifier)")
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
	assertConvertedPlan(fx.animalIDs[10], "Soin — Produit inconnu XYZ%", careplan.KindCare)
	assertConvertedPlan(fx.animalIDs[11], "Soin de plaie (conversion)%", careplan.KindCare)
	assertConvertedPlan(fx.animalIDs[12], "Traitement — Ivomec 1% (à vérifier)", careplan.KindObservation)

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
	ruleName := convertedFeedingName(diet, false)
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
		Name:          convertedFeedingName("Mixture A", false),
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
		Name:          convertedFeedingName("residue diet", false),
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
		Name:          convertedFeedingName("residue only", false),
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
	require.True(t, got["Soin — ProdiplasT-T (oreille) (à vérifier)"])
	require.True(t, got["Soin — Prodiplast-T (oreille) (à vérifier)"])

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
