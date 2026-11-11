package actions

// startup_v5 feeding repair + validation pass (bugs.md BUG-50, 2026-10-27
// user directive: "Review conversion to ensure correctness … All animals
// must be validated with original information!").
//
// The BUG-36 converter stored zero-length legacy windows (start == end,
// the "à disposition" grain cages) as repeat{from X, to X, every P} —
// animal 10231 rendered it verbatim in the protocol editor as
// "from 10:00 to 10:00 every 10:00". The BUG-50 converter semantics turn
// those rows into a plain single-slot schedule (identical legacy runtime
// behavior) and carry the legacy lateness cadence (period/2) as
// grace_minutes.
//
// This pass makes EXISTING databases converge to the fixed semantics:
// every in-care animal with a legacy feeding trace is compared against
// the plan the current converter would produce, and converter-owned
// feeding plans (created_by IS NULL — the §8.2 rollback family, which
// includes the v3 rule→plan migration rows) are brought back in line.
// Human-authored plans (created_by NOT NULL) are NEVER touched — a
// caregiver's edit outranks the converter.
//
// The same comparison runs read-only via `buffalo task
// careplan:validate-feeding` and is persisted with the startup_v5 marker
// report, so the "all animals validated against the original columns"
// gate is recorded, not claimed.

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
)

// ConversionMarkerKeyV5 gates the feeding repair pass.
const ConversionMarkerKeyV5 = "startup_v5"

// Feeding validation statuses (one line per animal or zombie plan).
const (
	fvOK        = "OK"
	fvStale     = "STALE"      // converter plan present, schedule/payload differs — repairable
	fvMissing   = "MISSING"    // legacy trace but no feeding plan at all — §8.1 violation
	fvHumanOnly = "HUMAN_ONLY" // no converter plan; a caregiver-authored plan covers the animal
	fvRetired   = "RETIRED"    // stale converter plan retired by the repair
	fvCreated   = "CREATED"    // missing plan created by the repair
	fvZombie    = "ZOMBIE"     // converter plan on an outtaken/empty-diet animal
)

// FeedingValidationLine is one per-animal validation outcome.
type FeedingValidationLine struct {
	AnimalID int    `json:"animal_id"`
	Label    string `json:"label"`
	Status   string `json:"status"`
	Detail   string `json:"detail,omitempty"`
}

// FeedingValidationReport is the per-animal §8.1 gate: EVERY in-care
// animal with a legacy feeding trace against the plan the current
// converter semantics produce. Persisted with the startup_v5 marker.
type FeedingValidationReport struct {
	GeneratedAt       string                  `json:"generated_at"`
	AnimalsConsidered int                     `json:"animals_considered"`
	OK                int                     `json:"ok"`
	Repaired          int                     `json:"repaired"`
	Stale             int                     `json:"stale"`
	Created           int                     `json:"created"`
	RetiredStale      int                     `json:"retired_stale"`
	ZombiesRetired    int                     `json:"zombies_retired"`
	Missing           int                     `json:"missing"`
	HumanOnly         int                     `json:"human_only"`
	Lines             []FeedingValidationLine `json:"lines,omitempty"`
}

// Healthy reports whether the conversion state matches the legacy columns
// (the user gate: "All animals must be validated with original
// information"). MISSING is a violation; STALE (read-only mode: the
// stored schedule still differs from the columns — wrong feeding info on
// the day plan) is ALSO a violation. In write mode the repair zeroes
// Stale, so the persisted boot report reflects the post-pass state.
func (r *FeedingValidationReport) Healthy() bool { return r.Missing == 0 && r.Stale == 0 }

// canonicalSchedule renders a §4.3 document through the package
// marshaler so STORAGE-normalized JSON (the schedule column is a MySQL
// JSON type: keys sorted, spaces added) compares equal to a freshly
// built document. Non-parsing input comes back verbatim (the mismatch
// then flags it — repair rewrites it).
func canonicalSchedule(raw string) string {
	b, err := careplan.NormalizeScheduleJSON([]byte(raw))
	if err != nil {
		return raw
	}
	return string(b)
}

