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

// TestBuildDashboardMedViewTodayOnly: the dashboard mode (bugs.md R5-2a)
// collapses a 4-day plan window to today's slots only — no yesterday, no
// tomorrow — suppresses Overridden occurrences and counts only open
// today slots (honest badge). The /care_plan projection is unchanged:
// every window slot stays visible there, overridden included.
func TestBuildDashboardMedViewTodayOnly(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	from, to := TodayPlanWindow(now)
	require.Equal(t, time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local), from)
	require.Equal(t, time.Date(2026, 9, 28, 23, 59, 59, 999999999, time.Local), to)

	med := testSource(careplan.KindMedication, "med-1", "Citramox", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	plan := testPlan()
	plan.From, plan.To, plan.Now = from, to, now

	todayDue := testItem(med, 1, careplan.StatusDue)
	todayDue.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 30, 0, 0, time.Local)
	todayApplied := testItem(med, 1, careplan.StatusApplied)
	todayApplied.Occurrence.DueAt = time.Date(2026, 9, 28, 7, 0, 0, 0, time.Local)
	yesterday := testItem(med, 1, careplan.StatusLate)
	yesterday.Occurrence.DueAt = time.Date(2026, 9, 27, 18, 0, 0, 0, time.Local)
	tomorrow := testItem(med, 1, careplan.StatusScheduled)
	tomorrow.Occurrence.DueAt = time.Date(2026, 9, 29, 8, 30, 0, 0, time.Local)
	overridden := testItem(med, 2, careplan.StatusOverridden)
	overridden.Occurrence.DueAt = time.Date(2026, 9, 28, 8, 30, 0, 0, time.Local)
	plan.Items = []careplan.PlanItem{yesterday, todayApplied, todayDue, tomorrow, overridden}

	// Dashboard mode: today only, overridden suppressed — one card
	// (animal 1) with exactly two slots and OpenCount 1.
	meds := BuildDashboardMedView(plan, "")
	require.Len(t, meds, 1, "animal 2 card disappears (its only slot is overridden)")
	require.Equal(t, 1, meds[0].AnimalID)
	require.Len(t, meds[0].Slots, 2, "today slots only")
	require.Equal(t, 1, meds[0].OpenCount, "honest today count: one open slot")
	require.False(t, meds[0].Slots[1].Applied)
	require.True(t, meds[0].Slots[0].Applied)
	// R5-2b: unconditional view link — dashboard mode points back at /
	require.Contains(t, meds[0].Slots[0].ViewLink, "back=%2F#nav-treatment",
		"no fulfillment yet: view link targets the animal Treatment tab, back=dashboard")
	require.Contains(t, meds[0].Slots[0].ViewLink, "/animals/1?")

	// /care_plan mode over the same plan: unchanged — every window slot
	// stays visible, overridden included.
	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, now)
	require.Len(t, v.Meds, 2, "animal 1 (3 slots) + animal 2 (overridden kept)")
	g1 := v.Meds[0]
	require.Len(t, g1.Slots, 4, "yesterday + 2 today + tomorrow slots (4-day window keeps everything)")
	require.Equal(t, 3, g1.OpenCount, "work-screen count unchanged: late + due + scheduled open across the 4-day window")
	g2 := v.Meds[1]
	require.Equal(t, 2, g2.AnimalID)
	require.Len(t, g2.Slots, 1, "overridden slot still surfaces on the work screen")
	require.True(t, g2.Slots[0].Overridden)
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
	// animal 1's kept chip is the earliest CURRENT occurrence: the missed
	// 08:00 slot is superseded by its open successors (§6.2-2/3), the due
	// noon slot is the actionable work. Superseded ones resurface in the
	// history section via the late-record path (WP4).
	var c1 *FeedingChip
	for i := range fc.Chips {
		if fc.Chips[i].AnimalID == 1 {
			c1 = &fc.Chips[i]
		}
	}
	require.NotNil(t, c1)
	require.Equal(t, "due", c1.Status)
	require.Equal(t, time.Date(2026, 9, 28, 12, 0, 0, 0, time.Local), c1.DueAt)
	// exactly one ref for animal 1 in the batch payload (the 12:00 one)
	require.Contains(t, fc.ChipRefsJSON, `"animal_id":1,"due_at":"2026-09-28T12:00:00`)
	require.NotContains(t, fc.ChipRefsJSON, `"animal_id":1,"due_at":"2026-09-28T08:00:00`)
	require.NotContains(t, fc.ChipRefsJSON, `"animal_id":1,"due_at":"2026-09-28T10:00:00`)
}

// TestCardNeedsInput (bugs.md R5-2c, D-a): the unified confirm policy —
// a card opens the confirm dialog ONLY when applying it needs user input:
// weighing → weight, observation → answer. Feeding, medication (dosage is
// optional; a required dosage arrives as a 422 dosage_required), care and
// cleanup apply instantly with no modal.
func TestCardNeedsInput(t *testing.T) {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	kinds := map[string]bool{
		// kind → needs input
		careplan.KindFeeding:     false,
		careplan.KindMedication:  false,
		careplan.KindCare:        false,
		careplan.KindCleanup:     false,
		careplan.KindWeighing:    true,
		careplan.KindObservation: true,
	}
	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	for kind, want := range kinds {
		src := testSource(kind, "src-"+kind, "S "+kind, map[string]interface{}{"food": "x"})
		it := testItem(src, 1, careplan.StatusDue)
		cv := v.cardFor(plan, &it)
		require.Equal(t, want, cv.NeedsInput, "kind %s", kind)
	}
}
