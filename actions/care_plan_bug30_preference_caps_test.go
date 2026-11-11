package actions

import (
	"os"
	"strings"
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// BUG-30 (2026-10-12): `/care_plan?kind=care` listed items far beyond the
// configured 8 h future / 8 h late windows. Root cause: the per-kind caps
// only apply to kinds that HAVE a row in the preferences table
// ("missing row = no cap" in preferencesByKind), and PreferencesEnsureSeeded
// ran ONLY when an admin opened /preferences. A DB that predates the R9
// preferences feature — or that was seeded before a kind joined actionKinds
// — therefore rendered the un-seeded kind(s) UNCAPPED: every occurrence of
// the raw ±1/+2-day plan window reached the screen.

// TestCarePlanIndexSeedsPreferencesBeforeCapping pins the wiring: the day
// plan handler seeds the per-kind preference rows BEFORE reading them, so
// "no row" can never silently mean "no cap" on the work screen. (Source
// pin, same style as the template pins — the wiring is the bug.)
func TestCarePlanIndexSeedsPreferencesBeforeCapping(t *testing.T) {
	raw, err := os.ReadFile("care_plan.go")
	require.NoError(t, err)
	src := string(raw)

	seed := strings.Index(src, "PreferencesEnsureSeeded(tx)")
	read := strings.Index(src, "preferencesByKind(tx)")
	require.NotEqual(t, -1, seed, "CarePlanIndex must seed the per-kind preference rows itself")
	require.NotEqual(t, -1, read, "CarePlanIndex must read the per-kind preferences")
	require.Less(t, seed, read, "the seeding must happen BEFORE the preferences are read")
}

// TestPreferenceSeededDefaultsCapEveryKind pins the behavioral end: once the
// rows are seeded (what CarePlanIndex now guarantees), EVERY day-plan kind
// is bounded by the R9 defaults — late/missing at or beyond 8 h leaves,
// scheduled at or beyond 8 h leaves, work inside the window stays. This is
// the seed-drift scenario of the report: a kind whose row was missing
// rendered UNCAPPED while the admin saw 8 h configured for the others.
func TestPreferenceSeededDefaultsCapEveryKind(t *testing.T) {
	now := time.Date(2026, 10, 12, 13, 0, 0, 0, time.Local)
	for _, kind := range actionKinds {
		t.Run(kind, func(t *testing.T) {
			source := testSource(kind, "bug30", "source", map[string]interface{}{})
			// The preferences table state CarePlanIndex guarantees: one row
			// per kind with the R9 defaults.
			prefs := seededPreferenceForTest(kind)
			insideLate := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-7 * time.Hour)}, Status: careplan.StatusLate, Applicable: true}
			beyondLate := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(-26 * time.Hour)}, Status: careplan.StatusLate, Applicable: true}
			insideFuture := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(7 * time.Hour)}, Status: careplan.StatusScheduled, Applicable: true}
			beyondFuture := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: source, DueAt: now.Add(30 * time.Hour)}, Status: careplan.StatusScheduled, Applicable: true}

			got := applyPreferenceCaps([]careplan.PlanItem{insideLate, beyondLate, insideFuture, beyondFuture}, prefs, now)
			require.Len(t, got, 2, "kind %s: the 8 h defaults bound both directions", kind)
			require.Equal(t, insideLate.Occurrence.DueAt, got[0].Occurrence.DueAt, "kind %s: 7 h late work keeps its toggle", kind)
			require.Equal(t, insideFuture.Occurrence.DueAt, got[1].Occurrence.DueAt, "kind %s: 7 h future work stays visible", kind)
		})
	}
}

// TestPreferenceMissingRowNeverMeansNoCapOnWorkScreen pins the guardrail
// against regressing to "no row = no cap": every kind in actionKinds MUST
// get a seeded row from PreferencesEnsureSeeded — i.e. preferenceKinds and
// actionKinds cannot drift apart (a kind added to the chip bar without a
// preference row was exactly the BUG-30 drift).
func TestPreferenceMissingRowNeverMeansNoCapOnWorkScreen(t *testing.T) {
	require.Equal(t, len(actionKinds), len(preferenceKinds),
		"every work-screen kind must have a seeded preference row — a missing row means NO CAP (BUG-30)")
	for _, k := range actionKinds {
		require.Contains(t, preferenceKinds, k, "kind %s has no preference row → renders uncapped", k)
	}
}

// seededPreferenceForTest mirrors the row PreferencesEnsureSeeded creates
// for a kind (R9 defaults) — keeping the test DB-free while pinning the
// seeded values.
func seededPreferenceForTest(kind string) models.Preference {
	return models.Preference{
		Kind:             kind,
		LateShowHours:    nulls.Int{Int: preferenceDefaults.lateHours, Valid: true},
		FutureShowHours:  nulls.Int{Int: preferenceDefaults.futureHours, Valid: true},
		NowWindowMinutes: nulls.Int{Int: preferenceDefaults.nowMinutes, Valid: true},
	}
}
