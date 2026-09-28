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

// TestBuildDayPlanViewMedGroups: compact view projects every medication
// occurrence into per-animal cards (bugs.md rework). One card per animal
// holding ALL its slots (morning/noon/evening buckets from the due hour,
// §6.2); applied slots stay visible (state visible, undoable); zone filter
// narrows the cards; non-medication kinds never leak in.
func TestBuildDayPlanViewMedGroups(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	medMorning := testSource(careplan.KindMedication, "med-M", "Itra (conversion)", map[string]interface{}{"drug": "Itra", "dosage": "0.1 ml"})
	medEvening := testSource(careplan.KindMedication, "med-E", "Baytril", map[string]interface{}{"drug": "Baytril", "dosage": "0.2 ml"})
	feed := testSource(careplan.KindFeeding, "feed-1", "Feed", map[string]interface{}{"food": "grenouilles"})

	morning := testItem(medMorning, 1, careplan.StatusDue)
	morning.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 30, 0, 0, time.Local)
	evening := testItem(medEvening, 1, careplan.StatusScheduled)
	evening.Occurrence.DueAt = time.Date(2026, 9, 28, 18, 0, 0, 0, time.Local)
	done := testItem(medMorning, 2, careplan.StatusApplied)
	done.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	otherZone := testItem(medMorning, 3, careplan.StatusDue)
	otherZone.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	feeding := testItem(feed, 1, careplan.StatusDue)

	plan.Items = []careplan.PlanItem{morning, evening, done, otherZone, feeding}

	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.Len(t, v.Meds, 3, "one medication card per animal (1, 2, 3)")

	// groups sort by animal ID
	require.Equal(t, 1, v.Meds[0].AnimalID)
	require.Equal(t, 2, v.Meds[1].AnimalID)
	require.Equal(t, 3, v.Meds[2].AnimalID)

	// animal 1: two slots sorted by due time — morning first, evening last;
	// feeding item must not leak into the medication card
	g1 := v.Meds[0]
	require.Len(t, g1.Slots, 2)
	require.Equal(t, "morning", g1.Slots[0].Slot)
	require.Equal(t, "evening", g1.Slots[1].Slot)
	require.Equal(t, "Itra", g1.Slots[0].SourceName, "conversion marker stripped (U5)")
	require.Equal(t, "Itra — 0.1 ml", g1.Slots[0].Detail)
	require.Equal(t, 2, g1.OpenCount, "both open slots count (due morning + scheduled evening — still to give today)")
	require.False(t, g1.Slots[0].Done)
	require.Contains(t, g1.AnimalLink, "/animals/1?back=")

	// animal 2: applied slot stays visible with Done/Applied flags
	g2 := v.Meds[1]
	require.Len(t, g2.Slots, 1)
	require.True(t, g2.Slots[0].Applied)
	require.True(t, g2.Slots[0].Done)
	require.Equal(t, 0, g2.OpenCount)

	// zone filter narrows to Z1 animals (1 and 2)
	v = BuildDayPlanView(plan, ViewCompact, "Z1", "", now)
	require.Len(t, v.Meds, 2)
	v = BuildDayPlanView(plan, ViewCompact, "Z2", "", now)
	require.Len(t, v.Meds, 1)
	require.Equal(t, 3, v.Meds[0].AnimalID)

	// kind filter ≠ medication hides the medication section entirely
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	require.Empty(t, v.Meds)
	// kind = medication keeps it
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, now)
	require.Len(t, v.Meds, 3)
}

// TestMedSlotOf: bucket boundaries match treatmentBucketBit (§6.2) so the
// card toggle lines up with the treatments bitmap written at apply time.
func TestMedSlotOf(t *testing.T) {
	at := func(h int) time.Time { return time.Date(2026, 9, 28, h, 0, 0, 0, time.Local) }
	require.Equal(t, "morning", medSlotOf(at(0)))
	require.Equal(t, "morning", medSlotOf(at(10)))
	require.Equal(t, "noon", medSlotOf(at(11)))
	require.Equal(t, "noon", medSlotOf(at(15)))
	require.Equal(t, "evening", medSlotOf(at(16)))
	require.Equal(t, "evening", medSlotOf(at(23)))
}

// TestBuildDayPlanViewFeedingDedup: several OPEN occurrences of the same
// animal × same source (a missed morning slot + the noon slot) must render
// ONE chip per card (bugs.md "pure duplicated" rework) — the earliest open
// one, with a single applicable ref, so one click never writes N records
// for the same feeding.
func TestBuildDayPlanViewFeedingDedup(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 13, 0, 0, 0, time.Local)
	src := testSource(careplan.KindFeeding, "src-A", "Feed", map[string]interface{}{"food": "grenouilles"})

	missed := testItem(src, 1, careplan.StatusMissing)
	missed.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local)
	due := testItem(src, 1, careplan.StatusDue)
	due.Occurrence.DueAt = time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	late := testItem(src, 1, careplan.StatusLate)
	late.Occurrence.DueAt = time.Date(2026, 9, 28, 10, 0, 0, 0, time.Local)
	other := testItem(src, 2, careplan.StatusDue)
	other.Occurrence.DueAt = time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local)
	// scheduled future slot of animal 1 must not count as the kept occurrence
	sched := testItem(src, 1, careplan.StatusScheduled)
	sched.Occurrence.DueAt = time.Date(2026, 9, 28, 18, 0, 0, 0, time.Local)

	plan.Items = []careplan.PlanItem{due, missed, late, other, sched}

	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.Len(t, v.Feedings, 1)
	fc := v.Feedings[0]
	require.Len(t, fc.Chips, 2, "one chip per animal — duplicates deduped")
	require.Equal(t, 2, fc.ApplicableCount)
	// animal 1's kept chip is the EARLIEST open occurrence (08:00 missing)
	var c1 *FeedingChip
	for i := range fc.Chips {
		if fc.Chips[i].AnimalID == 1 {
			c1 = &fc.Chips[i]
		}
	}
	require.NotNil(t, c1)
	require.Equal(t, "missing", c1.Status)
	require.Equal(t, time.Date(2026, 9, 28, 8, 0, 0, 0, time.Local), c1.DueAt)
	// exactly one ref for animal 1 in the batch payload (the 08:00 one)
	require.Contains(t, fc.ChipRefsJSON, `"due_at":"2026-09-28T08:00:00`)
	require.NotContains(t, fc.ChipRefsJSON, `"due_at":"2026-09-28T10:00:00`)
	require.NotContains(t, fc.ChipRefsJSON, `"due_at":"2026-09-28T12:00:00+02:00","source_id":"src-A","animal_id":1`)
}