// expectedConvertedFeeding computes what the CURRENT converter semantics
// produce for one legacy row: the plan name and the §4.3 schedule JSON.
// Single source shared by the converter, the repair pass and the
// validation gate so the three can never disagree again.
func expectedConvertedFeeding(a models.Animal, feedCareID string) (name, schedule string) {
	times, repeat, fallback := legacyFeedingSchedule(a)
	diet := NormalizeDiet(a.Feeding.String)
	entry := feedingEntry{
		AnimalID:     a.ID,
		Label:        animalLabel(a),
		Times:        times,
		Repeat:       repeat,
		Force:        a.ForceFeed,
		Diet:         diet,
		GraceMinutes: legacyFeedingGraceMinutes(a.FeedingPeriod),
		Fallback:     fallback,
	}
	return convertedFeedingName(diet),
		buildConvertedScheduleJSON(entry.Times, entry.Repeat, entry.GraceMinutes)
}

// ValidateLegacyFeedingConversion compares EVERY in-care animal with a
// legacy feeding trace (plus converter-owned plans on out-of-scope
// animals) against the plan the current converter semantics produce.
// write=false is the read-only validation gate; write=true is the
// startup_v5 repair: stale converter schedules are updated, missing plans
// created, zombie plans (outtaken / empty-diet animals) retired. Plans
// with a human author are never modified.
func ValidateLegacyFeedingConversion(tx *pop.Connection, write bool) (*FeedingValidationReport, error) {
	report := &FeedingValidationReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	feedCareID, err := resolveFeedingCaretype(tx)
	if err != nil {
		return nil, err
	}

	// Same selection as the converter (§8.1 no-loss scope): in-care,
	// non-empty diet, any feeding trace.
	var animals []models.Animal
	if err := tx.Where("outtake_id IS NULL AND feeding IS NOT NULL AND TRIM(feeding) <> '' AND (feeding_period > 0 OR feeding_start IS NOT NULL OR feeding_end IS NOT NULL OR TRIM(feeding) <> '')").All(&animals); err != nil {
		return nil, err
	}
	report.AnimalsConsidered = len(animals)

	for i := range animals {
		wantName, wantSchedule := expectedConvertedFeeding(animals[i], feedCareID)
		if err := validateOneFeedingAnimal(tx, write, &animals[i], wantName, wantSchedule, feedCareID, report); err != nil {
			return nil, err
		}
	}

	if err := retireZombieFeedingPlans(tx, write, report); err != nil {
		return nil, err
	}
	return report, nil
}

// validateOneFeedingAnimal classifies ONE animal against its expected
// converter plan and applies the write-mode consequences.
func validateOneFeedingAnimal(tx *pop.Connection, write bool, a *models.Animal, wantName, wantSchedule, feedCareID string, report *FeedingValidationReport) error {
	wantCanonical := canonicalSchedule(wantSchedule)

	// Converter-owned active feeding plans of this animal (the §8.2
	// family: created_by IS NULL), any name — catches diet-drift
	// duplicates as well as the expected plan.
	var plans models.CareAnimalPlans
	if err := tx.Where("animal_id = ? AND action_kind = ? AND created_by IS NULL AND active = ?", a.ID, careplan.KindFeeding, true).All(&plans); err != nil {
		return err
	}
	// Human-authored active feeding plans — a caregiver's protocol
	// outranks the converter; the animal then counts as covered.
	humanPlans, err := hasHumanFeedingPlan(tx, a.ID)
	if err != nil {
		return err
	}

	var expected *models.CareAnimalPlan
	stale := []*models.CareAnimalPlan{}
	for i := range plans {
		if plans[i].Name == wantName {
			expected = &plans[i]
		} else {
			stale = append(stale, &plans[i])
		}
	}

	label := animalLabel(*a)
	switch {
	case expected != nil && canonicalSchedule(string(expected.Schedule)) == wantCanonical:
		return settleFeedingDuplicate(tx, write, expected, stale, a.ID, label, report)
	case expected != nil:
		return repairStaleFeedingPlan(tx, write, expected, stale, a, wantSchedule, feedCareID, label, report)
	case humanPlans:
		report.HumanOnly++
		report.Lines = append(report.Lines, FeedingValidationLine{a.ID, label, fvHumanOnly,
			"covered by a caregiver-authored plan"})
		return nil
	default:
		return createMissingFeedingPlan(tx, write, a, label, feedCareID, report)
	}
}

