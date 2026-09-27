package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Startup converter of docs/care-expert.md §8.1: a single-shot, idempotent
// data migration that converts the legacy feeding/treatment scheduling rows
// into care-plan rules and animal plans. It runs on application boot, after
// migrations, before the HTTP server accepts requests (see cmd/app/main.go);
// a failure aborts the boot so a half-converted database can never serve.
//
// Idempotency: the care_plan_conversion row key='startup_v1' is written on
// success; on later boots the converter is a no-op. Deleting the marker
// re-runs it; generated rows are guarded by name-existence checks and all
// carry the ConverterTag marker in their description so rollback (§8.2) can
// identify them.

// ConversionMarkerKey is the §8.1 marker primary key.
const ConversionMarkerKey = "startup_v1"

// FeedingClusterThreshold is the §8.1 step-2 cluster size at which a shared
// diet becomes a generic rule instead of per-animal plans (≥5; calibrated
// on §2.4 cohort counts).
const FeedingClusterThreshold = 5

// ---------------------------------------------------------------------------
// Report (§8.1 step 5)
// ---------------------------------------------------------------------------

// ConversionLine is one per-animal report line (converted or skipped).
type ConversionLine struct {
	AnimalID int    `json:"animal_id"`
	Label    string `json:"label"`
	Reason   string `json:"reason"`
}

// ConversionReport aggregates the converter outcome; persisted as JSON in
// the marker row and logged at boot.
type ConversionReport struct {
	GeneratedAt string `json:"generated_at"`
	Seeds       struct {
		MatchersInserted int      `json:"matchers_inserted"`
		MatchersSkipped  int      `json:"matchers_skipped"`
		RulesInserted    int      `json:"rules_inserted"`
		RulesSkipped     int      `json:"rules_skipped"`
		Skipped          []string `json:"skipped,omitempty"`
	} `json:"seeds"`
	Feeding struct {
		AnimalsConsidered int              `json:"animals_considered"`
		Clusters          int              `json:"clusters"`
		RulesCreated      int              `json:"rules_created"`
		PlansCreated      int              `json:"plans_created"`
		Lines             []ConversionLine `json:"lines,omitempty"`
	} `json:"feeding"`
	Treatments struct {
		Series       int              `json:"series"`
		PlansCreated int              `json:"plans_created"`
		Lines        []ConversionLine `json:"lines,omitempty"`
	} `json:"treatments"`
	// Reconciliation is the §8.1 no-loss gate: per-animal comparison of the
	// legacy schedule vs the converted rule/plan. UNCOVERED must be 0 before
	// the rollout is approved (bugs.md Phase 1.6).
	Reconciliation struct {
		FeedingOK          int              `json:"feeding_ok"`
		FeedingDegraded    int              `json:"feeding_degraded"`
		FeedingUncovered   int              `json:"feeding_uncovered"`
		TreatmentOK        int              `json:"treatment_ok"`
		TreatmentUncovered int              `json:"treatment_uncovered"`
		Lines              []ConversionLine `json:"lines,omitempty"`
	} `json:"reconciliation"`
	// coverage maps animal_id → converted slot list (in-memory only, used by
	// the reconciliation step; not serialized).
	coverage map[int][]careplan.TimeOfDay
}

func (r *ConversionReport) addSkip(kind string, l ConversionLine) {
	switch kind {
	case "feeding":
		r.Feeding.Lines = append(r.Feeding.Lines, l)
	case "treatment":
		r.Treatments.Lines = append(r.Treatments.Lines, l)
	}
}

// ---------------------------------------------------------------------------
// Entry points
// ---------------------------------------------------------------------------

// RunCarePlanConverterAtBoot runs the §8.1 conversion against models.DB.
// A finished marker makes it a no-op; any error is returned so the caller
// (cmd/app/main.go) can abort the boot.
func RunCarePlanConverterAtBoot() error {
	db := models.DB
	markerDone, err := conversionMarkerDone(db)
	if err != nil {
		return err
	}
	if markerDone {
		log.Printf("care_plan_converter: marker %q present — no-op", ConversionMarkerKey)
		return nil
	}
	report, err := RunCarePlanConverter(db)
	if err != nil {
		return err
	}
	if report != nil {
		log.Printf("care_plan_converter: done — seeds +%d rules /%d matchers, feeding %d rules + %d plans, treatments %d plans",
			report.Seeds.RulesInserted, report.Seeds.MatchersInserted,
			report.Feeding.RulesCreated, report.Feeding.PlansCreated,
			report.Treatments.PlansCreated)
	}
	return nil
}

func conversionMarkerDone(db *pop.Connection) (bool, error) {
	var markers []models.CarePlanConversion
	if err := db.Where("`key` = ?", ConversionMarkerKey).All(&markers); err != nil {
		return false, fmt.Errorf("care_plan_converter: marker check failed: %w", err)
	}
	return len(markers) > 0, nil
}

