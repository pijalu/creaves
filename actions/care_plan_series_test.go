package actions

import (
	"os"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/gobuffalo/plush/v5"
	"github.com/stretchr/testify/require"
)

// Round-2 §8.3 drug-series component tests (CP1/Dash-8/T2): (drug,
// dosage) merge, 3-per-row bucket chunking and morning/noon/evening
// ordering with the <11h / <15h boundaries.

var seriesBucketOrder = map[string]int{"morning": 0, "noon": 1, "evening": 2}

func medSlot(detail, slot string, due time.Time) MedSlotView {
	return MedSlotView{
		Detail:  detail,
		Slot:    slot,
		DueAt:   due,
		DueAtHM: due.Format("15:04"),
	}
}

// TestMedSeriesGroupsByDrugAndDosage: occurrences of the same animal
// sharing (drug, dosage) merge into ONE series even across different
// sources; a differing dosage stays a separate line.
func TestMedSeriesGroupsByDrugAndDosage(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	day := func(h, m int) time.Time { return time.Date(2026, 9, 28, h, m, 0, 0, time.Local) }

	citA := testSource(careplan.KindMedication, "cit-A", "Citramox AM", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	citB := testSource(careplan.KindMedication, "cit-B", "Citramox PM", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})
	other := testSource(careplan.KindMedication, "itr-C", "Itra", map[string]interface{}{"drug": "Citramox", "dosage": "1 ml"})

	s1 := testItem(citA, 1, careplan.StatusDue)
	s1.Occurrence.DueAt = day(8, 0)
	s2 := testItem(citB, 1, careplan.StatusDue)
	s2.Occurrence.DueAt = day(18, 0)
	s3 := testItem(citA, 1, careplan.StatusDue)
	s3.Occurrence.DueAt = day(12, 0)
	dos := testItem(other, 1, careplan.StatusDue)
	dos.Occurrence.DueAt = day(9, 0)
	plan.Items = []careplan.PlanItem{s1, s2, s3, dos}

	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	meds := v.buildMedGroups(plan, "", false) // per-animal slot cards = dashboard projection
	require.Len(t, meds, 1)
	series := meds[0].Series
	require.Len(t, series, 2, "(drug, dosage) merge: 0.5 ml once, 1 ml separate")
	require.Equal(t, "Citramox — 0.5 ml", series[0].Label)
	require.Equal(t, "Citramox — 1 ml", series[1].Label)

	// 0.5 ml series holds the three occurrences of BOTH sources
	var half *MedSeriesView
	for i := range series {
		if series[i].Key == "Citramox — 0.5 ml" {
			half = &series[i]
		}
	}
	require.NotNil(t, half)
	n := 0
	for _, row := range half.Rows {
		n += len(row.Slots)
	}
	require.Equal(t, 3, n, "all occurrences of (Citramox, 0.5 ml) on one line")

	// backward compat: Slots still carries every occurrence flat
	require.Len(t, meds[0].Slots, 4)
}

// TestMedSeriesChunksThreePerRowByBucket: five morning occurrences chunk
// 3+2; a bucket change always starts a new row even when the last row
// has room.
func TestMedSeriesChunksThreePerRowByBucket(t *testing.T) {
	day := func(h, m int) time.Time { return time.Date(2026, 9, 28, h, m, 0, 0, time.Local) }
	slots := []MedSlotView{
		medSlot("Citramox — 0.5 ml", "morning", day(8, 0)),
		medSlot("Citramox — 0.5 ml", "morning", day(8, 30)),
		medSlot("Citramox — 0.5 ml", "morning", day(9, 0)),
		medSlot("Citramox — 0.5 ml", "morning", day(9, 30)),
		medSlot("Citramox — 0.5 ml", "morning", day(10, 0)),
		medSlot("Citramox — 0.5 ml", "noon", day(12, 0)),
	}
	series := seriesOf(slots, seriesBucketOrder)
	require.Len(t, series, 1)
	rows := series[0].Rows
	require.Len(t, rows, 3, "3 + 2 morning, noon forces its own row")
	require.Equal(t, []int{3, 2, 1}, []int{len(rows[0].Slots), len(rows[1].Slots), len(rows[2].Slots)})
	for _, row := range rows {
		for _, s := range row.Slots {
			require.Equal(t, row.Slots[0].Slot, s.Slot, "a row never mixes buckets")
		}
	}
	require.Equal(t, "morning", rows[0].Slots[0].Slot)
	require.Equal(t, "morning", rows[1].Slots[0].Slot)
	require.Equal(t, "noon", rows[2].Slots[0].Slot)
	require.False(t, rows[0].DividerBefore)
	require.False(t, rows[1].DividerBefore, "same bucket, no divider")
	require.True(t, rows[2].DividerBefore, "bucket change divides")
}

// TestMedSeriesSlotOrderMatchesBuckets: within a series the slots are
// bucket-major (morning <11h, noon <15h, evening — same boundaries as
// medSlotOf), due time within a bucket.
func TestMedSeriesSlotOrderMatchesBuckets(t *testing.T) {
	day := func(h, m int) time.Time { return time.Date(2026, 9, 28, h, m, 0, 0, time.Local) }
	slots := []MedSlotView{
		medSlot("Citramox — 0.5 ml", "evening", day(18, 0)),
		medSlot("Citramox — 0.5 ml", "morning", day(8, 0)),
		medSlot("Citramox — 0.5 ml", "noon", day(12, 0)),
		medSlot("Citramox — 0.5 ml", "morning", day(8, 0).AddDate(0, 0, 1)),
	}
	series := seriesOf(slots, seriesBucketOrder)
	require.Len(t, series, 1)
	var got []string
	for _, row := range series[0].Rows {
		for _, s := range row.Slots {
			got = append(got, s.Slot+" "+s.DueAt.Format("01-02 15:04"))
		}
	}
	require.Equal(t, []string{
		"morning 09-28 08:00",
		"morning 09-29 08:00",
		"noon 09-28 12:00",
		"evening 09-28 18:00",
	}, got)
}

