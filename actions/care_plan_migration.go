package actions

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// One-shot startup_v3 migration (bugs.md t10, 2026-10-27 user ruling:
// "cage names are per centers — eliminate the cage-name matchers; in
// doubt, migrate to the animal; this general rule must apply to all
// centers").
//
// The v1/v2 feeding converter built cluster rules `species IN (…) AND
// cage …` (and, before R5-1d, even species-only matchers). Both shapes
// age badly in a multi-center world:
//   - cage names are per-center vocabulary: a matcher naming cages
//     breaks the moment a center renames/reallocates them;
//   - species-only sweeps over-match: any same-species animal housed
//     elsewhere inherits a diet that is not its own. The 2026-10-27
//     clone audit found v1 species-only "zombies" still active next to
//     their renamed v2 duplicates (the R4-7.24 word-boundary name fix
//     changed the upsert-by-name identity), so hedgehogs were served
//     two feeding plans and LAIT-diet animals a bogus "poussin moulu"
//     plan.
//
// The migration therefore migrates EVERY converter-owned active feeding
// rule to the animal:
//   - for each in-care animal the rule's matcher still matches, a
//     per-animal care_animal_plans row is created with the rule's
//     ActionPayload + Schedule copied VERBATIM — but only when the
//     animal's own diet equals the rule's payload food. Over-swept
//     animals (different diet) get nothing: their rule coverage was the
//     bug. Animals already owning a same-named plan (their own slots)
//     keep it — the name guard prefers the animal's own schedule.
//   - the rule is RETIRED (active=0), never deleted: care_plan_applications
//     rows reference its id.
//   - the matcher is deleted once no rule references it, so cage-name
//     expressions disappear from /care_matchers.
//   - hand-edited converter rules (updated_at ≠ created_at) are kept
//     untouched and reported — an administrator's adjustment wins.
//
// Idempotency: the care_plan_conversion row key=ConversionMarkerKeyV3 is
// written on success; later boots are a no-op. Deleting the marker
// re-runs it write-neutrally (retired rules are no longer active).
// A failure aborts the boot (cmd/app/main.go).

// RunCarePlanMigrationAtBoot runs the startup_v3 migration against
// models.DB. A finished marker makes it a no-op; any error is returned
// so the caller (cmd/app/main.go) can abort the boot.
func RunCarePlanMigrationAtBoot() error {
	db := models.DB
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV3)
	if err != nil {
		return err
	}
	if done {
		log.Printf("care_plan_migration: marker %q present — no-op", ConversionMarkerKeyV3)
		return nil
	}
	report, err := RunCarePlanMigration(db)
	if err != nil {
		return err
	}
	if report != nil {
		log.Printf("care_plan_migration: done — rules considered %d, retired %d (hand-edited kept %d), plans +%d, over-sweep dropped %d, matchers deleted %d",
			report.Migration.RulesConsidered, report.Migration.RulesRetired,
			report.Migration.RulesHandEditedSkipped, report.Migration.PlansCreated,
			report.Migration.OverSweepDropped, report.Migration.MatchersDeleted)
	}
	return nil
}

// RunCarePlanMigration executes the startup_v3 pass in one transaction and
// writes the marker with the persisted report on success. Returns nil when
// the marker is already present.
func RunCarePlanMigration(db *pop.Connection) (*ConversionReport, error) {
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV3)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, nil
	}
	report := &ConversionReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	if err := db.Transaction(func(tx *pop.Connection) error {
		return migrateConverterRulesToAnimals(tx, report)
	}); err != nil {
		return nil, fmt.Errorf("care_plan_migration: migration step failed: %w", err)
	}

	if err := db.Transaction(func(tx *pop.Connection) error {
		raw, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		return tx.RawQuery(
			"INSERT INTO care_plan_conversion (`key`, finished_at, report, created_at, updated_at) VALUES (?, ?, ?, NOW(), NOW()) "+
				"ON DUPLICATE KEY UPDATE finished_at = VALUES(finished_at), report = VALUES(report), updated_at = NOW()",
			ConversionMarkerKeyV3, time.Now(), string(raw),
		).Exec()
	}); err != nil {
		return nil, fmt.Errorf("care_plan_migration: marker step failed: %w", err)
	}
	return report, nil
}

