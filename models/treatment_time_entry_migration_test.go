package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// care:migrate_treatment_times tests (bugs.md R5-3e, U25/D-e): bitmap →
// entries backfill, remarks appendum parsing, idempotency.

func migrationTreatment(date time.Time, timebitmap, timedonebitmap int, remarks string) *Treatment {
	return &Treatment{
		Date:           date,
		AnimalID:       42,
		Drug:           "TestDrug",
		Dosage:         "0.1 ml",
		Remarks:        nulls.NewString(remarks),
		Timebitmap:     timebitmap,
		Timedonebitmap: timedonebitmap,
	}
}

func TestMigrationEntriesDoneBitsOnly(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(day, Treatement_MORNING, Treatement_MORNING, "")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "08:00", entries[0].TimeLabel)
	require.Equal(t, TreatmentEntryStatusDone, entries[0].Status)
	require.Equal(t, TreatmentEntrySourceMigration, entries[0].Source)
	// The bitmap never recorded a click time — applied_at stays NULL.
	require.False(t, entries[0].AppliedAt.Valid)
	require.Equal(t, time.Date(2026, 9, 29, 8, 0, 0, 0, time.Local), entries[0].DueAt)
}

func TestMigrationEntriesRequiredNotDonePending(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(day, Treatement_MORNING|Treatement_NOON, Treatement_MORNING, "")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "08:00", entries[0].TimeLabel)
	require.Equal(t, TreatmentEntryStatusDone, entries[0].Status)
	require.Equal(t, "12:00", entries[1].TimeLabel)
	require.Equal(t, TreatmentEntryStatusPending, entries[1].Status)
	require.False(t, entries[1].AppliedAt.Valid)
}

func TestMigrationEntriesAppendumParsing(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	// Legacy same-bucket flow: first dose sets the bit, later doses append
	// "; HH:MM" / "; HH:MM (note)" to the remarks.
	tr := migrationTreatment(day, Treatement_MORNING, Treatement_MORNING,
		"Base remark; 09:30 (given late); 10:15")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	// Bit dose (default time, unknown apply time) + the two extra doses.
	require.Len(t, entries, 3)

	require.Equal(t, "08:00", entries[0].TimeLabel)
	require.False(t, entries[0].AppliedAt.Valid, "bit dose has no recorded time")
	require.False(t, entries[0].Note.Valid)

	require.Equal(t, "09:30", entries[1].TimeLabel)
	require.Equal(t, TreatmentEntryStatusDone, entries[1].Status)
	require.True(t, entries[1].AppliedAt.Valid)
	require.Equal(t, time.Date(2026, 9, 29, 9, 30, 0, 0, time.Local), entries[1].AppliedAt.Time)
	require.Equal(t, "given late", entries[1].Note.String)
	require.True(t, entries[1].Note.Valid)

	require.Equal(t, "10:15", entries[2].TimeLabel)
	require.True(t, entries[2].AppliedAt.Valid)
	require.False(t, entries[2].Note.Valid, "appendum without note stays noteless")
}

func TestMigrationEntriesAppendumOutsideRequiredBuckets(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	// An evening appendum on a row without any evening bits must survive.
	tr := migrationTreatment(day, Treatement_MORNING, 0, "Base; 18:45")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Len(t, entries, 2)
	require.Equal(t, "08:00", entries[0].TimeLabel, "required-not-done -> pending")
	require.Equal(t, TreatmentEntryStatusPending, entries[0].Status)
	require.Equal(t, "18:45", entries[1].TimeLabel)
	require.Equal(t, TreatmentEntryStatusDone, entries[1].Status)
	require.True(t, entries[1].AppliedAt.Valid)
}

func TestMigrationEntriesSkipsMalformedLabel(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(day, 0, 0, "Base; 25:99; 09:30")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "09:30", entries[0].TimeLabel, "25:99 out of range -> skipped, text stays in remarks")
}