// hasHumanFeedingPlan reports whether an ACTIVE human-authored feeding
// plan covers the animal.
func hasHumanFeedingPlan(tx *pop.Connection, animalID int) (bool, error) {
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND action_kind = ? AND created_by IS NOT NULL AND active = ?",
		animalID, careplan.KindFeeding, true).All(&n); err != nil {
		return false, err
	}
	return len(n) > 0 && n[0].C > 0, nil
}

// settleFeedingDuplicate handles a canonically-correct expected plan:
// retire same-author duplicates with other names (diet edited after
// conversion) — the expected plan already covers the animal.
func settleFeedingDuplicate(tx *pop.Connection, write bool, expected *models.CareAnimalPlan, stale []*models.CareAnimalPlan, animalID int, label string, report *FeedingValidationReport) error {
	for _, sp := range stale {
		if write {
			sp.Active = false
			if err := tx.Save(sp); err != nil {
				return err
			}
		}
		report.Lines = append(report.Lines, FeedingValidationLine{animalID, label, fvRetired,
			fmt.Sprintf("stale duplicate %s (diet drift)", sp.Name)})
	}
	if len(stale) > 0 {
		report.RetiredStale += len(stale)
		return nil
	}
	report.OK++
	report.Lines = append(report.Lines, FeedingValidationLine{animalID, label, fvOK, ""})
	return nil
}

// repairStaleFeedingPlan rewrites a stale converter schedule (+ payload)
// to the current semantics and retires its same-author duplicates.
func repairStaleFeedingPlan(tx *pop.Connection, write bool, expected *models.CareAnimalPlan, stale []*models.CareAnimalPlan, a *models.Animal, wantSchedule, feedCareID, label string, report *FeedingValidationReport) error {
	if write {
		expected.Schedule = []byte(wantSchedule)
		expected.ActionPayload = buildSeedPayload(seedRuleDef{
			Kind: careplan.KindFeeding, PayloadFood: NormalizeDiet(a.Feeding.String), ForceFeed: a.ForceFeed,
		}, feedCareID)
		if err := tx.Save(expected); err != nil {
			return err
		}
		for _, sp := range stale {
			sp.Active = false
			if err := tx.Save(sp); err != nil {
				return err
			}
		}
	} else {
		// Read-only gate: the drift REMAINS on the animal — it must fail
		// Healthy (BUG-51: a stale schedule is wrong feeding info on the
		// day plan).
		report.Stale++
	}
	report.Repaired++
	report.Lines = append(report.Lines, FeedingValidationLine{a.ID, label, fvStale,
		fmt.Sprintf("schedule → %s", wantSchedule)})
	return nil
}

