package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Read-path tests for the per-time entries (bugs.md R5-3c/R5-3e, U25/D-e):
// ScheduleStatus delegation, legacy bitmap fallback, accordion map builder.

func entryTreatment(date time.Time, timebitmap, timedonebitmap int) *Treatment {
	return &Treatment{
		Date:           date,
		AnimalID:       42,
		Drug:           "TestDrug",
		Timebitmap:     timebitmap,
		Timedonebitmap: timedonebitmap,
	}
}

func entryAt(tr *Treatment, label string, hour, minute int, done bool) TreatmentTimeEntry {
	e := TreatmentTimeEntry{
		TreatmentID: tr.ID,
		AnimalID:    tr.AnimalID,
		DueAt:       time.Date(2026, 9, 29, hour, minute, 0, 0, time.Local),
		TimeLabel:   label,
		Status:      TreatmentEntryStatusPending,
		Source:      TreatmentEntrySourceManual,
	}
	if done {
		e.Status = TreatmentEntryStatusDone
	}
	return e
}

// TestScheduleStatusDelegatesToEntries: with entries loaded, a bucket is
// done only when ALL of its entries are done (bugs.md R5-3c).
func TestScheduleStatusDelegatesToEntries(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := entryTreatment(day, Treatement_MORNING, Treatement_MORNING)
	tr.Entries = TreatmentTimeEntries{
		entryAt(tr, "08:00", 8, 0, true),
		entryAt(tr, "09:30", 9, 30, false),
	}

	got := tr.ScheduleStatus(Treatement_MORNING)
	require.True(t, got.Valid, "bucket with entries is not unknown")
	require.False(t, got.Bool, "one pending entry keeps the bucket not done")

	tr.Entries[1].Status = TreatmentEntryStatusDone
	got = tr.ScheduleStatus(Treatement_MORNING)
	require.True(t, got.Valid)
	require.True(t, got.Bool, "all entries done -> bucket done")
}

// TestScheduleStatusBucketWithoutEntriesUnknown: a loaded-entries row whose
// entries live in another bucket reports unknown for the empty bucket.
func TestScheduleStatusBucketWithoutEntriesUnknown(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := entryTreatment(day, Treatement_MORNING, Treatement_MORNING)
	tr.Entries = TreatmentTimeEntries{entryAt(tr, "08:00", 8, 0, true)}

	got := tr.ScheduleStatus(Treatement_EVENING)
	require.False(t, got.Valid, "no entries in the evening bucket -> unknown")
}

// TestScheduleStatusBitmapFallback: without entries the dormant bitmap
// still answers (rollback path, bugs.md R5-3c/D-e).
func TestScheduleStatusBitmapFallback(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)

	tr := entryTreatment(day, Treatement_MORNING, Treatement_MORNING)
	got := tr.ScheduleStatus(Treatement_MORNING)
	require.True(t, got.Valid)
	require.True(t, got.Bool, "bitmap bit done -> done")

	tr = entryTreatment(day, Treatement_MORNING, 0)
	got = tr.ScheduleStatus(Treatement_MORNING)
	require.True(t, got.Valid)
	require.False(t, got.Bool, "bitmap bit pending -> not done")

	tr = entryTreatment(day, 0, 0)
	got = tr.ScheduleStatus(Treatement_NOON)
	require.False(t, got.Valid, "not required -> unknown")
}

// TestScheduleStatusSkippedEntryNotDone: skipped entries don't count as
// done — the bucket stays open until every run is either done or the
// remaining ones are explicitly skipped (kept out of the "done" rollup).
func TestScheduleStatusSkippedEntryNotDone(t *testing.T) {
	day := time.Date(2026, 9, 29, 0, 0, 0, 0, time.Local)
	tr := entryTreatment(day, Treatement_MORNING, Treatement_MORNING)
	tr.Entries = TreatmentTimeEntries{
		entryAt(tr, "08:00", 8, 0, true),
		func() TreatmentTimeEntry {
			e := entryAt(tr, "09:30", 9, 30, false)
			e.Status = TreatmentEntryStatusSkipped
			return e
		}(),
	}

	got := tr.ScheduleStatus(Treatement_MORNING)
	require.True(t, got.Valid)
	require.False(t, got.Bool, "skipped is not done")
}

// TestEntryBucketBounds pins the §10-M1 bucket windows used by delegation.
func TestEntryBucketBounds(t *testing.T) {
	require.Equal(t, []int{0, 11}, entryBucketBounds(Treatement_MORNING))
	require.Equal(t, []int{11, 16}, entryBucketBounds(Treatement_NOON))
	require.Equal(t, []int{16, 24}, entryBucketBounds(Treatement_EVENING))
	require.Nil(t, entryBucketBounds(8))
}

// TestTreatmentEntriesMapBuilder: the R5-4b accordion data source groups
// rows per date key, keeps empty-entries rows, and orders keys newest first.
func TestTreatmentEntriesMapBuilder(t *testing.T) {
	yesterday := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	today := time.Date(2026, 9, 29, 12, 30, 0, 0, time.Local) // Date carries a time on purpose

	tr1 := entryTreatment(yesterday, Treatement_MORNING, Treatement_MORNING)
	tr1.Entries = TreatmentTimeEntries{entryAt(tr1, "08:00", 8, 0, true)}
	tr2 := entryTreatment(today, Treatement_MORNING, 0) // legacy row without entries
	tr2.Entries = nil

	ts := Treatments{*tr1, *tr2}
	m := ts.TreatmentEntriesMap()
	require.Len(t, m, 2, "one key per date")

	keys := m.OrderedKeys()
	require.Len(t, keys, 2)
	require.Equal(t, "2026/09/29", keys[0].DateFmt, "newest day first")
	require.True(t, keys[0].Current, "today flagged current")
	require.True(t, keys[1].Past, "yesterday flagged past")

	rows := m[keys[0]]
	require.Len(t, rows, 1)
	require.Equal(t, tr2.ID, rows[0].Treatment.ID)
	require.Empty(t, rows[0].Entries, "empty-entries rows still appear")

	rows = m[keys[1]]
	require.Len(t, rows, 1)
	require.Equal(t, tr1.ID, rows[0].Treatment.ID)
	require.Len(t, rows[0].Entries, 1)
}

// TestTreatmentKeyForConsistency: extracted key builder matches the
// inline logic it replaced in TreatmentsMap.
func TestTreatmentKeyForConsistency(t *testing.T) {
	day := time.Date(2026, 9, 29, 15, 45, 0, 0, time.Local)
	tr := entryTreatment(day, 0, 0)

	k := treatmentKeyFor(*tr)
	require.Equal(t, "2026/09/29", k.DateFmt)
	require.True(t, k.Current)
	require.False(t, k.Past)
	require.False(t, k.Future)

	old := time.Date(2020, 1, 5, 8, 0, 0, 0, time.Local)
	k = treatmentKeyFor(*entryTreatment(old, 0, 0))
	require.True(t, k.Past)
	require.Equal(t, "2020/01/05", k.DateFmt)
}
