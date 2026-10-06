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
// into care-plan data. It runs on application boot, after
// migrations, before the HTTP server accepts requests (see cmd/app/main.go);
// a failure aborts the boot so a half-converted database can never serve.
//
// Idempotency: the care_plan_conversion row key=ConversionMarkerKey is
// written on success; on later boots the converter is a no-op. Deleting
// the marker re-runs it; generated rows are guarded by name-existence
// checks and all carry the ConverterTag marker in their description so
// rollback (§8.2) can identify them.
//
// v3 (2026-10-27): the feeding step creates PER-ANIMAL plans only — the
// former ≥5 cluster rules (species IN … AND cage … matchers) are retired
// by the one-shot startup_v3 migration (care_plan_migration.go): cage
// names are per-center and species-only sweeps over-match. No converter
// rule or matcher is created anymore.

// ConversionMarkerKeyV1 is the pre-R5-1e marker key ("startup_v1"), kept
// for reference: databases marked v1 re-run the converter once on boot
// (marker v2 missing), which refreshed the converted cluster matchers
// with the R5-1d cage clause. The v1 row itself is never deleted — the
// report history stays additive.
const ConversionMarkerKeyV1 = "startup_v1"

// ConversionMarkerKey is the §8.1 marker primary key. Bumped v1→v2 with
// R5-1d/R5-1e (cluster matchers gain the cage clause) so existing
// installs re-run the converter exactly once. Since v3 the converter no
// longer builds cluster matchers at all (per-animal plans only — see
// ConversionMarkerKeyV3); the key name is kept for continuity with the
// persisted marker rows.
const ConversionMarkerKey = "startup_v2"

// ConversionMarkerKeyV3 gates the one-shot cage-matcher retirement
// (bugs.md t10, 2026-10-27 user ruling: "cage names are per centers —
// eliminate the cage-name matchers; in doubt, migrate to the animal;
// this general rule must apply to all centers"). Databases marked v2
// re-run nothing of the v2 converter but migrate ONCE at boot: every
// converter-owned active feeding rule is replaced by per-animal plans
// (payload + schedule copied verbatim), the rule is retired (active=0,
// row kept — care_plan_applications references its id) and its matcher
// deleted when unreferenced. Hand-edited converter rules are kept as-is.
// See care_plan_migration.go.
const ConversionMarkerKeyV3 = "startup_v3"

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
	// Migration records the one-shot startup_v3 pass (care_plan_migration.go):
	// converter-owned feeding rules retired in favor of per-animal plans
	// (bugs.md t10, 2026-10-27: cage names are per centers; in doubt,
	// migrate to the animal). Rule-level lines reuse ConversionLine with
	// AnimalID=0 and Label=rule name. Additive to the v1/v2 report shape —
	// older readers ignore the unknown key.
	Migration struct {
		RulesConsidered        int              `json:"rules_considered"`
		RulesRetired           int              `json:"rules_retired"`
		RulesHandEditedSkipped int              `json:"rules_hand_edited_skipped"`
		PlansCreated           int              `json:"plans_created"`
		OverSweepDropped       int              `json:"over_sweep_dropped"`
		MatchersDeleted        int              `json:"matchers_deleted"`
		MatchersKept           int              `json:"matchers_kept"`
		Lines                  []ConversionLine `json:"lines,omitempty"`
	} `json:"migration"`
	// coverage maps animal_id → converted slot list (in-memory only, used by
	// the reconciliation step; not serialized).
	coverage map[int][]careplan.TimeOfDay
}

// handEdited reports whether a converter-generated row was modified after
// creation (R5-1e skip heuristic): UpdatedAt ≠ CreatedAt beyond the MySQL
// DATETIME second-precision rounding counts as a hand edit. Converter
// refreshes bump UpdatedAt too, so a refreshed row counts as hand-edited
// on any later re-run — conservative: converter output is never clobbered
// twice, and user edits always win.
func handEdited(createdAt, updatedAt time.Time) bool {
	return updatedAt.Sub(createdAt) > 2*time.Second
}

// ---------------------------------------------------------------------------
// Entry points
// ---------------------------------------------------------------------------

