package careplan

import (
	"time"
)

// Occurrence generation (§6.1 step 1): one PlanSource × one animal → the
// virtual `due_at` instants of its schedule inside a window. Pure function —
// no DB, no clock: `now`-dependent decisions (status, apply window) live in
// status.go, membership (which animals a rule covers) is resolved by the
// matcher before calling this, and course latching (§10-A4) is layered on
// top by the service for bounded courses after first application.
//
// All instants are computed in server local time (§10-B1), matching the
// existing care/treatment rows of the single-center deployment.

// Occurrence is one virtual scheduled instant (§6.1): the (source, animal,
// due_at) triple that — joined with applications — becomes a plan item.
// The applications UNIQUE key (§4.5) is exactly this triple.
type Occurrence struct {
	Source   PlanSource
	AnimalID int
	DueAt    time.Time
}

// maxOccurrenceDaySteps is a defensive cap on candidate-day iterations. A
// validated schedule (every_days ≥ 1) with sane windows never approaches
// it; it only guards against a pathological window passed by a caller.
const maxOccurrenceDaySteps = 100000

// GenerateOccurrences returns the source's occurrences for one animal whose
// due_at lies within [windowStart, windowEnd] (inclusive bounds), sorted
// ascending. Inactive sources, missing anchors and degenerate care periods
// generate nothing — never an error (the plan view degrades to fewer items).
//
// Semantics (§4.3, §6.1):
//   - anchor=intake: day 1 is the first times[] slot strictly after the
//     intake instant (§10-B3) — the intake calendar day if a slot remains,
//     else the next morning; anchor=fixed: day 1 is anchor_date. Both are
//     shifted by from_offset_days first.
//   - candidate days step every_days; weekdays filters days; duration_days
//     counts GENERATED days (weekday-passing), not calendar days.
//   - rule validity [valid_from, valid_to] filters days (day granularity).
//   - instants are clamped to [intake, outtake) and the window.
func GenerateOccurrences(src PlanSource, a *AnimalContext, windowStart, windowEnd time.Time) []Occurrence {
	if src == nil || a == nil || !src.Active() {
		return nil
	}
	s := src.Schedule()
	if len(s.Times) == 0 || s.EveryDays < 1 || windowEnd.Before(windowStart) {
		return nil
	}
	loc := time.Local

	intake := a.IntakeDate
	day1, firstSlotStart, ok := resolveDay1(s, a, loc)
	if !ok {
		return nil
	}

	validFrom, validTo := src.ValidFrom(), src.ValidTo()
	var out []Occurrence
	generated := 0
	firstGeneratedDone := false
	day := day1
	for i := 0; !day.After(windowEnd) && i < maxOccurrenceDaySteps; i++ {
		if validFrom != nil && day.Before(midnightIn(loc, *validFrom)) {
			day = day.AddDate(0, 0, s.EveryDays)
			continue
		}
		if validTo != nil && day.After(*validTo) {
			break // validity window over — every later day is too
		}
		if s.DurationDays > 0 && generated >= s.DurationDays {
			break // course complete (§10-B3: generated days counted)
		}
		if s.AllowsWeekday(day.Weekday()) {
			generated++
			times := s.Times
			if !firstGeneratedDone { // day-1 slot anchoring, once
				times = s.Times[firstSlotStart:]
				firstGeneratedDone = true
			}
			for _, slot := range times {
				due := time.Date(day.Year(), day.Month(), day.Day(),
					slot.Hour, slot.Minute, 0, 0, loc)
				if due.Before(windowStart) || due.After(windowEnd) {
					continue
				}
				if !intake.IsZero() && due.Before(intake) {
					continue // pre-intake slot (§4.3 clamp, fixed anchor)
				}
				if a.OuttakeDate != nil && !due.Before(*a.OuttakeDate) {
					continue // [intake, outtake): post-outtake slot dropped
				}
				out = append(out, Occurrence{Source: src, AnimalID: a.ID, DueAt: due})
			}
		}
		day = day.AddDate(0, 0, s.EveryDays)
	}
	return out
}

// resolveDay1 computes the first candidate day and its day-1 slot start
// (§10-B3). ok=false only when the schedule cannot be anchored (intake
// anchor without an intake instant).
func resolveDay1(s Schedule, a *AnimalContext, loc *time.Location) (day1 time.Time, firstSlotStart int, ok bool) {
	if s.Anchor == AnchorIntake {
		if a.IntakeDate.IsZero() {
			return time.Time{}, 0, false
		}
		day1 = midnightIn(loc, a.IntakeDate).AddDate(0, 0, s.FromOffsetDays)
		// Day-1 anchoring applies while day 1 is still the intake calendar
		// day: drop slots at/before the intake instant, rolling to the next
		// morning when the intake instant is past the last slot.
		if sameLocalDay(loc, day1, a.IntakeDate) {
			firstSlotStart = s.SlotsAfter(minutesIn(loc, a.IntakeDate))
			if firstSlotStart == len(s.Times) {
				day1 = day1.AddDate(0, 0, 1)
				firstSlotStart = 0
			}
		}
		return day1, firstSlotStart, true
	}
	return midnightIn(loc, *s.AnchorDate).AddDate(0, 0, s.FromOffsetDays), 0, true
}

// CourseBounds returns the first and last occurrence instants of the
// source's course for the animal (§10-A4 engine surface): the service uses
// `last` to keep a latched bounded course attached after its matcher stops
// matching; open-ended courses report bounded=false and are never latched.
// Clamps apply: a course truncated by the outtake ends at its last
// pre-outtake instant. ok=false when nothing can be generated.
func CourseBounds(src PlanSource, a *AnimalContext) (first, last time.Time, ok, bounded bool) {
	if src == nil || a == nil || !src.Active() {
		return time.Time{}, time.Time{}, false, false
	}
	s := src.Schedule()
	loc := time.Local
	day1, _, anchored := resolveDay1(s, a, loc)
	if !anchored {
		return time.Time{}, time.Time{}, false, false
	}
	var occs []Occurrence
	if s.OpenEnded() {
		// Sample window for `first` only — an open-ended course has no end.
		occs = GenerateOccurrences(src, a, day1, day1.AddDate(0, 0, 14))
		if len(occs) == 0 {
			return time.Time{}, time.Time{}, false, false
		}
		return occs[0].DueAt, time.Time{}, true, false
	}
	occs = GenerateOccurrences(src, a, day1, day1.AddDate(0, 0, s.DurationDays*s.EveryDays+2))
	if len(occs) == 0 {
		return time.Time{}, time.Time{}, false, true
	}
	return occs[0].DueAt, occs[len(occs)-1].DueAt, true, true
}

// midnightIn returns local midnight of t's calendar day in loc.
func midnightIn(loc *time.Location, t time.Time) time.Time {
	t = t.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

// minutesIn returns t's minutes since midnight in loc (wall clock).
func minutesIn(loc *time.Location, t time.Time) int {
	t = t.In(loc)
	return t.Hour()*60 + t.Minute()
}

func sameLocalDay(loc *time.Location, a, b time.Time) bool {
	a, b = a.In(loc), b.In(loc)
	return a.Year() == b.Year() && a.Month() == b.Month() && a.Day() == b.Day()
}
