package grifts

import (
	"fmt"

	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
)

// care:migrate_treatment_times — one-shot backfill of the legacy
// Timedonebitmap into per-time treatment entries (bugs.md R5-3d, U25/D-e).
// Idempotent: treatments already owning entries are skipped, so the task
// can be re-run safely (deploy, retry after partial failure).
var _ = grift.Add("care:migrate_treatment_times", func(c *grift.Context) error {
	fmt.Println("Migrating legacy treatment bitmap to per-time entries (bugs.md R5-3d, U25/D-e)...")

	report, err := models.MigrateTreatmentTimes(models.DB)
	if err != nil {
		return err
	}

	fmt.Printf("scanned=%d migrated=%d skipped(already had entries)=%d empty(nothing to migrate)=%d\n",
		report.TreatmentsScanned, report.TreatmentsMigrated, report.TreatmentsSkipped, report.TreatmentsEmpty)
	fmt.Printf("entries created=%d (done=%d pending=%d, from remarks appendums=%d)\n",
		report.EntriesCreated, report.DoneEntries, report.PendingEntries, report.AppendumEntries)
	fmt.Println("Timedonebitmap left untouched (dormant rollback path).")
	return nil
})