func TestMigrationEntriesNothingToMigrate(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(day, 0, 0, "plain remark, no appendums")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestParseRemarksAppendums(t *testing.T) {
	require.Nil(t, parseRemarksAppendums(nulls.String{}))
	require.Nil(t, parseRemarksAppendums(nulls.NewString("no appendums here")))

	got := parseRemarksAppendums(nulls.NewString("a; 07:05; 18:00 (evening dose); trailing"))
	require.Len(t, got, 2)
	require.Equal(t, RemarkAppendum{TimeLabel: "07:05", Note: ""}, got[0])
	require.Equal(t, RemarkAppendum{TimeLabel: "18:00", Note: "evening dose"}, got[1])
}

// TestMigrateTreatmentTimesIdempotent proves the DB-backed run creates the
// expected entries and that a second run skips everything (bugs.md R5-3d).
func TestMigrateTreatmentTimesIdempotent(t *testing.T) {
	if DB == nil {
		t.Skip("no database connection")
	}
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	rows := []*Treatment{
		migrationTreatment(day, Treatement_MORNING, Treatement_MORNING, "legacy; 09:30 (late)"),
		migrationTreatment(day, Treatement_EVENING, 0, ""),
		migrationTreatment(day, 0, 0, "nothing to do"),
	}
	for _, r := range rows {
		require.NoError(t, DB.Create(r))
	}
	t.Cleanup(func() {
		for _, r := range rows {
			DB.RawQuery("DELETE FROM treatment_time_entries WHERE treatment_id = ?", r.ID).Exec()
			DB.RawQuery("DELETE FROM treatments WHERE id = ?", r.ID).Exec()
		}
	})

	ids := []uuid.UUID{rows[0].ID, rows[1].ID, rows[2].ID}
	restrict := func(q *pop.Query) *pop.Query { return q.Where("id IN (?)", ids) }

	report, err := migrateTreatmentTimes(DB, restrict)
	require.NoError(t, err)
	require.Equal(t, 3, report.TreatmentsScanned)
	require.Equal(t, 2, report.TreatmentsMigrated)
	require.Equal(t, 0, report.TreatmentsSkipped)
	require.Equal(t, 1, report.TreatmentsEmpty)
	require.Equal(t, 3, report.EntriesCreated)
	require.Equal(t, 2, report.DoneEntries)
	require.Equal(t, 1, report.PendingEntries)
	require.Equal(t, 1, report.AppendumEntries)

	// Row 1: done 08:00 + appendum 09:30 — the appendum carries the note.
	entries := TreatmentTimeEntries{}
	require.NoError(t, DB.Where("treatment_id = ?", rows[0].ID).All(&entries))
	require.Len(t, entries, 2)
	require.Equal(t, TreatmentEntryStatusDone, entries.FindByLabel("08:00").Status)
	first := entries.FindByLabel("09:30")
	require.NotNil(t, first)
	require.Equal(t, "late", first.Note.String)
	require.True(t, first.AppliedAt.Valid)

	// Row 2: evening required, not done -> pending 18:00.
	entries = TreatmentTimeEntries{}
	require.NoError(t, DB.Where("treatment_id = ?", rows[1].ID).All(&entries))
	require.Len(t, entries, 1)
	require.Equal(t, TreatmentEntryStatusPending, entries[0].Status)

	// Row 3: no bits, no appendums -> no entries.
	entries = TreatmentTimeEntries{}
	require.NoError(t, DB.Where("treatment_id = ?", rows[2].ID).All(&entries))
	require.Empty(t, entries)

	// Second run: every row now skipped or empty — zero new entries.
	before := TreatmentTimeEntries{}
	require.NoError(t, DB.Where("treatment_id IN (?)", ids).All(&before))

	report2, err := migrateTreatmentTimes(DB, restrict)
	require.NoError(t, err)
	require.Equal(t, 0, report2.EntriesCreated, "idempotent: second run creates nothing")
	require.Equal(t, 2, report2.TreatmentsSkipped, "rows with entries are skipped")
	require.Equal(t, 1, report2.TreatmentsEmpty)

	after := TreatmentTimeEntries{}
	require.NoError(t, DB.Where("treatment_id IN (?)", ids).All(&after))
	require.Len(t, after, len(before), "no duplicate entries after re-run")
}