// migrateConverterRulesToAnimals retires every converter-owned active
// feeding rule (name suffix " (conversion)" — see converterOwnedMatcher
// for the provenance rationale) in favor of per-animal plans.
func migrateConverterRulesToAnimals(tx *pop.Connection, report *ConversionReport) error {
	var rules []models.CareRule
	if err := tx.Where("active = 1 AND action_kind = ? AND (name LIKE ? OR description LIKE ?)",
		careplan.KindFeeding, "% (conversion)", "%"+ConverterTag+"%").Order("created_at, name").All(&rules); err != nil {
		return err
	}
	report.Migration.RulesConsidered = len(rules)
	// NOTE: no early return when len(rules)==0 — production can be in the
	// "everything already retired" corner (first buggy v3 run), and the
	// completion pass below is exactly what must still run there.
	if len(rules) > 0 {

		// In-care animals + enriched matcher contexts — the SAME evaluation
		// the day plan uses, so "the rule still matches" is measured with the
		// production semantics, not a re-implementation.
		var animals []models.Animal
		if err := tx.Where("outtake_id IS NULL").All(&animals); err != nil {
			return err
		}
		pa, err := assemblePlanAnimals(tx, time.Now(), animals)
		if err != nil {
			return err
		}
		// Deterministic pass order.
		ids := make([]int, 0, len(pa.ctxs))
		for id := range pa.ctxs {
			ids = append(ids, id)
		}
		sort.Ints(ids)
		reg := careplan.DefaultRegistry()

		for i := range rules {
			rule := &rules[i]
			if handEdited(rule.CreatedAt, rule.UpdatedAt) {
				report.Migration.RulesHandEditedSkipped++
				// The rule (and its matcher) stays exactly as the administrator
				// left it — the matcher counts as kept.
				if rule.MatcherID.Valid {
					report.Migration.MatchersKept++
				}
				report.Migration.Lines = append(report.Migration.Lines,
					ConversionLine{Label: rule.Name, Reason: "règle modifiée à la main — conservée telle quelle"})
				continue
			}
			if err := migrateOneConverterRule(tx, report, rule, pa, ids, reg); err != nil {
				return fmt.Errorf("rule %s (%s): %w", rule.ID, rule.Name, err)
			}
		}
	}

	// Completion pass (v3.1, 2026-10-27): the first v3 build retired rules
	// WITHOUT releasing the matcher link, so the cleanup could never fire
	// (care_rules.matcher_id → care_matchers is ON DELETE RESTRICT) and
	// the cage matchers stayed in /care_matchers behind a written marker.
	// Retired converter rules still carrying a matcher link are unlinked
	// here and the matcher deleted when no other rule references it. A
	// hand-deactivated converter rule is processed the same way: an
	// inactive rule evaluates nothing, and eliminating cage-name matchers
	// is the point of this migration. Write-neutral on re-runs.
	var linked []models.CareRule
	if err := tx.Where("active = 0 AND matcher_id IS NOT NULL AND (name LIKE ? OR description LIKE ?)",
		"% (conversion)", "%"+ConverterTag+"%").Order("created_at, name").All(&linked); err != nil {
		return err
	}
	for i := range linked {
		if err := releaseRuleMatcher(tx, report, &linked[i]); err != nil {
			return fmt.Errorf("rule %s (%s): %w", linked[i].ID, linked[i].Name, err)
		}
	}
	return nil
}

// releaseRuleMatcher unlinks one (already inactive) rule from its matcher
// and deletes the matcher when no rule references it anymore.
func releaseRuleMatcher(tx *pop.Connection, report *ConversionReport, rule *models.CareRule) error {
	matcherID := rule.MatcherID
	rule.MatcherID = uuid.NullUUID{}
	verrs, err := tx.ValidateAndUpdate(rule)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("unlink invalid: %v", verrs)
	}
	var refs []struct {
		C int64 `db:"c"`
	}
	if err := tx.RawQuery("SELECT count(*) as c FROM care_rules WHERE matcher_id = ?", matcherID.UUID).All(&refs); err != nil {
		return err
	}
	if len(refs) > 0 && refs[0].C == 0 {
		var m models.CareMatcher
		if err := tx.Find(&m, matcherID.UUID); err != nil {
			return fmt.Errorf("matcher cleanup: %w", err)
		}
		if err := tx.Destroy(&m); err != nil {
			return fmt.Errorf("matcher cleanup: %w", err)
		}
		report.Migration.MatchersDeleted++
		report.Migration.Lines = append(report.Migration.Lines,
			ConversionLine{Label: m.Name, Reason: "matcher sans règle référencante — supprimé"})
		return nil
	}
	report.Migration.MatchersKept++
	return nil
}