// createMissingFeedingPlan closes a §8.1 violation: a legacy feeding
// trace without ANY feeding plan. In write mode the plan is created like
// the converter would; the report counts the hole FOUND and — in write
// mode — closed, so Healthy() reflects the POST-pass state.
func createMissingFeedingPlan(tx *pop.Connection, write bool, a *models.Animal, label, feedCareID string, report *FeedingValidationReport) error {
	if write {
		entry := feedingEntry{
			AnimalID:     a.ID,
			Label:        label,
			Force:        a.ForceFeed,
			Diet:         NormalizeDiet(a.Feeding.String),
			GraceMinutes: legacyFeedingGraceMinutes(a.FeedingPeriod),
		}
		entry.Times, entry.Repeat, entry.Fallback = legacyFeedingSchedule(*a)
		if err := createConvertedFeedingPlan(tx, &ConversionReport{coverage: map[int][]careplan.TimeOfDay{}}, entry, feedCareID); err != nil {
			return err
		}
		report.Created++
		report.Missing--
	}
	report.Missing++
	report.Lines = append(report.Lines, FeedingValidationLine{a.ID, label, fvMissing,
		"legacy trace without a feeding plan"})
	return nil
}

// retireZombieFeedingPlans is the inverse no-loss check: converter-owned
// ACTIVE feeding plans on animals out of the §8.1 scope (outtaken, or
// empty diet) — zombies that keep showing on the day plan for nothing.
func retireZombieFeedingPlans(tx *pop.Connection, write bool, report *FeedingValidationReport) error {
	var zombies []models.CareAnimalPlan
	if err := tx.RawQuery(
		"SELECT p.* FROM care_animal_plans p JOIN animals a ON a.id = p.animal_id "+
			"WHERE p.action_kind = ? AND p.created_by IS NULL AND p.active = ? "+
			"AND (a.outtake_id IS NOT NULL OR a.feeding IS NULL OR TRIM(a.feeding) = '')",
		careplan.KindFeeding, true).All(&zombies); err != nil {
		return err
	}
	for i := range zombies {
		if write {
			zombies[i].Active = false
			if err := tx.Save(&zombies[i]); err != nil {
				return err
			}
		}
		report.ZombiesRetired++
		report.Lines = append(report.Lines, FeedingValidationLine{zombies[i].AnimalID, "", fvZombie,
			fmt.Sprintf("converter plan %s on an out-of-scope animal", zombies[i].Name)})
	}
	return nil
}

// RunCarePlanFeedingRepairAtBoot runs the startup_v5 repair against
// models.DB. A finished marker makes it a no-op; any error aborts the
// boot (cmd/app/main.go) so a half-repaired state never serves.
func RunCarePlanFeedingRepairAtBoot() error {
	db := models.DB
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV5)
	if err != nil {
		return err
	}
	if done {
		log.Printf("care_plan_feeding_repair: marker %q present — no-op", ConversionMarkerKeyV5)
		return nil
	}
	report, err := RunCarePlanFeedingRepair(db)
	if err != nil {
		return err
	}
	log.Printf("care_plan_feeding_repair: done — %d animals considered, ok %d, repaired %d, created %d, retired %d, zombies %d, missing %d, human-only %d",
		report.AnimalsConsidered, report.OK, report.Repaired, report.Created, report.RetiredStale, report.ZombiesRetired, report.Missing, report.HumanOnly)
	return nil
}

// RunCarePlanFeedingRepair executes the startup_v5 pass in one
// transaction and writes the marker with the persisted report on success.
func RunCarePlanFeedingRepair(db *pop.Connection) (*FeedingValidationReport, error) {
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV5)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, nil
	}
	var report *FeedingValidationReport
	if err := db.Transaction(func(tx *pop.Connection) error {
		var err error
		report, err = ValidateLegacyFeedingConversion(tx, true)
		return err
	}); err != nil {
		return nil, err
	}
	if err := db.Transaction(func(tx *pop.Connection) error {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		return tx.RawQuery(
			"INSERT INTO care_plan_conversion (`key`, finished_at, report, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW()) "+
				"ON DUPLICATE KEY UPDATE finished_at = VALUES(finished_at), report = VALUES(report), updated_at = NOW()",
			ConversionMarkerKeyV5, time.Now(), string(raw),
		).Exec()
	}); err != nil {
		return nil, err
	}
	return report, nil
}
