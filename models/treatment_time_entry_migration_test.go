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

// migrationHostAnimalID returns the id of an animal the fixture treatments can
// legally reference (bugs.md TEST-2).
//
// The helper used to hardcode AnimalID 42, but `treatments.animal_id` carries a
// foreign key to `animals(id)` and that table is auto-increment from ~996249 —
// no animal has ever had id 42, so every DB-backed run of this file died with
// "Error 1452: treatments_animals_id_fk" before asserting anything.
//
// Reuse an existing animal when the test database has one (it always does: it
// is never reset between runs); otherwise build the minimum chain an animal
// needs — animaltype, discoverer, discovery, intake — so the fixture is
// self-contained on a freshly migrated database too.
func migrationHostAnimalID(t *testing.T) int {
	t.Helper()
	if DB == nil {
		// Pure unit test (migrationEntries): no row is written, so the animal
		// reference is never resolved.
		return 0
	}
	var existing Animal
	if err := DB.Order("id").First(&existing); err == nil {
		return existing.ID
	}

	id, err := uuid.NewV4()
	require.NoError(t, err)
	marker := "TEST-2-" + id.String()[:8]
	now := time.Now()
	at := &Animaltype{Name: marker, Description: nulls.NewString(marker)}
	require.NoError(t, DB.Create(at))
	discoverer := &Discoverer{Firstname: nulls.NewString(marker), Lastname: nulls.NewString(marker)}
	require.NoError(t, DB.Create(discoverer))
	discovery := &Discovery{Date: now, DiscovererID: discoverer.ID, EntryCauseID: "1.1"}
	require.NoError(t, DB.Create(discovery))
	intake := &Intake{Date: now}
	require.NoError(t, DB.Create(intake))

	// animals has a UNIQUE(year, yearNumber) and the zero pair is already
	// taken by the seed data, so derive a free one from the random uuid
	// bytes (same class of collision as bugs.md TEST-1).
	b := id.Bytes()
	n := int64(b[0])<<24 | int64(b[1])<<16 | int64(b[2])<<8 | int64(b[3])

	animal := &Animal{
		Species:      marker,
		AnimaltypeID: at.ID,
		DiscoveryID:  discovery.ID,
		IntakeID:     intake.ID,
		Cage:         nulls.NewString(marker),
		// animals.IntakeDate is NOT NULL with no default: left unset it
		// sends the zero time and MySQL rejects '0000-00-00'.
		IntakeDate: now,
		Year:       1900 + int(n)%200,
		YearNumber: int(n) % 900000,
	}
	require.NoError(t, DB.Create(animal))

	t.Cleanup(func() {
		DB.RawQuery("DELETE FROM treatments WHERE animal_id = ?", animal.ID).Exec()
		DB.RawQuery("DELETE FROM animals WHERE id = ?", animal.ID).Exec()
		DB.RawQuery("DELETE FROM intakes WHERE id = ?", intake.ID).Exec()
		DB.RawQuery("DELETE FROM discoveries WHERE id = ?", discovery.ID).Exec()
		DB.RawQuery("DELETE FROM discoverers WHERE id = ?", discoverer.ID).Exec()
		DB.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec()
	})
	return animal.ID
}

func migrationTreatment(t *testing.T, date time.Time, timebitmap, timedonebitmap int, remarks string) *Treatment {
	t.Helper()
	return &Treatment{
		Date:           date,
		AnimalID:       migrationHostAnimalID(t),
		Drug:           "TestDrug",
		Dosage:         "0.1 ml",
		Remarks:        nulls.NewString(remarks),
		Timebitmap:     timebitmap,
		Timedonebitmap: timedonebitmap,
	}
}

func TestMigrationEntriesDoneBitsOnly(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(t, day, Treatement_MORNING, Treatement_MORNING, "")

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
	tr := migrationTreatment(t, day, Treatement_MORNING|Treatement_NOON, Treatement_MORNING, "")

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
	tr := migrationTreatment(t, day, Treatement_MORNING, Treatement_MORNING,
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
	tr := migrationTreatment(t, day, Treatement_MORNING, 0, "Base; 18:45")

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
	tr := migrationTreatment(t, day, 0, 0, "Base; 25:99; 09:30")

	entries, err := tr.migrationEntries()
	require.NoError(t, err)
	require.Len(t, entries, 1)
	require.Equal(t, "09:30", entries[0].TimeLabel, "25:99 out of range -> skipped, text stays in remarks")
}

func TestMigrationEntriesNothingToMigrate(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := migrationTreatment(t, day, 0, 0, "plain remark, no appendums")

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
		migrationTreatment(t, day, Treatement_MORNING, Treatement_MORNING, "legacy; 09:30 (late)"),
		migrationTreatment(t, day, Treatement_EVENING, 0, ""),
		migrationTreatment(t, day, 0, 0, "nothing to do"),
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