// TestMedSeriesPartialRenders: the shared `_med_series` partial (all four
// locale forks) parses and renders every button state — applied (live
// undo), open apply, late-recordable dimmed, skipped, deferred — plus the
// bucket divider between rows of different buckets.
func TestMedSeriesPartialRenders(t *testing.T) {
	mg := MedGroupView{
		AnimalID:    1,
		AnimalLabel: "472/26",
		Series: []MedSeriesView{{
			Key:   "Citramox — 0.5 ml",
			Label: "Citramox — 0.5 ml",
			Rows: []MedSeriesRow{
				{Slots: []MedSlotView{
					{Slot: "morning", Detail: "Citramox — 0.5 ml", DueAtHM: "08:00", Status: "applied", Done: true, Applied: true, CanUndo: true,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T08:00:00+02:00"},
					{Slot: "morning", Detail: "Citramox — 0.5 ml", DueAtHM: "09:00", Status: "late", Applicable: false, LateAllowed: true,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T09:00:00+02:00", SourceName: "Citramox AM"},
					{Slot: "morning", Detail: "Citramox — 0.5 ml", DueAtHM: "10:00", Status: "scheduled", Applicable: false,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T10:00:00+02:00"},
				}},
				{DividerBefore: true, Slots: []MedSlotView{
					{Slot: "noon", Detail: "Citramox — 0.5 ml", DueAtHM: "12:00", Status: "due", Applicable: true,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T12:00:00+02:00", SourceName: "Citramox AM"},
					{Slot: "noon", Detail: "Citramox — 0.5 ml", DueAtHM: "12:30", Status: "skipped", Done: true,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T12:30:00+02:00"},
					{Slot: "noon", Detail: "Citramox — 0.5 ml", DueAtHM: "13:00", Status: "deferred", Done: true,
						SourceType: "rule", SourceID: "a", DueAtRFC: "2026-09-28T13:00:00+02:00"},
				}},
			},
		}},
	}
	forks := []string{
		"../templates/care_plan/_med_series.plush.html",
		"../templates/care_plan/_med_series.plush.de.html",
		"../templates/care_plan/_med_series.plush.fr.html",
		"../templates/care_plan/_med_series.plush.nl.html",
	}
	for _, f := range forks {
		raw, err := os.ReadFile(f)
		require.NoError(t, err, f)
		ctx := plush.NewContextWith(map[string]interface{}{
			"mg": mg,
			"t":  func(s string) string { return s },
		})
		out, err := plush.Render(string(raw), ctx)
		require.NoError(t, err, f)

		require.Contains(t, out, "Citramox — 0.5 ml", f)
		require.Contains(t, out, "✓ 08:00", f)            // applied, undoable
		require.Contains(t, out, "plan-unapply-btn", f)   // undo hook present
		require.Contains(t, out, "data-late=\"true\"", f) // late-recordable dimmed
		require.Contains(t, out, "– 09:00", f)            // late button
		require.Contains(t, out, "○ 12:00", f)            // open apply
		require.Contains(t, out, "plan-apply-btn", f)     // apply hook present
		require.Contains(t, out, "⊘ 12:30", f)            // skipped
		require.Contains(t, out, "⏸ 13:00", f)            // deferred
		require.Contains(t, out, "– 10:00", f)            // out of window, locked
		require.Contains(t, out, "med-series-divider", f) // bucket divider
		require.NotContains(t, out, "<%= for (", f, "unrendered plush tag")
		require.Contains(t, out, `data-animal-id="1"`, f, "apply hooks carry the animal")
		require.Contains(t, out, `data-due-at="2026-09-28T12:00:00+02:00"`, f)
	}
}

// TestMedSlotLateAllowed: the A1 plumbing — a past-due, unapplied,
// non-applicable occurrence flags LateAllowed (the dimmed series button
// records it with the late acknowledgment); future and done slots never.
func TestMedSlotLateAllowed(t *testing.T) {
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan := testPlan()
	plan.Now = now
	day := func(h, m int) time.Time { return time.Date(2026, 9, 28, h, m, 0, 0, time.Local) }
	med := testSource(careplan.KindMedication, "med-1", "Med", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	late := testItem(med, 1, careplan.StatusLate)
	late.Occurrence.DueAt = day(7, 0)
	late.Applicable = false // apply window passed
	future := testItem(med, 1, careplan.StatusDue)
	future.Occurrence.DueAt = day(14, 0)
	done := testItem(med, 1, careplan.StatusApplied)
	done.Occurrence.DueAt = day(7, 30)
	done.Applicable = false
	plan.Items = []careplan.PlanItem{late, future, done}

	v := BuildDayPlanView(plan, ViewCompact, "", "", now)
	meds := v.buildMedGroups(plan, "", false)
	require.Len(t, meds, 1)
	slots := meds[0].Slots
	require.Len(t, slots, 3)
	byTime := map[string]bool{}
	for _, s := range slots {
		byTime[s.DueAtHM] = s.LateAllowed
	}
	require.True(t, byTime["07:00"], "past-due out-of-window: late-recordable")
	require.False(t, byTime["14:00"], "future in window: normal apply")
	require.False(t, byTime["07:30"], "done: never late-recordable")
}
