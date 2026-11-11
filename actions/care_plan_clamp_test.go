package actions

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// TestClampDeferredUntilSparseSchedule (bugs.md BUG-18): the defer clamp
// searched for the next occurrence in a fixed 30-day window — a schedule
// sparser than that (every_days=45) had no successor in range, so the defer
// passed through unclamped, sailed past the real successor, and the eventual
// apply 409'd hors-délai without the item ever showing the late flag.
func TestClampDeferredUntilSparseSchedule(t *testing.T) {
	// Intake 06:00, slot 08:00 (intake before the slot so day 1 keeps it).
	intake := time.Date(2026, 1, 1, 6, 0, 0, 0, time.Local)
	ctx := &careplan.AnimalContext{ID: 1, IntakeDate: intake}

	// Every 45 days at 08:00, default miss_after_hours=24. Occurrences:
	// Jan 1, Feb 15, Apr 1, …
	src := careplan.NewSource(careplan.SourceRule, "r-sparse", "Sparse", "medication",
		careplan.Schedule{
			Times:          []careplan.TimeOfDay{{Hour: 8}},
			EveryDays:      45,
			Anchor:         careplan.AnchorIntake,
			GraceMinutes:   60,
			MissAfterHours: 24,
		})

	due := time.Date(2026, 1, 1, 8, 0, 0, 0, time.Local)        // first occurrence
	successor := time.Date(2026, 2, 15, 8, 0, 0, 0, time.Local) // +45d
	requested := due.Add(50 * 24 * time.Hour)                   // Feb 20 — past the successor AND the old 30d window

	got := ClampDeferredUntil(src, ctx, 1, due, requested)
	require.Equal(t, successor.Add(-time.Minute), got,
		"defer must clamp to just before the real successor, even beyond 30 days")

	// A request before the successor still passes through.
	earlier := due.Add(10 * 24 * time.Hour)
	require.Equal(t, earlier, ClampDeferredUntil(src, ctx, 1, due, earlier))
}

// TestClampDeferredUntilNoSuccessor (bugs.md BUG-18): when the course is
// over (no successor at all), the defer must not outlive the occurrence's
// own applicability — clamp to the miss deadline (§4.3 miss_after_hours).
func TestClampDeferredUntilNoSuccessor(t *testing.T) {
	// Intake 06:00, slot 08:00 (intake before the slot so day 1 keeps it).
	intake := time.Date(2026, 1, 1, 6, 0, 0, 0, time.Local)
	ctx := &careplan.AnimalContext{ID: 1, IntakeDate: intake}

	// One-day course: the single occurrence is the last one.
	src := careplan.NewSource(careplan.SourceRule, "r-course", "Course", "medication",
		careplan.Schedule{
			Times:          []careplan.TimeOfDay{{Hour: 8}},
			EveryDays:      1,
			Anchor:         careplan.AnchorIntake,
			DurationDays:   1,
			GraceMinutes:   60,
			MissAfterHours: 24,
		})

	due := time.Date(2026, 1, 1, 8, 0, 0, 0, time.Local) // the only occurrence
	got := ClampDeferredUntil(src, ctx, 1, due, due.Add(7*24*time.Hour))
	require.Equal(t, due.Add(24*time.Hour), got,
		"no successor: clamp to the miss deadline, not free passthrough")

	// Within the miss window the request still passes through.
	within := due.Add(6 * time.Hour)
	require.Equal(t, within, ClampDeferredUntil(src, ctx, 1, due, within))

	// Floor still applies: requested before due+1min → due+1min.
	require.Equal(t, due.Add(time.Minute),
		ClampDeferredUntil(src, ctx, 1, due, due.Add(-time.Hour)))
}
