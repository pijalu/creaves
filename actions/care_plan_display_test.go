package actions

import (
	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// Round-2 display layer unit tests (docs/care-plan-ux-fix-plan-round2.md
// §6.2, §4b-A1/A2): current-vs-superseded semantics, date-aware labels and
// the feeding chip dedupe. DB-free — everything derives from PlanItem.

// dispSource builds a feeding source with the default windows
// (grace 60m / miss 24h / lookahead 60m, §4.3).
func dispSource(id string) *careplan.Source {
	return careplan.NewSource(careplan.SourceRule, id, "R-"+id, careplan.KindFeeding,
		careplan.Schedule{GraceMinutes: 60, MissAfterHours: 24, LookaheadMinutes: 60})
}

func dispItem(status careplan.PlanStatus, due, nextDue time.Time, applicable bool) *careplan.PlanItem {
	return &careplan.PlanItem{
		Occurrence: careplan.Occurrence{
			Source:   dispSource("r1"),
			AnimalID: 1,
			DueAt:    due,
		},
		Status:     status,
		Applicable: applicable,
		NextDue:    nextDue,
	}
}

func at(y, m, d, hh, mm int) time.Time {
	return time.Date(y, time.Month(m), d, hh, mm, 0, 0, time.Local)
}

// modelsAnimalForPlanTests builds a minimal animal row for GroupCards
// (labels/zone/cage only — no DB).
func modelsAnimalForPlanTests(id int) models.Animal {
	return models.Animal{ID: id, Species: "CP-A"}
}

// §6.2-1: current while the successor is more than one lookahead away.
func TestIsCurrentUntilSuccessorImminent(t *testing.T) {
	now := at(2026, 9, 30, 10, 0)

	// 08:00 slot, successor 12:00 — 2 h gap > 60 m lookahead → current.
	cur := dispItem(careplan.StatusLate, at(2026, 9, 30, 8, 0), at(2026, 9, 30, 12, 0), true)
	require.True(t, IsCurrent(cur, now), "2h to successor must stay current")
	require.Empty(t, SupersededReason(cur, now))

	// Successor imminent (30 m) → leaves the work set.
	imm := dispItem(careplan.StatusLate, at(2026, 9, 30, 8, 0), at(2026, 9, 30, 10, 30), true)
	require.False(t, IsCurrent(imm, now))
	require.Equal(t, "replaced", SupersededReason(imm, now))

	// The window's last occurrence (zero NextDue) stays current while applicable.
	last := dispItem(careplan.StatusDue, at(2026, 9, 30, 8, 0), time.Time{}, true)
	require.True(t, IsCurrent(last, now))
}

// §6.2-2: replaced vs lapsed; current items have no reason.
func TestSupersededReasonReplacedVsWindow(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)

	replaced := dispItem(careplan.StatusLate, at(2026, 9, 30, 8, 0), at(2026, 9, 30, 12, 0), false)
	require.Equal(t, "replaced", SupersededReason(replaced, now))

	lapsed := dispItem(careplan.StatusMissing, at(2026, 9, 28, 8, 0), time.Time{}, false)
	require.Equal(t, "window", SupersededReason(lapsed, now))

	cur := dispItem(careplan.StatusDue, at(2026, 9, 30, 14, 0), time.Time{}, true)
	require.Empty(t, SupersededReason(cur, now))
}

// Terminal statuses are never "superseded" — they stay truthful history.
func TestTerminalStatusesNeverSuperseded(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)
	for _, s := range []careplan.PlanStatus{
		careplan.StatusApplied, careplan.StatusSkipped, careplan.StatusDeferred, careplan.StatusOverridden,
	} {
		it := dispItem(s, at(2026, 9, 30, 8, 0), at(2026, 9, 30, 12, 0), false)
		require.False(t, IsCurrent(it, now), s)
		require.Empty(t, SupersededReason(it, now), s)
	}
}

// §6.2-4 (user's grace rule): a 19:00 feeding missed overnight stays late
// and current until the 07:00 successor becomes imminent (~06:00 with the
// 60 m lookahead) — grace derived from the cadence, not a fixed window.
func TestEveningFeedingStaysLateUntilMorning(t *testing.T) {
	due := at(2026, 9, 29, 19, 0)
	next := at(2026, 9, 30, 7, 0)

	night := dispItem(careplan.StatusLate, due, next, true)
	require.True(t, IsCurrent(night, at(2026, 9, 29, 23, 0)), "still current late in the evening")
	require.True(t, IsCurrent(night, at(2026, 9, 30, 3, 0)), "still current overnight")

	morning := dispItem(careplan.StatusLate, due, next, false) // engine: successor due → window closed
	require.False(t, IsCurrent(morning, at(2026, 9, 30, 6, 30)), "successor imminent")
	require.Equal(t, "replaced", SupersededReason(morning, at(2026, 9, 30, 6, 30)))
}

// §6.4 single daily occurrence: no successor — stays late/current until the
// window lapses, then superseded ("window") in history.
func TestSingleDailyOccurrenceLapse(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)
	due := at(2026, 9, 29, 8, 0)

	inWindow := dispItem(careplan.StatusLate, due, time.Time{}, true)
	require.True(t, IsCurrent(inWindow, now))
	require.Empty(t, SupersededReason(inWindow, now))

	lapsed := dispItem(careplan.StatusMissing, due, time.Time{}, false)
	require.False(t, IsCurrent(lapsed, now))
	require.Equal(t, "window", SupersededReason(lapsed, now))
}

