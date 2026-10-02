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
// (tiers empty, kind chips present, zero stats).
func TestBuildDayPlanViewEmpty(t *testing.T) {
	plan := &DayPlan{Animals: &planAnimals{}}
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.NotNil(t, v)
	require.Equal(t, "10:30", v.UpdatedAt)
	for ti := range v.Tiers {
		require.Empty(t, v.Tiers[ti].Cards)
		require.Equal(t, 0, v.Tiers[ti].Count)
	}
	require.Empty(t, v.History)
	require.Empty(t, v.Feedings)
	require.Empty(t, v.Cares)
	require.Equal(t, FilterStats{LateCap: "0", NowCap: "0", LaterCap: "0"}, v.Stats)
	require.Len(t, v.Kinds, 6)
	require.Empty(t, v.Zones)
}

// TestBuildDayPlanViewMedTiers (fix 6): medication renders as TIER ROWS
// (no per-hour button series, no separate collapsible section). Compact
// folds each (source × animal) group to its next open action; zone and
// kind filters narrow the rows; non-medication kinds stay out of the
// med rows (feeding has its own section). The per-animal SLOT cards stay
// dashboard-only (TestBuildDashboardMedViewTodayOnly).
func TestBuildDayPlanViewMedTiers(t *testing.T) {
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
	done.Application = &careplan.ApplicationView{Status: "applied"}
	otherZone := testItem(medMorning, 3, careplan.StatusDue)
	otherZone.Occurrence.DueAt = time.Date(2026, 9, 28, 9, 0, 0, 0, time.Local)
	feeding := testItem(feed, 1, careplan.StatusDue)

	plan.Items = []careplan.PlanItem{morning, evening, done, otherZone, feeding}

	// compact, unfiltered: three open med rows — now: 08:30 (animal 1)
	// then 09:00 (animal 3), later: 18:00 (animal 1); the applied
	// occurrence is history. Feeding is its own section, never a row.
	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.Len(t, v.Tiers[1].Cards, 2, "due med rows in the now tier")
	require.Equal(t, 1, v.Tiers[1].Cards[0].AnimalID)
	require.Equal(t, "08:30", v.Tiers[1].Cards[0].DueHM)
	require.Equal(t, 3, v.Tiers[1].Cards[1].AnimalID)
	require.Len(t, v.Tiers[2].Cards, 1, "scheduled evening med row")
	require.Equal(t, 1, v.Tiers[2].Cards[0].AnimalID)
	require.Equal(t, "18:00", v.Tiers[2].Cards[0].DueHM)
	require.Len(t, v.History, 1, "applied med is history, undoable")
	require.Equal(t, 2, v.History[0].AnimalID)
	require.True(t, v.History[0].Undoable)
	require.Len(t, v.Feedings, 1)
	require.Equal(t, "Itra — 0.1 ml", v.Tiers[1].Cards[0].Detail, "drug — dosage detail line, marker stripped (U5)")
	require.Contains(t, v.Tiers[1].Cards[0].AnimalLink, "/animals/1?back=")

	// zone filter narrows to Z1 animals (1 and 2): rows of animal 3 gone
	v = BuildDayPlanView(plan, ViewCompact, "Z1", "", now)
	require.Len(t, v.Tiers[1].Cards, 1)
	require.Len(t, v.Tiers[2].Cards, 1)
	// S2 badge parity: the toggle-button badge equals the visible count
	// (2 rows + 1 feeding card)
	require.Equal(t, "3", v.ZoneCap)
	v = BuildDayPlanView(plan, ViewCompact, "Z2", "", now)
	require.Len(t, v.Tiers[1].Cards, 1)
	require.Equal(t, "1", v.ZoneCap)
	require.Equal(t, 3, v.Tiers[1].Cards[0].AnimalID)
	// no zone selected → button badge = all-zones cap
	v = BuildDayPlanView(plan, ViewCompact, "", "", now)
	require.Equal(t, v.ZoneAllCap, v.ZoneCap)

	// kind filter = medication keeps ONLY the med rows (feeding section gone)
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, now)
	require.Len(t, v.Tiers[1].Cards, 2)
	require.Len(t, v.Tiers[2].Cards, 1)
	require.Empty(t, v.Feedings)
	// kind = feeding: no med rows anywhere, feeding section only
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	for ti := range v.Tiers {
		require.Empty(t, v.Tiers[ti].Cards)
	}
	require.Len(t, v.Feedings, 1)

	// detailed: every open occurrence gets its own row — same med set,
	// one row per occurrence (morning 08:30, evening 18:00, Z2 09:00)
	d := BuildDayPlanView(plan, ViewDetailed, "", "", now)
	require.Len(t, d.Tiers[1].Cards, 2)
	require.Len(t, d.Tiers[2].Cards, 1)
}

