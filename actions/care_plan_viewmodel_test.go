package actions

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// TestBadgeCap covers the 99+ badge cap (bugs.md U2): a three-digit badge
// breaks pill layout; negative counts never surface.
func TestBadgeCap(t *testing.T) {
	require.Equal(t, "0", BadgeCap(0))
	require.Equal(t, "1", BadgeCap(1))
	require.Equal(t, "42", BadgeCap(42))
	require.Equal(t, "99", BadgeCap(99))
	require.Equal(t, "99+", BadgeCap(100))
	require.Equal(t, "99+", BadgeCap(12345))
	require.Equal(t, "0", BadgeCap(-3))
}

// TestTierOrder pins the U2 mapping: missing|late→late, due→now,
// scheduled→later, applied|skipped|deferred→done, overridden excluded.
func TestTierOrder(t *testing.T) {
	require.Equal(t, 0, tierOrder(careplan.StatusMissing))
	require.Equal(t, 0, tierOrder(careplan.StatusLate))
	require.Equal(t, 1, tierOrder(careplan.StatusDue))
	require.Equal(t, 2, tierOrder(careplan.StatusScheduled))
	require.Equal(t, 3, tierOrder(careplan.StatusApplied))
	require.Equal(t, 3, tierOrder(careplan.StatusSkipped))
	require.Equal(t, 3, tierOrder(careplan.StatusDeferred))
	require.Equal(t, -1, tierOrder(careplan.StatusOverridden))
}

// TestBuildDayPlanViewEmpty: an empty plan still yields a renderable view
// (four tiers keyed, kind chips present, no hours).
func TestBuildDayPlanViewEmpty(t *testing.T) {
	plan := &DayPlan{Animals: &planAnimals{}}
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.NotNil(t, v)
	require.Equal(t, "10:30", v.UpdatedAt)
	require.Equal(t, [4]string{TierLate, TierNow, TierLater, TierDone},
		[4]string{v.Tiers[0].Key, v.Tiers[1].Key, v.Tiers[2].Key, v.Tiers[3].Key})
	for _, tier := range v.Tiers {
		require.Equal(t, 0, tier.Count)
		require.Equal(t, "0", tier.CountCap)
	}
	require.Len(t, v.Kinds, 6)
	require.Empty(t, v.Hours)
	require.Empty(t, v.Zones)
}