// RunCarePlanConverter executes the §8.1 steps, each in its own transaction,
// and writes the marker with the persisted report on success.
func RunCarePlanConverter(db *pop.Connection) (*ConversionReport, error) {
	done, err := conversionMarkerDone(db)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, nil
	}
	report := &ConversionReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	if err := db.Transaction(func(tx *pop.Connection) error {
		return convertSeedLibrary(tx, report)
	}); err != nil {
		return nil, fmt.Errorf("care_plan_converter: seed step failed: %w", err)
	}

	feedCareID, err := resolveFeedingCaretype(db)
	if err != nil {
		return nil, err
	}

	if err := db.Transaction(func(tx *pop.Connection) error {
		return convertFeedingSchedules(tx, report, feedCareID)
	}); err != nil {
		return nil, fmt.Errorf("care_plan_converter: feeding step failed: %w", err)
	}

	if err := db.Transaction(func(tx *pop.Connection) error {
		return convertTreatmentSeries(tx, report)
	}); err != nil {
		return nil, fmt.Errorf("care_plan_converter: treatment step failed: %w", err)
	}

	if err := db.Transaction(func(tx *pop.Connection) error {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		// Raw upsert: the table's PK is `key` (no surrogate id column), so
		// pop's model machinery does not apply here.
		return tx.RawQuery(
			"INSERT INTO care_plan_conversion (`key`, finished_at, report, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW()) "+
				"ON DUPLICATE KEY UPDATE finished_at = VALUES(finished_at), report = VALUES(report), updated_at = NOW()",
			ConversionMarkerKey, time.Now(), string(raw),
		).Exec()
	}); err != nil {
		return nil, fmt.Errorf("care_plan_converter: marker step failed: %w", err)
	}
	return report, nil
}

// ---------------------------------------------------------------------------
// Step 1 — seed library (§7.4 / §8.1)
// ---------------------------------------------------------------------------

func convertSeedLibrary(tx *pop.Connection, report *ConversionReport) error {
	// Canonical + derived matchers; existing rows (by name) are left alone.
	matcherIDs := make(map[string]uuid.NullUUID)
	for _, def := range SeedMatchers() {
		id, inserted, err := insertMatcherOnce(tx, buildSeedMatcher(def))
		if err != nil {
			return err
		}
		matcherIDs[def.Key] = id
		if inserted {
			report.Seeds.MatchersInserted++
		} else {
			report.Seeds.MatchersSkipped++
		}
	}
	feedID, careID, err := resolveSeedCaretypes(tx)
	if err != nil {
		return err
	}
	for _, def := range SeedRules() {
		var ctID string
		switch {
		case def.CaretypeName == "":
			ctID = ""
		case def.Kind == careplan.KindFeeding:
			ctID = feedID
			if ctID == "" {
				report.Seeds.RulesSkipped++
				report.Seeds.Skipped = append(report.Seeds.Skipped, def.Key+" "+def.Name+" (caretype Repas/Alimentation introuvable)")
				continue
			}
		default:
			ctID = careID
			if ctID == "" {
				report.Seeds.RulesSkipped++
				report.Seeds.Skipped = append(report.Seeds.Skipped, def.Key+" "+def.Name+" (caretype Soin introuvable)")
				continue
			}
		}
		rule, err := buildSeedRule(def, matcherIDs[def.MatcherKey], ctID)
		if err != nil {
			report.Seeds.RulesSkipped++
			report.Seeds.Skipped = append(report.Seeds.Skipped, err.Error())
			continue
		}
		exists, err := careRuleNameExists(tx, rule.Name)
		if err != nil {
			return err
		}
		if exists {
			report.Seeds.RulesSkipped++
			continue
		}
		if err := tx.Create(rule); err != nil {
			return err
		}
		report.Seeds.RulesInserted++
	}
	return nil
}

func careRuleNameExists(tx *pop.Connection, name string) (bool, error) {
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_rules WHERE name = ?", name).All(&n); err != nil {
		return false, err
	}
	return len(n) > 0 && n[0].C > 0, nil
}

func insertMatcherOnce(tx *pop.Connection, m *models.CareMatcher) (uuid.NullUUID, bool, error) {
	var n []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_matchers WHERE name = ?", m.Name).All(&n); err != nil {
		return uuid.NullUUID{}, false, err
	}
	if len(n) > 0 && n[0].C > 0 {
		var found models.CareMatcher
		if err := tx.Where("name = ?", m.Name).First(&found); err != nil {
			return uuid.NullUUID{}, false, err
		}
		return uuid.NullUUID{UUID: found.ID, Valid: true}, false, nil
	}
	verrs, err := tx.ValidateAndCreate(m)
	if err != nil {
		return uuid.NullUUID{}, false, err
	}
	if verrs.HasAny() {
		return uuid.NullUUID{}, false, fmt.Errorf("matcher %q invalid: %v", m.Name, verrs)
	}
	return uuid.NullUUID{UUID: m.ID, Valid: true}, true, nil
}

// resolveFeedingCaretype returns the feeding caretype id used by converted
// feeding plans (first caretype of type=1, preferring "Repas").
func resolveFeedingCaretype(db *pop.Connection) (string, error) {
	var cts models.Caretypes
	if err := db.Where("type = ?", models.CareTypeFeed).All(&cts); err != nil {
		return "", err
	}
	for _, want := range []string{"Repas", "Alimentation"} {
		for _, ct := range cts {
			if strings.EqualFold(ct.Name, want) {
				return ct.ID.String(), nil
			}
		}
	}
	if len(cts) > 0 {
		return cts[0].ID.String(), nil
	}
	return "", nil
}

func resolveSeedCaretypes(tx *pop.Connection) (feedID, careID string, err error) {
	feedID, err = resolveFeedingCaretype(tx)
	if err != nil {
		return "", "", err
	}
	var ct models.Caretype
	if e := tx.Where("LOWER(name) = ?", "soin").First(&ct); e == nil {
		careID = ct.ID.String()
	}
	return feedID, careID, nil
}
