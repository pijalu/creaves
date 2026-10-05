package actions

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// R4-7.18: "not sure to see a difference between compact/detailled — if none,
// remove".
//
// Measured before the change (byte diff of the rendered HTML, csrf/URL noise
// ignored): care, weighing, cleanup, medication and feeding rendered
// IDENTICALLY; only the toggle's own active state differed. Observation was the
// single exception (13 rows compact vs 23 detailed).
//
// The screen therefore has ONE density: the toggle is gone and the work screen
// renders the compact view for every request, including the ones that still
// carry a legacy `?view=detailed`.
func TestCarePlanHasSingleDensity(t *testing.T) {
	forks := []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/index.plush.fr.html",
		"../templates/care_plan/index.plush.de.html",
		"../templates/care_plan/index.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)

		// No toggle link survives: nothing may link to view=detailed/compact.
		require.NotContains(t, raw, "view=detailed", f+" still links to the detailed view")
		require.NotContains(t, raw, "view=compact", f+" still links to the compact view")
		// The switch was the only user of view.View in the markup.
		require.NotContains(t, raw, "view.View", f+" still branches on view.View")

		// The localized labels are now unused; drop them so they cannot rot
		// into a misleading "this feature exists" entry.
		for _, key := range []string{
			"care_plan.view.compact",
			"care_plan.view.detailed",
		} {
			require.NotContains(t, raw, key, f+" still renders "+key)
		}
	}

	// The handler ignores the parameter instead of honouring it, so an old
	// bookmark or an old `back=` target lands on the one view rather than 404
	// or silently switching density.
	src := readTemplate(t, "../actions/care_plan.go")
	require.Contains(t, src, "view := ViewCompact",
		"the work screen must force the single density")
}

// TestCarePlanOneDensityForEveryKind: the view model must produce the same
// summary regardless of the density that was asked for, so a stale
// `?view=detailed` link cannot resurrect a different workload. The handler
// pins one density; this pins that the projection's honesty counters (the
// summary strip) never depended on it in the first place.
func TestCarePlanOneDensityForEveryKind(t *testing.T) {
	plan := pipelinePlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	for _, kind := range []string{"", "observation", "care", "weighing", "feeding"} {
		c := BuildDayPlanView(plan, ViewCompact, "", kind, "", now)
		d := BuildDayPlanView(plan, ViewDetailed, "", kind, "", now)
		require.Equal(t, c.Stats, d.Stats,
			"kind=%q: the summary must not depend on the density", kind)
	}

	// Sanity: the projection still accepts both values (the constant is part
	// of the model); it is the REQUEST HANDLER that pins one density.
	c := BuildDayPlanView(plan, ViewCompact, "", "", "", now)
	d := BuildDayPlanView(plan, ViewDetailed, "", "", "", now)
	require.False(t, c.Detailed)
	require.True(t, d.Detailed)
}

// TestPlanSelfPathCarriesNoViewParam: the propagated back URL must not carry
// the dead density parameter — it is copied, bookmarked and read by caregivers.
func TestPlanSelfPathCarriesNoViewParam(t *testing.T) {
	for _, view := range []string{ViewCompact, ViewDetailed, ""} {
		got := planSelfPath(view, "Z1", "feeding", "", "/animals/1#nav-plan")
		require.NotContains(t, got, "view=", "view=%q leaked into %q", view, got)
	}
	require.True(t, strings.HasPrefix(planSelfPath(ViewCompact, "", "", "", ""), "/care_plan"))
}