// TestBuildDashboardMedViewTodayOnly: the dashboard mode (bugs.md R5-2a)
// TestSeriesTier pins the R3-5 tier placement rule: a series lands in its
// most urgent OPEN slot's tier (late beats now beats later), carries that
// slot's due time for the in-tier sort, and returns -1 when every slot is
// terminal (applied/skipped/deferred/overridden) — such a series renders
// no work line at all.
func TestSeriesTier(t *testing.T) {
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	slot := func(h int, status careplan.PlanStatus, done, overridden bool) MedSlotView {
		return MedSlotView{
			DueAt:      day.Add(time.Duration(h) * time.Hour),
			Status:     string(status),
			Done:       done,
			Overridden: overridden,
		}
	}
	series := func(slots ...MedSlotView) MedSeriesView {
		return MedSeriesView{Label: "Drug — 1 ml", Rows: []MedSeriesRow{{Slots: slots}}}
	}

	// most urgent wins: a late 08:00 slot beats a scheduled 18:00 one
	tier, first := seriesTier(series(
		slot(18, careplan.StatusScheduled, false, false),
		slot(8, careplan.StatusLate, false, false),
		slot(12, careplan.StatusDue, false, false),
	))
	require.Equal(t, 0, tier)
	require.Equal(t, day.Add(8*time.Hour), first)

	// no late slot → now tier with the earliest due slot
	tier, first = seriesTier(series(
		slot(18, careplan.StatusScheduled, false, false),
		slot(12, careplan.StatusDue, false, false),
		slot(10, careplan.StatusDue, false, false),
	))
	require.Equal(t, 1, tier)
	require.Equal(t, day.Add(10*time.Hour), first)

	// scheduled only → later tier
	tier, first = seriesTier(series(slot(18, careplan.StatusScheduled, false, false)))
	require.Equal(t, 2, tier)
	require.Equal(t, day.Add(18*time.Hour), first)

	// done and overridden slots never place a series
	tier, _ = seriesTier(series(
		slot(8, careplan.StatusApplied, true, false),
		slot(12, careplan.StatusDue, false, true),
	))
	require.Equal(t, -1, tier)
}

