package actions

// Phase-0b §6 table tests for the ONE generic tier distribution:
// fillTierBuckets over the Openable interface, exercised on a minimal
// fake plus the three real section types (CardView rows, FeedingGroupView,
// CareView) — same bucketing, same skip rule (-1/>2), per-kind sort.

import (
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// fakeOpenable is the smallest Openable: a label to trace bucketing and
// a tier/due pair straight from the table row.
type fakeOpenable struct {
	label string
	tier  int
	due   time.Time
	open  int
}

func (f fakeOpenable) TierOf() int         { return f.tier }
func (f fakeOpenable) FirstDue() time.Time { return f.due }
func (f fakeOpenable) OpenCount() int      { return f.open }

func TestFillTierBucketsGeneric(t *testing.T) {
	now := time.Now()
	hour := time.Hour
	cases := []struct {
		name string
		in   []fakeOpenable
		want [3][]string // labels per bucket, in sorted order
	}{
		{
			name: "empty input",
			in:   nil,
			want: [3][]string{},
		},
		{
			name: "every tier one item",
			in: []fakeOpenable{
				{"later", 2, now.Add(2 * hour), 1},
				{"late", 0, now.Add(-hour), 1},
				{"now", 1, now, 1},
			},
			want: [3][]string{{"late"}, {"now"}, {"later"}},
		},
		{
			name: "no-open-work tiers are skipped",
			in: []fakeOpenable{
				{"minus", -1, now, 0},
				{"done", 3, now, 0},
				{"beyond", 4, now, 0},
				{"now", 1, now, 1},
			},
			want: [3][]string{{}, {"now"}, {}},
		},
		{
			name: "within a tier the sort is stable and by due time",
			in: []fakeOpenable{
				{"b-later", 1, now.Add(hour), 1},
				{"a-first", 1, now, 1},
				{"c-tie-1", 1, now.Add(30 * time.Minute), 1},
				{"d-tie-2", 1, now.Add(30 * time.Minute), 1},
			},
			want: [3][]string{{}, {"a-first", "c-tie-1", "d-tie-2", "b-later"}, {}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := fillTierBuckets(tc.in, func(a, b fakeOpenable) bool {
				return a.due.Before(b.due)
			})
			for i := 0; i < 3; i++ {
				labels := make([]string, 0, len(got[i]))
				for _, it := range got[i] {
					labels = append(labels, it.label)
				}
				if len(tc.want[i]) == 0 {
					require.Empty(t, labels, "bucket %d", i)
					continue
				}
				require.Equal(t, tc.want[i], labels, "bucket %d", i)
			}
		})
	}
}

// TestFillTierBucketsRealTypes pins the three production Openable
// implementations through the same generic fill: identical skip rule,
// identical bucketing, per-kind tiebreak at the call site.
func TestFillTierBucketsRealTypes(t *testing.T) {
	now := time.Now()
	hour := time.Hour

	t.Run("CardView rows: tier from status, due-time order", func(t *testing.T) {
		rows := []CardView{
			{AnimalLabel: "done", Status: "applied", DueAt: now},
			{AnimalLabel: "late", Status: "late", DueAt: now.Add(-hour)},
			{AnimalLabel: "now-b", Status: "due", DueAt: now.Add(20 * time.Minute)},
			{AnimalLabel: "now-a", Status: "due", DueAt: now.Add(10 * time.Minute)},
			{AnimalLabel: "later", Status: "scheduled", DueAt: now.Add(26 * hour)},
		}
		got := fillTierBuckets(rows, func(a, b CardView) bool { return a.DueAt.Before(b.DueAt) })
		require.Len(t, got[0], 1)
		require.Equal(t, "late", got[0][0].AnimalLabel)
		require.Equal(t, []string{"now-a", "now-b"}, []string{got[1][0].AnimalLabel, got[1][1].AnimalLabel})
		require.Len(t, got[2], 1)
		require.Equal(t, "later", got[2][0].AnimalLabel)
		// CardView badge unit: one row = one open action.
		require.Equal(t, 1, got[0][0].OpenCount())
	})

	t.Run("FeedingGroupView: zero-due sorts last, cage breaks ties", func(t *testing.T) {
		groups := []FeedingGroupView{
			{Cage: "C-Z", Tier: 1, FirstDueAt: now.Add(hour), ApplicableCount: 3},
			{Cage: "C-none", Tier: 1, ApplicableCount: 0}, // zero FirstDueAt → last
			{Cage: "C-A", Tier: 1, FirstDueAt: now.Add(hour), ApplicableCount: 1},
			{Cage: "C-late", Tier: 0, FirstDueAt: now.Add(-hour), ApplicableCount: 2},
		}
		got := fillTierBuckets(groups, func(fa, fb FeedingGroupView) bool {
			if fa.FirstDueAt.IsZero() != fb.FirstDueAt.IsZero() {
				return fb.FirstDueAt.IsZero()
			}
			if fa.FirstDueAt.Equal(fb.FirstDueAt) {
				return fa.Cage < fb.Cage
			}
			return fa.FirstDueAt.Before(fb.FirstDueAt)
		})
		require.Equal(t, "C-late", got[0][0].Cage)
		require.Equal(t, []string{"C-A", "C-Z", "C-none"},
			[]string{got[1][0].Cage, got[1][1].Cage, got[1][2].Cage})
		require.Empty(t, got[2])
		// Feeding badge unit: occurrences (applicable chips) — C-Z carries 3.
		require.Equal(t, 3, got[1][1].OpenCount())
	})

	t.Run("CareView: late occurrences pull the row into the late tier", func(t *testing.T) {
		// Phase 3 / D3: TierOf() now returns the STAMPED Tier (fillCareTiers
		// derives it from the most urgent open occurrence via careRowTier,
		// tested in TestBuildDayPlanViewFillsCareTiers). fillTierBuckets
		// distributes on that stamped tier — mirror the feeding subtest and
		// stamp Tier explicitly.
		cares := []CareView{
			{Cage: "B", SourceName: "daily", Tier: 1, ApplicableCount: 2},
			{Cage: "A", SourceName: "weekly", Tier: 0, ApplicableCount: 1, LateCount: 1},
			{Cage: "C", SourceName: "daily", Tier: 1, ApplicableCount: 4},
		}
		got := fillTierBuckets(cares, func(a, b CareView) bool {
			if a.Cage != b.Cage {
				return a.Cage < b.Cage
			}
			return a.SourceName < b.SourceName
		})
		require.Equal(t, "A", got[0][0].Cage)
		require.Equal(t, []string{"B", "C"}, []string{got[1][0].Cage, got[1][1].Cage})
		require.Empty(t, got[2])
		require.Equal(t, 4, got[1][1].OpenCount())
	})
}

// TestBuildDayPlanViewFillsCareTiers proves the cleanup rows reach the
// shared distribution on the real view-model path (fillCareTiers wired
// into BuildDayPlanView), while the flat Cares list the template renders
// stays untouched — the screen does not change.
func TestBuildDayPlanViewFillsCareTiers(t *testing.T) {
	plan := testPlan()
	clean := testSource(careplan.KindCleanup, "src-clean", "Clean", map[string]interface{}{"note": "désinfecter"})
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{
		testItem(clean, 1, careplan.StatusLate),
		testItem(clean, 2, careplan.StatusDue),
	}

	v := BuildDayPlanView(plan, ViewCompact, "", "cleanup", "", now)
	total := 0
	for i := 0; i < 3; i++ {
		total += len(v.CareTiers[i])
	}
	require.Equal(t, len(v.Cares), total,
		"every rendered care row appears in exactly one tier bucket")
	require.NotEmpty(t, v.Cares, "fixture must produce the flat cleanup rows")
	for i := 0; i < 3; i++ {
		for _, cv := range v.CareTiers[i] {
			require.Equal(t, i, cv.TierOf(), "bucket %d holds only its own tier", i)
		}
	}
	// One cage × one source → one row; the late occurrence pulls it into
	// the late tier.
	require.Equal(t, 1, len(v.CareTiers[0]))
	require.Empty(t, v.CareTiers[1])
	require.Empty(t, v.CareTiers[2])
}
