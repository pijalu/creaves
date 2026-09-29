package models

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

// One-shot migration of the legacy Timedonebitmap to per-time treatment
// entries (bugs.md R5-3d, U25 / D-e). The core lives in models so it is
// testable without the grift layer; grifts/care.go is a thin wrapper.
//
// Semantics (documented decisions):
//   - A treatment that already owns entries is skipped (idempotent) — any
//     entry means the row was either migrated earlier or written by the
//     R5-3b write paths.
//   - Done bitmap buckets become done entries at the bucket default time
//     (08:00 / 12:00 / 18:00) with AppliedAt left NULL: the bitmap never
//     recorded the actual click time.
//   - Remarks appendums ("; HH:MM" / "; HH:MM (note)", written by the old
//     same-bucket fulfillment path) become done entries with the actual
//     time and note (AppliedAt set). They are ADDITIONAL doses: the old
//     code appended one appendum per same-bucket apply AFTER the first
//     (the first dose only set the bit), so a done bucket with one
//     appendum means two doses — both must survive the migration.
//   - Required bitmap buckets that are not done become pending entries.
//   - Every migrated entry carries source='migration'; the treatments row
//     itself is left untouched (bitmap column stays, dormant).

// legacyBuckets lists the bitmap buckets in canonical order.
var legacyBuckets = []int{Treatement_MORNING, Treatement_NOON, Treatement_EVENING}

// MigrationReport summarizes one MigrateTreatmentTimes run.
type MigrationReport struct {
	TreatmentsScanned  int
	TreatmentsMigrated int
	TreatmentsSkipped  int // already owned entries (idempotent skip)
	TreatmentsEmpty    int // no bits and no appendums — nothing to migrate
	EntriesCreated     int
	DoneEntries        int
	PendingEntries     int
	AppendumEntries    int // done entries carrying an actual apply time
}

// RemarkAppendum is one "; HH:MM (note)" fragment parsed from a legacy
// treatment's remarks.
type RemarkAppendum struct {
	TimeLabel string
	Note      string
}

// remarksAppendumRe matches the appendums written by the legacy
// same-bucket fulfillment path ("; HH:MM" with optional " (note)").
var remarksAppendumRe = regexp.MustCompile(`; (\d{2}:\d{2})(?: \(([^)]*)\))?`)

// parseRemarksAppendums extracts all appendums from a remarks field.
func parseRemarksAppendums(remarks nulls.String) []RemarkAppendum {
	if !remarks.Valid {
		return nil
	}
	matches := remarksAppendumRe.FindAllStringSubmatch(remarks.String, -1)
	if len(matches) == 0 {
		return nil
	}
	out := make([]RemarkAppendum, 0, len(matches))
	for _, m := range matches {
		out = append(out, RemarkAppendum{TimeLabel: m[1], Note: m[2]})
	}
	return out
}

// entryDayDueAt resolves an "HH:MM" label against a day start.
func entryDayDueAt(day time.Time, label string) (time.Time, error) {
	parts := strings.Split(label, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid time label %q", label)
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil || h < 0 || h > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in label %q", label)
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil || m < 0 || m > 59 {
		return time.Time{}, fmt.Errorf("invalid minute in label %q", label)
	}
	return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, day.Location()), nil
}

// migrationEntry builds one backfilled entry for a bitmap bucket.
func migrationEntry(t *Treatment, day time.Time, label, status string) (TreatmentTimeEntry, error) {
	due, err := entryDayDueAt(day, label)
	if err != nil {
		return TreatmentTimeEntry{}, err
	}
	return TreatmentTimeEntry{
		TreatmentID: t.ID,
		AnimalID:    t.AnimalID,
		DueAt:       due,
		TimeLabel:   label,
		Status:      status,
		Source:      TreatmentEntrySourceMigration,
	}, nil
}