// TestFillMedTiers (R3-5): the medication kind renders one work line per
// (animal × drug series) inside the series' most urgent open tier, sorted
// by that slot's due time — the ordinary table-row tiers stay empty.
func TestFillMedTiers(t *testing.T) {
	day := time.Date(2026, 9, 28, 0, 0, 0, 0, time.Local)
	slot := func(h int, status careplan.PlanStatus) MedSlotView {
		return MedSlotView{DueAt: day.Add(time.Duration(h) * time.Hour), Status: string(status)}
	}
	mk := func(label string, slots ...MedSlotView) MedSeriesView {
		return MedSeriesView{Label: label, Rows: []MedSeriesRow{{Slots: slots}}}
	}
	groups := []MedGroupView{
		{
			AnimalID: 1, AnimalLabel: "Fox-1", AnimalLink: "/animals/1",
			Series: []MedSeriesView{
				mk("Late — 1 ml", slot(7, careplan.StatusLate)),
				mk("Now — 1 ml", slot(10, careplan.StatusDue)),
			},
		},
		{
			AnimalID: 2, AnimalLabel: "Owl-2", AnimalLink: "/animals/2",
			Series: []MedSeriesView{
				mk("LateB — 1 ml", slot(6, careplan.StatusLate)),
				mk("Later — 1 ml", slot(18, careplan.StatusScheduled)),
				mk("Done — 1 ml", MedSlotView{DueAt: day.Add(9 * time.Hour), Status: string(careplan.StatusApplied), Done: true}),
			},
		},
	}

	v := &DayPlanView{}
	v.fillMedTiers(groups)

	// late tier: LateB (06:00) before Late (07:00); now: Now; later: Later;
	// the all-done series renders no line. A line is the group narrowed to
	// its one series (the `_med_series` partial renders it unchanged).
	require.Len(t, v.MedTiers[0], 2)
	require.Equal(t, "LateB — 1 ml", v.MedTiers[0][0].Series[0].Label)
	require.Equal(t, "Owl-2", v.MedTiers[0][0].AnimalLabel)
	require.Equal(t, "Late — 1 ml", v.MedTiers[0][1].Series[0].Label)
	require.Equal(t, "Fox-1", v.MedTiers[0][1].AnimalLabel)
	require.Equal(t, "/animals/1", v.MedTiers[0][1].AnimalLink)
	require.Len(t, v.MedTiers[1], 1)
	require.Equal(t, "Now — 1 ml", v.MedTiers[1][0].Series[0].Label)
	require.Equal(t, 1, v.MedTiers[1][0].Series[0].Tier)
	require.Len(t, v.MedTiers[2], 1)
	require.Equal(t, "Later — 1 ml", v.MedTiers[2][0].Series[0].Label)
	require.Equal(t, 2, v.MedTiers[2][0].Series[0].Tier)
	require.Equal(t, day.Add(18*time.Hour), v.MedTiers[2][0].Series[0].FirstDueAt)
}

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
	// Dash-2 (round-2 §8.1): year-number-only button text — sibling-table
	// parity; the full label stays on the card/detail modal.
	require.Equal(t, "11/26", meds[0].AnimalYear)
	// Dash-7 (§8.2): the eye deep-link carries the occurrence reference
	// (query-escaped; url.Values orders due before item) + the hash.
	require.Contains(t, meds[0].Slots[0].DeepLink, "/animals/1?due=")
	require.Contains(t, meds[0].Slots[0].DeepLink, "item=rule%3Amed-1#nav-treatment")

	// /care_plan tier projection over the same plan (fix 6): compact
	// folds the three open occurrences of (med-1 × animal 1) into ONE
	// late row with a "+2" remaining badge — no per-slot button series.
	v := BuildDayPlanView(plan, ViewCompact, "", careplan.KindMedication, now)
	require.Len(t, v.Tiers[0].Cards, 1, "one folded row: yesterday's late slot is the next open action")
	require.Equal(t, 1, v.Tiers[0].Cards[0].AnimalID)
	require.Equal(t, 2, v.Tiers[0].Cards[0].Remaining, "due today + scheduled tomorrow fold into the +N badge")
	require.Len(t, v.History, 1, "today's applied slot is history; overridden stays hidden in compact")
	require.Equal(t, 1, v.History[0].AnimalID)
	// detailed: every open occurrence is its own tier row; overridden
	// surfaces in the history section only in this density.
	dv := BuildDayPlanView(plan, ViewDetailed, "", careplan.KindMedication, now)
	require.Len(t, dv.Tiers[0].Cards, 1)
	require.Len(t, dv.Tiers[1].Cards, 1, "today's due slot")
	require.Len(t, dv.Tiers[2].Cards, 1, "tomorrow's scheduled slot")
	require.Len(t, dv.History, 2, "applied + overridden (detailed debug surface)")
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
