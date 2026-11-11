package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Entry-model unit tests (bugs.md R5-3e, U25/D-e): status lifecycle,
// validation, collection helpers, bucket windowing.

func testEntry() TreatmentTimeEntry {
	return TreatmentTimeEntry{
		ID:          uuid.Must(uuid.NewV4()),
		TreatmentID: uuid.Must(uuid.NewV4()),
		AnimalID:    42,
		DueAt:       time.Date(2026, 9, 29, 8, 0, 0, 0, time.Local),
		TimeLabel:   "08:00",
		Status:      TreatmentEntryStatusPending,
		Source:      TreatmentEntrySourceManual,
	}
}

func TestTreatmentTimeEntryValidate(t *testing.T) {
	e := testEntry()
	verrs, err := e.Validate(nil)
	require.NoError(t, err)
	require.False(t, verrs.HasAny(), "unexpected errors: %v", verrs)
}

func TestTreatmentTimeEntryValidateInclusions(t *testing.T) {
	e := testEntry()
	e.Status = "later"
	verrs, _ := e.Validate(nil)
	require.NotNil(t, verrs.Get("status"), "expected status inclusion error")

	e = testEntry()
	e.Source = "carrier-pigeon"
	verrs, _ = e.Validate(nil)
	require.NotNil(t, verrs.Get("source"), "expected source inclusion error")
}

func TestTreatmentTimeEntryMarkDoneRevert(t *testing.T) {
	e := testEntry()
	user := uuid.Must(uuid.NewV4())
	at := time.Date(2026, 9, 29, 8, 12, 0, 0, time.Local)

	e.MarkDone(at, user)
	require.Equal(t, TreatmentEntryStatusDone, e.Status)
	require.True(t, e.AppliedAt.Valid)
	require.Equal(t, at, e.AppliedAt.Time)
	require.True(t, e.UserID.Valid)
	require.Equal(t, user, e.UserID.UUID)

	e.Revert()
	require.Equal(t, TreatmentEntryStatusPending, e.Status)
	require.False(t, e.AppliedAt.Valid)
	require.False(t, e.UserID.Valid)
	require.False(t, e.ApplicationID.Valid)
}

func TestTreatmentTimeEntriesSortByTime(t *testing.T) {
	entries := TreatmentTimeEntries{
		{TimeLabel: "18:00", DueAt: time.Date(2026, 9, 29, 18, 0, 0, 0, time.Local)},
		{TimeLabel: "08:00", DueAt: time.Date(2026, 9, 29, 8, 0, 0, 0, time.Local)},
		{TimeLabel: "12:30", DueAt: time.Date(2026, 9, 29, 12, 30, 0, 0, time.Local)},
	}
	entries.SortByTime()
	require.Equal(t, "08:00", entries[0].TimeLabel)
	require.Equal(t, "12:30", entries[1].TimeLabel)
	require.Equal(t, "18:00", entries[2].TimeLabel)
}

func TestTreatmentTimeEntriesFindByLabelAndApplication(t *testing.T) {
	appID := uuid.Must(uuid.NewV4())
	entries := TreatmentTimeEntries{
		{TimeLabel: "08:00", Status: TreatmentEntryStatusDone, ApplicationID: nulls.NewUUID(appID)},
		{TimeLabel: "12:00", Status: TreatmentEntryStatusPending},
	}
	require.NotNil(t, entries.FindByLabel("12:00"))
	require.Nil(t, entries.FindByLabel("21:00"))
	require.Equal(t, "08:00", entries.FindByApplication(appID).TimeLabel)
	require.Nil(t, entries.FindByApplication(uuid.Must(uuid.NewV4())))
	require.Nil(t, entries.FindByApplication(uuid.Nil), "nil application never matches a set one")
}

func TestTreatmentTimeEntriesHasDone(t *testing.T) {
	require.False(t, TreatmentTimeEntries{}.HasDone())
	require.False(t, TreatmentTimeEntries{{Status: TreatmentEntryStatusPending}}.HasDone())
	require.True(t, TreatmentTimeEntries{
		{Status: TreatmentEntryStatusPending},
		{Status: TreatmentEntryStatusDone},
	}.HasDone())
}

func TestTreatmentTimeEntryInBucket(t *testing.T) {
	mk := func(h, m int) *TreatmentTimeEntry {
		return &TreatmentTimeEntry{DueAt: time.Date(2026, 9, 29, h, m, 0, 0, time.Local)}
	}
	// §10-M1 bounds: <11:00 morning, 11:00–15:59 noon, ≥16:00 evening.
	require.True(t, mk(10, 59).InBucket(0, 11))
	require.False(t, mk(11, 0).InBucket(0, 11))
	require.True(t, mk(11, 0).InBucket(11, 16))
	require.True(t, mk(15, 59).InBucket(11, 16))
	require.False(t, mk(16, 0).InBucket(11, 16))
	require.True(t, mk(16, 0).InBucket(16, 24))
	require.True(t, mk(23, 59).InBucket(16, 24))
}