// legacyBucketPlan maps a bitmap bucket to the entry it should produce:
// done bit → done at the default time, required-only → pending. A bucket
// with neither bit produces no entry ("", "").
func legacyBucketPlan(t *Treatment, bucket int) (status, label string) {
	label = DefaultEntryTimes[bucket]
	if t.Timedonebitmap&bucket != 0 {
		return TreatmentEntryStatusDone, label
	}
	if t.Timebitmap&bucket != 0 {
		return TreatmentEntryStatusPending, label
	}
	return "", ""
}

// appendumEntries turns the remarks appendums into done entries with the
// actual apply times and notes preserved verbatim. Malformed labels are
// skipped (the text stays in the untouched remarks).
func appendumEntries(t *Treatment, day time.Time, appendums []RemarkAppendum) TreatmentTimeEntries {
	entries := TreatmentTimeEntries{}
	for _, ap := range appendums {
		due, err := entryDayDueAt(day, ap.TimeLabel)
		if err != nil {
			continue
		}
		e := TreatmentTimeEntry{
			TreatmentID: t.ID,
			AnimalID:    t.AnimalID,
			DueAt:       due,
			TimeLabel:   ap.TimeLabel,
			Status:      TreatmentEntryStatusDone,
			AppliedAt:   nulls.NewTime(due),
			Source:      TreatmentEntrySourceMigration,
		}
		if ap.Note != "" {
			e.Note = nulls.NewString(ap.Note)
		}
		entries = append(entries, e)
	}
	return entries
}

// migrationEntries builds the entries a legacy treatment row should own,
// purely from its bitmap + remarks (no DB access — unit-testable).
func (t *Treatment) migrationEntries() (TreatmentTimeEntries, error) {
	day := time.Date(t.Date.Year(), t.Date.Month(), t.Date.Day(), 0, 0, 0, 0, t.Date.Location())

	entries := appendumEntries(t, day, parseRemarksAppendums(t.Remarks))
	for _, bucket := range legacyBuckets {
		status, label := legacyBucketPlan(t, bucket)
		if status == "" {
			continue
		}
		e, err := migrationEntry(t, day, label, status)
		if err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}

	entries.SortByTime()
	return entries, nil
}

// legacyTreatmentEntries returns the entries to backfill for one row;
// skipped=true means the row already owns entries (idempotent skip).
func legacyTreatmentEntries(tx *pop.Connection, t *Treatment) (entries TreatmentTimeEntries, skipped bool, err error) {
	owned, err := tx.Where("treatment_id = ?", t.ID).Count(&TreatmentTimeEntry{})
	if err != nil {
		return nil, false, err
	}
	if owned > 0 {
		return nil, true, nil
	}
	entries, err = t.migrationEntries()
	return entries, false, err
}

// tallyMigrationEntry folds one created entry into the report.
func tallyMigrationEntry(report *MigrationReport, e *TreatmentTimeEntry) {
	report.EntriesCreated++
	switch e.Status {
	case TreatmentEntryStatusDone:
		report.DoneEntries++
		// Only appendum-derived done entries carry a real apply time; the
		// bitmap never recorded one, so bit-derived entries keep it NULL.
		if e.AppliedAt.Valid {
			report.AppendumEntries++
		}
	case TreatmentEntryStatusPending:
		report.PendingEntries++
	}
}

// MigrateTreatmentTimes backfills per-time entries for every treatment row
// still relying on the legacy bitmap. Idempotent: rows owning entries are
// skipped, so a second run is a no-op.
func MigrateTreatmentTimes(tx *pop.Connection) (MigrationReport, error) {
	report := MigrationReport{}

	var all Treatments
	if err := tx.Order("date asc, animal_id asc").All(&all); err != nil {
		return report, err
	}

	for i := range all {
		t := &all[i]
		report.TreatmentsScanned++

		entries, skipped, err := legacyTreatmentEntries(tx, t)
		if err != nil {
			return report, err
		}
		if skipped {
			report.TreatmentsSkipped++
			continue
		}
		if len(entries) == 0 {
			report.TreatmentsEmpty++
			continue
		}
		for j := range entries {
			if err := tx.Create(&entries[j]); err != nil {
				return report, err
			}
			tallyMigrationEntry(&report, &entries[j])
		}
		report.TreatmentsMigrated++
	}

	return report, nil
}