// migrateOneConverterRule migrates a single rule: per-animal plans for the
// correctly covered animals, retirement of the rule, matcher cleanup.
func migrateOneConverterRule(tx *pop.Connection, report *ConversionReport, rule *models.CareRule, pa *planAnimals, ids []int, reg *careplan.Registry) error {
	// The payload food is the cluster diet: an animal counts as covered
	// only when its own diet equals it (NormalizeDiet semantics, the §8.1
	// cluster key). The matcher alone may over-sweep — that coverage is
	// dropped, not preserved.
	var node careplan.Node
	if rule.MatcherID.Valid {
		var m models.CareMatcher
		if err := tx.Find(&m, rule.MatcherID.UUID); err != nil {
			return fmt.Errorf("matcher load: %w", err)
		}
		n, err := careplan.ParseValidatedWith(m.Expression, reg)
		if err != nil {
			// An unparseable matcher cannot be evaluated — leave the rule
			// untouched for a human instead of guessing its coverage.
			report.Migration.Lines = append(report.Migration.Lines,
				ConversionLine{Label: rule.Name, Reason: fmt.Sprintf("matcher non évaluable (%v) — règle conservée", err)})
			return nil
		}
		node = n
	}
	food := payloadFoodOf(rule.ActionPayload)

	for _, id := range ids {
		ctx := pa.ctxs[id]
		if node == nil || !careplan.Eval(reg, node, ctx) {
			continue
		}
		a := pa.rows[id]
		if food == "" || NormalizeDiet(a.Feeding.String) != NormalizeDiet(food) {
			report.Migration.OverSweepDropped++
			report.Migration.Lines = append(report.Migration.Lines,
				ConversionLine{a.ID, animalLabel(a), "couverture retirée: régime propre ≠ régime de la règle (sur-balayage)"})
			continue
		}
		if err := createMigrationPlanFromRule(tx, report, a, rule, food); err != nil {
			return err
		}
	}

	// Retire the rule — never delete: care_plan_applications.source_id
	// references it and the audit trail must stay resolvable. The matcher
	// link is released with the retirement (matcher_id → NULL): the rule
	// no longer evaluates, and care_rules.matcher_id carries ON DELETE
	// RESTRICT — an unlinked retired rule is what lets an unreferenced
	// matcher disappear from /care_matchers.
	matcherID := rule.MatcherID
	rule.Active = false
	rule.MatcherID = uuid.NullUUID{}
	verrs, err := tx.ValidateAndUpdate(rule)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("retired rule invalid: %v", verrs)
	}
	report.Migration.RulesRetired++
	report.Migration.Lines = append(report.Migration.Lines,
		ConversionLine{Label: rule.Name, Reason: "règle retirée (active=0) — couverture migrée vers l'animal"})

	// Matcher cleanup via the shared helper (unlink + delete when
	// unreferenced).
	if matcherID.Valid {
		rule.MatcherID = matcherID
		if err := releaseRuleMatcher(tx, report, rule); err != nil {
			return err
		}
	}
	return nil
}

// createMigrationPlanFromRule creates the per-animal plan carrying the
// rule's payload and schedule VERBATIM, idempotent by animal+name exactly
// like createConvertedFeedingPlan (so an animal that already owns its own
// plan of the same diet keeps its own slots).
func createMigrationPlanFromRule(tx *pop.Connection, report *ConversionReport, a models.Animal, rule *models.CareRule, food string) error {
	name := convertedFeedingName(food, false)
	var n []struct {
		C int64 `db:"c"`
	}
	// COLLATE utf8mb4_bin — see createConvertedFeedingPlan for why the
	// prod collation would silently merge distinct diets.
	if err := tx.RawQuery("SELECT count(*) as c FROM care_animal_plans WHERE animal_id = ? AND name = ? COLLATE utf8mb4_bin AND created_by IS NULL",
		a.ID, name).All(&n); err != nil {
		return err
	}
	if len(n) > 0 && n[0].C > 0 {
		report.Migration.Lines = append(report.Migration.Lines,
			ConversionLine{a.ID, animalLabel(a), "plan individuel déjà présent — ses slots propres sont conservés"})
		return nil
	}
	p := &models.CareAnimalPlan{
		AnimalID:      a.ID,
		Name:          name,
		ActionKind:    careplan.KindFeeding,
		ActionPayload: rule.ActionPayload, // verbatim — the rule is the truth being migrated
		Schedule:      rule.Schedule,      // verbatim
		ReplacesKind:  true,
		Active:        true,
		CreatedBy:     nulls.UUID{}, // converter author: NULL (users FK); §8.2 rollback target
	}
	verrs, err := tx.ValidateAndCreate(p)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		return fmt.Errorf("migrated feeding plan invalid (animal %d): %v", a.ID, verrs)
	}
	report.Migration.PlansCreated++
	report.Migration.Lines = append(report.Migration.Lines,
		ConversionLine{a.ID, animalLabel(a), fmt.Sprintf("règle #%s migrée vers l'animal (plan #%s)", rule.ID, p.ID)})
	return nil
}

// payloadFoodOf extracts the "food" key of a feeding action payload
// (empty when absent — an empty-food rule migrates nothing).
func payloadFoodOf(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	food, _ := m["food"].(string)
	return food
}
