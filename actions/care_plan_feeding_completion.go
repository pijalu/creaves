package actions

import (
	"encoding/json"
	"log"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
)

// One-shot startup_v4 feeding-completion pass (bugs.md BUG-10, 2026-10-14
// user ruling: "all replaced action *must* be converted — they can be
// created as per-animal rules for migration simplicity").
//
// The startup_v2 feeding converter only selected in-care animals with
// feeding_period > 0 AND feeding_start/end NOT NULL. Rows with a period
// but NULL times — or with only a diet text — were silently skipped,
// leaving those animals with NO feeding plan at all while the show page
// kept displaying the (phantom-defaulted) legacy schedule. Example:
// animal 8842 — feeding_period 480, NULL start/end, diet "15 poussins et
// 5 souris // EAU X8" — had zero feeding plans until this pass.
//
// This pass simply re-runs the (now widened) feeding conversion step once
// against databases whose startup_v2 marker was already written before
// the widening. The per-animal name guard in createConvertedFeedingPlan
// makes it strictly write-neutral for animals already converted; only the
// previously-skipped rows get their fallback plans (single daily slot,
// 09:00 when no legacy start — BUG-52e user ruling).
//
// Idempotency: the care_plan_conversion row key=ConversionMarkerKeyV4 is
// written on success; later boots are a no-op. A failure aborts the boot
// (cmd/app/main.go) so a half-completed pass never serves.

// ConversionMarkerKeyV4 gates the feeding-completion pass.
const ConversionMarkerKeyV4 = "startup_v4"

// RunCarePlanFeedingCompletionAtBoot runs the startup_v4 pass against
// models.DB. A finished marker makes it a no-op; any error is returned so
// the caller (cmd/app/main.go) can abort the boot.
func RunCarePlanFeedingCompletionAtBoot() error {
	db := models.DB
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV4)
	if err != nil {
		return err
	}
	if done {
		log.Printf("care_plan_feeding_completion: marker %q present — no-op", ConversionMarkerKeyV4)
		return nil
	}
	report, err := RunCarePlanFeedingCompletion(db)
	if err != nil {
		return err
	}
	if report != nil {
		log.Printf("care_plan_feeding_completion: done — %d animals considered, %d feeding plans created, uncovered %d",
			report.Feeding.AnimalsConsidered, report.Feeding.PlansCreated,
			report.Reconciliation.FeedingUncovered)
	}
	return nil
}

// RunCarePlanFeedingCompletion executes the widened feeding step in one
// transaction and writes the v4 marker with the persisted report on
// success. Returns nil when the marker is already present.
func RunCarePlanFeedingCompletion(db *pop.Connection) (*ConversionReport, error) {
	done, err := conversionMarkerDone(db, ConversionMarkerKeyV4)
	if err != nil {
		return nil, err
	}
	if done {
		return nil, nil
	}
	report := &ConversionReport{GeneratedAt: time.Now().Format(time.RFC3339)}

	feedCareID, err := resolveFeedingCaretype(db)
	if err != nil {
		return nil, err
	}

	if err := db.Transaction(func(tx *pop.Connection) error {
		return convertFeedingSchedules(tx, report, feedCareID)
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
			ConversionMarkerKeyV4, time.Now(), string(raw),
		).Exec()
	}); err != nil {
		return nil, err
	}
	return report, nil
}