// RunCarePlanConverterAtBoot runs the §8.1 conversion against models.DB.
// A finished marker makes it a no-op; any error is returned so the caller
// (cmd/app/main.go) can abort the boot.
func RunCarePlanConverterAtBoot() error {
	db := models.DB
	markerDone, err := conversionMarkerDone(db, ConversionMarkerKey)
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

func conversionMarkerDone(db *pop.Connection, key string) (bool, error) {
	var markers []models.CarePlanConversion
	if err := db.Where("`key` = ?", key).All(&markers); err != nil {
		return false, fmt.Errorf("care_plan_converter: marker check failed: %w", err)
	}
	return len(markers) > 0, nil
}

// RunCarePlanConverter executes the §8.1 steps, each in its own transaction,
// and writes the marker with the persisted report on success.
func RunCarePlanConverter(db *pop.Connection) (*ConversionReport, error) {
	done, err := conversionMarkerDone(db, ConversionMarkerKey)
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

// SeedLibraryOnce re-runs ONLY the idempotent seed-library step (R8-3):
// matchers insert-once by name, rules guarded by name existence. Lets
// post-cutover DBs (converter marker already present) pick up new seed
// rows — e.g. the SR13 cleanup rule — via `buffalo task careplan:seed`.
func SeedLibraryOnce(db *pop.Connection) (*ConversionReport, error) {
	report := &ConversionReport{GeneratedAt: time.Now().Format(time.RFC3339)}
	if err := convertSeedLibrary(db, report); err != nil {
		return nil, err
	}
	return report, nil
}

// ---------------------------------------------------------------------------
// Step 1 — seed library (§7.4 / §8.1)
// ---------------------------------------------------------------------------

func convertSeedLibrary(tx *pop.Connection, report *ConversionReport) error {
	// Canonical + derived matchers; existing rows (by name) are left alone.
	matcherIDs := make(map[string]uuid.NullUUID)
	matcherTr := SeedMatcherNameTranslations()
	for _, def := range SeedMatchers() {
		id, err := seedOneMatcher(tx, report, def, matcherTr[def.Key])
		if err != nil {
			return err
		}
		matcherIDs[def.Key] = id
	}
	feedID, careID, err := resolveSeedCaretypes(tx)
	if err != nil {
		return err
	}
	ruleTr := SeedRuleNameTranslations()
	for _, def := range SeedRules() {
		if err := seedOneRule(tx, report, def, matcherIDs[def.MatcherKey], feedID, careID, ruleTr[def.Key]); err != nil {
			return err
		}
	}
	return nil
}

// seedOneMatcher inserts one §7.4 seed matcher if absent (deduped by name)
// and, on insert, ships its localized display names (R9-6).
func seedOneMatcher(tx *pop.Connection, report *ConversionReport, def seedMatcherDef, names map[string]string) (uuid.NullUUID, error) {
	id, inserted, err := insertMatcherOnce(tx, buildSeedMatcher(def))
	if err != nil {
		return uuid.NullUUID{}, err
	}
	if !inserted {
		report.Seeds.MatchersSkipped++
		return id, nil
	}
	report.Seeds.MatchersInserted++
	// R9-6: ship the localized names of the freshly seeded matcher (fr stays
	// the base care_matchers.name column). Insert-only — an existing matcher
	// is untouched, so administrator edits survive.
	if err := saveSeedNameTranslations(tx, "care_matchers", id.UUID.String(), names); err != nil {
		return uuid.NullUUID{}, err
	}
	return id, nil
}

// seedOneRule resolves the caretype, builds and inserts one §7.4 seed rule
// if absent (deduped by name), and on insert ships its localized display
// names (R9-6). Rules whose caretype is missing are skipped with a report
// line (§4.2 payload needs a real id).
func seedOneRule(tx *pop.Connection, report *ConversionReport, def seedRuleDef, matcherID uuid.NullUUID, feedID, careID string, names map[string]string) error {
	var ctID string
	switch {
	case def.CaretypeName == "":
		ctID = ""
	case def.Kind == careplan.KindFeeding:
		ctID = feedID
		if ctID == "" {
			report.Seeds.RulesSkipped++
			report.Seeds.Skipped = append(report.Seeds.Skipped, def.Key+" "+def.Name+" (caretype Repas/Alimentation introuvable)")
			return nil
		}
	default:
		ctID = careID
		if ctID == "" {
			report.Seeds.RulesSkipped++
			report.Seeds.Skipped = append(report.Seeds.Skipped, def.Key+" "+def.Name+" (caretype Soin introuvable)")
			return nil
		}
	}
	rule, err := buildSeedRule(def, matcherID, ctID)
	if err != nil {
		report.Seeds.RulesSkipped++
		report.Seeds.Skipped = append(report.Seeds.Skipped, err.Error())
		return nil
	}
	exists, err := careRuleNameExists(tx, rule.Name)
	if err != nil {
		return err
	}
	if exists {
		report.Seeds.RulesSkipped++
		return nil
	}
	if err := tx.Create(rule); err != nil {
		return err
	}
	report.Seeds.RulesInserted++
	// R9-6: ship the localized names of the freshly seeded rule.
	return saveSeedNameTranslations(tx, "care_rules", rule.ID.String(), names)
}

// saveSeedNameTranslations writes the en-US/de/nl display-name translations
// of one freshly seeded care rule/matcher. It runs only on the insert path
// (existing rows are skipped by name), so it is naturally idempotent and
// never overwrites an administrator's edits. The canonical French name
// remains the base column; no fr row is generated (R9-6).
func saveSeedNameTranslations(tx *pop.Connection, table, id string, names map[string]string) error {
	for _, loc := range []string{"en-US", "de", "nl"} {
		v := names[loc]
		if v == "" {
			continue
		}
		if err := models.SaveTranslation(tx, table, id, "name", loc, v); err != nil {
			return fmt.Errorf("seed translation %s/%s/%s: %w", table, id, loc, err)
		}
	}
	return nil
}

// converterOwnedMatcher reports whether an existing matcher row belongs to
// the converter and may be refreshed in place. Before R9-5 the description
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