// §6.2-5 date-aware label parts.
func TestDueLabelPartsOfDays(t *testing.T) {
	now := at(2026, 9, 30, 7, 54)

	today := DueLabelPartsOf(at(2026, 9, 30, 8, 0), now)
	require.Empty(t, today.DayKey)
	require.Empty(t, today.ShortDate)
	require.Equal(t, "08:00", today.TimeHM)

	yest := DueLabelPartsOf(at(2026, 9, 29, 8, 0), now)
	require.Equal(t, "care_plan.time.yesterday", yest.DayKey)
	require.Empty(t, yest.ShortDate)

	tomo := DueLabelPartsOf(at(2026, 10, 1, 8, 0), now)
	require.Equal(t, "care_plan.time.tomorrow", tomo.DayKey)

	far := DueLabelPartsOf(at(2026, 9, 27, 8, 0), now)
	require.Empty(t, far.DayKey)
	require.Equal(t, "27/09", far.ShortDate)
}

func TestFormatDueLabel(t *testing.T) {
	require.Equal(t, "08:00", FormatDueLabel(DueLabelParts{TimeHM: "08:00"}, "hier", "demain"))
	require.Equal(t, "hier 08:00", FormatDueLabel(DueLabelParts{DayKey: "care_plan.time.yesterday", TimeHM: "08:00"}, "hier", "demain"))
	require.Equal(t, "demain 08:00", FormatDueLabel(DueLabelParts{DayKey: "care_plan.time.tomorrow", TimeHM: "08:00"}, "hier", "demain"))
	require.Equal(t, "27/09 08:00", FormatDueLabel(DueLabelParts{ShortDate: "27/09", TimeHM: "08:00"}, "hier", "demain"))
	// Postel: empty day words degrade to the bare time.
	require.Equal(t, "08:00", FormatDueLabel(DueLabelParts{DayKey: "care_plan.time.yesterday", TimeHM: "08:00"}, "", ""))
}

// §6.2-3: one chip per (animal × source) — the earliest CURRENT occurrence;
// a superseded first chip is replaced when a current one arrives.
func TestGroupCardsChipDedupeEarliestCurrent(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)
	src := dispSource("r1")
	a := modelsAnimalForPlanTests(1)

	d := &DayPlan{Now: now, Animals: &planAnimals{rows: map[int]models.Animal{1: a}}}

	// 08:00: successor 12:00 already due → NOT current; 12:00: successor
	// 16:00 (3 h > lookahead) → current.
	eight := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: src, AnimalID: 1, DueAt: at(2026, 9, 30, 8, 0)},
		Status: careplan.StatusLate, Applicable: false, NextDue: at(2026, 9, 30, 12, 0)}
	twelve := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: src, AnimalID: 1, DueAt: at(2026, 9, 30, 12, 0)},
		Status: careplan.StatusLate, Applicable: true, NextDue: at(2026, 9, 30, 16, 0)}

	_, feedings := GroupCards([]careplan.PlanItem{eight, twelve}, d)
	require.Len(t, feedings, 1)
	require.Len(t, feedings[0].Chips, 1, "one chip per animal × source")
	require.Equal(t, at(2026, 9, 30, 12, 0), feedings[0].Chips[0].DueAt, "chip = earliest current")
	require.False(t, feedings[0].Chips[0].Superseded)
}

// §6.2-3/§9 fallback: an animal with only superseded occurrences keeps a
// dimmed info chip pointing at the next due time.
func TestGroupCardsChipFallbackSuperseded(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)
	src := dispSource("r1")
	a := modelsAnimalForPlanTests(2)
	d := &DayPlan{Now: now, Animals: &planAnimals{rows: map[int]models.Animal{2: a}}}

	eight := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: src, AnimalID: 2, DueAt: at(2026, 9, 30, 8, 0)},
		Status: careplan.StatusLate, Applicable: false, NextDue: at(2026, 9, 30, 12, 0)}

	_, feedings := GroupCards([]careplan.PlanItem{eight}, d)
	require.Len(t, feedings[0].Chips, 1)
	chip := feedings[0].Chips[0]
	require.True(t, chip.Superseded)
	require.Equal(t, "12:00", chip.SupersededBy)
	require.Empty(t, chip.SupersededByDayKey, "same-day replacement = time only")
}

// §6.2-3: two sources of the same animal keep one chip each.
func TestGroupCardsChipPerSource(t *testing.T) {
	now := at(2026, 9, 30, 13, 0)
	r1, r2 := dispSource("r1"), dispSource("r2")
	a := modelsAnimalForPlanTests(3)
	d := &DayPlan{Now: now, Animals: &planAnimals{rows: map[int]models.Animal{3: a}}}

	i1 := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: r1, AnimalID: 3, DueAt: at(2026, 9, 30, 9, 0)},
		Status: careplan.StatusDue, Applicable: true, NextDue: time.Time{}}
	i2 := careplan.PlanItem{Occurrence: careplan.Occurrence{Source: r2, AnimalID: 3, DueAt: at(2026, 9, 30, 10, 0)},
		Status: careplan.StatusDue, Applicable: true, NextDue: time.Time{}}

	_, feedings := GroupCards([]careplan.PlanItem{i1, i2}, d)
	require.Len(t, feedings, 1, "same food groups onto one card")
	require.Len(t, feedings[0].Chips, 2, "one chip per (animal × source)")
}
