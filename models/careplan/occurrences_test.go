package careplan

import (
	"fmt"
	"testing"
	"time"

	// Embedded tz database: the DST boundary test (§4.3/§10-B1) must be
	// hermetic and not depend on the host having Europe/Brussels installed.
	_ "time/tzdata"
)

// occDueStrings renders occurrences as "2006-01-02 15:04" in their location
// — the shape the day plan and tests reason about.
func occDueStrings(occs []Occurrence) []string {
	out := make([]string, len(occs))
	for i, o := range occs {
		out[i] = o.DueAt.Format("2006-01-02 15:04")
	}
	return out
}

func assertOccs(t *testing.T, got []Occurrence, want ...string) {
	t.Helper()
	gotS := occDueStrings(got)
	if fmt.Sprint(gotS) != fmt.Sprint(want) {
		t.Fatalf("occurrences = %v, want %v", gotS, want)
	}
	for i := 1; i < len(got); i++ {
		if got[i].DueAt.Before(got[i-1].DueAt) {
			t.Fatalf("occurrences not sorted: %v after %v", got[i].DueAt, got[i-1].DueAt)
		}
		if got[i].AnimalID != got[0].AnimalID {
			t.Fatalf("mixed animals in one generation: %d and %d", got[i].AnimalID, got[0].AnimalID)
		}
	}
}

func occAnimal(intake time.Time, outtake *time.Time) *AnimalContext {
	return &AnimalContext{ID: 42, IntakeDate: intake, OuttakeDate: outtake}
}

func day(t *testing.T, s string) time.Time {
	t.Helper()
	d, err := time.ParseInLocation("2006-01-02 15:04", s, time.Local)
	if err != nil {
		t.Fatalf("bad test instant %q: %v", s, err)
	}
	return d
}

func ruleSource(t *testing.T, schedJSON string) PlanSource {
	return NewSource(SourceRule, "r1", "Règle 1", KindFeeding, mustSchedule(t, schedJSON))
}

// §10-B3: day 1 is the first slot strictly after the intake instant — not
// the intake calendar day.
func TestGenerateIntakeDay1Anchoring(t *testing.T) {
	src := ruleSource(t, `{"times":["07:00","19:00"]}`)
	ws, we := day(t, "2026-09-01 00:00"), day(t, "2026-09-03 23:59")

	// Intake 18:00 → day 1 starts with the 19:00 slot the same evening.
	a := occAnimal(day(t, "2026-09-01 18:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, ws, we),
		"2026-09-01 19:00",
		"2026-09-02 07:00", "2026-09-02 19:00",
		"2026-09-03 07:00", "2026-09-03 19:00")

	// Intake 20:00 (after all slots) → day 1 is the NEXT morning; no
	// retroactive 19:00 item on intake day.
	a = occAnimal(day(t, "2026-09-01 20:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, ws, we),
		"2026-09-02 07:00", "2026-09-02 19:00",
		"2026-09-03 07:00", "2026-09-03 19:00")

	// Intake 06:00 (before all slots) → the whole intake day counts.
	a = occAnimal(day(t, "2026-09-01 06:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, ws, we),
		"2026-09-01 07:00", "2026-09-01 19:00",
		"2026-09-02 07:00", "2026-09-02 19:00",
		"2026-09-03 07:00", "2026-09-03 19:00")

	// Intake exactly on a slot instant: "strictly after" excludes it.
	a = occAnimal(day(t, "2026-09-01 19:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, ws, we),
		"2026-09-02 07:00", "2026-09-02 19:00",
		"2026-09-03 07:00", "2026-09-03 19:00")
}

// from_offset_days shifts day 1 from the anchor (§4.3).
func TestGenerateFromOffsetDays(t *testing.T) {
	src := ruleSource(t, `{"times":["07:00"],"from_offset_days":2}`)
	ws, we := day(t, "2026-09-01 00:00"), day(t, "2026-09-06 23:59")
	a := occAnimal(day(t, "2026-09-01 18:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, ws, we),
		"2026-09-03 07:00", "2026-09-04 07:00", "2026-09-05 07:00", "2026-09-06 07:00")
}

// every_days steps occurrence days from the anchor (§4.3).
func TestGenerateEveryDays(t *testing.T) {
	// Intake anchor: day 1 = 09-01 (slot 08:00 is after the 07:30 intake),
	// then every 2 days.
	src := ruleSource(t, `{"times":["08:00"],"every_days":2}`)
	a := occAnimal(day(t, "2026-09-01 07:30"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-09-09 23:59")),
		"2026-09-01 08:00", "2026-09-03 08:00", "2026-09-05 08:00", "2026-09-07 08:00", "2026-09-09 08:00")

	// Fixed anchor steps from anchor_date.
	src = ruleSource(t, `{"times":["08:00"],"every_days":2,"anchor":"fixed","anchor_date":"2026-09-02"}`)
	a = occAnimal(day(t, "2026-08-30 07:30"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-09-09 23:59")),
		"2026-09-02 08:00", "2026-09-04 08:00", "2026-09-06 08:00", "2026-09-08 08:00")
}

// weekdays restrict occurrence days; a filtered day is not "generated"
// (duration_days counts generated days, §10-B3).
func TestGenerateWeekdaysAndDuration(t *testing.T) {
	// 2026-09-07 is a Monday. Mondays only, 3-day course → exactly 3
	// Mondays, the 8 filtered in-between days do not consume the course.
	src := ruleSource(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-09-07","weekdays":[1],"duration_days":3}`)
	a := occAnimal(day(t, "2026-08-30 07:30"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-10-31 23:59")),
		"2026-09-07 08:00", "2026-09-14 08:00", "2026-09-21 08:00")

	// Same schedule open-ended with a 2-week window → 2 Mondays.
	src = ruleSource(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-09-07","weekdays":[1]}`)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-08 00:00"), day(t, "2026-09-21 23:59")),
		"2026-09-14 08:00", "2026-09-21 08:00")

	// every_days=7 + weekdays=[1] = "every Monday" — same result shape.
	src = ruleSource(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-09-07","every_days":7,"weekdays":[1],"duration_days":3}`)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-10-31 23:59")),
		"2026-09-07 08:00", "2026-09-14 08:00", "2026-09-21 08:00")
}

// duration_days without filters yields exactly N consecutive days.
func TestGenerateDurationDaysExact(t *testing.T) {
	src := ruleSource(t, `{"times":["08:00","19:00"],"anchor":"fixed","anchor_date":"2026-09-01","duration_days":2}`)
	a := occAnimal(day(t, "2026-08-30 07:30"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-09-30 23:59")),
		"2026-09-01 08:00", "2026-09-01 19:00", "2026-09-02 08:00", "2026-09-02 19:00")
}

// Care-period clamp: instants land in [intakeDate, outtakeDate) (§4.3).
func TestGenerateOuttakeClamp(t *testing.T) {
	out := day(t, "2026-09-03 10:00")
	src := ruleSource(t, `{"times":["08:00","19:00"]}`)

	// Intake anchor: last occurrence is the last slot before the outtake.
	a := occAnimal(day(t, "2026-09-01 06:00"), &out)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-09-30 23:59")),
		"2026-09-01 08:00", "2026-09-01 19:00",
		"2026-09-02 08:00", "2026-09-02 19:00",
		"2026-09-03 08:00")

	// Outtake at/before intake → nothing (empty care period).
	zero := day(t, "2026-09-01 06:00")
	a = occAnimal(day(t, "2026-09-01 06:00"), &zero)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-01 00:00"), day(t, "2026-09-30 23:59")))

	// Fixed anchor: pre-intake instants are dropped by the same clamp.
	src = ruleSource(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-08-30"}`)
	a = occAnimal(day(t, "2026-09-01 12:00"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-08-30 00:00"), day(t, "2026-09-03 23:59")),
		// 08-30/08-31 08:00 and 09-01 08:00 are all before intake 09-01 12:00.
		"2026-09-02 08:00", "2026-09-03 08:00")
}

// Rule validity window (§4.1) filters occurrence days, day granularity.
func TestGenerateValidFromTo(t *testing.T) {
	vf := day(t, "2026-09-03 00:00")
	vt := day(t, "2026-09-05 23:59")
	var src PlanSource = NewSource(SourceRule, "r1", "Règle 1", KindFeeding, mustSchedule(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-09-01"}`))
	s := src.(*Source)
	s.ValidFromD, s.ValidToD = &vf, &vt
	a := occAnimal(day(t, "2026-08-30 07:30"), nil)
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-08-30 00:00"), day(t, "2026-09-30 23:59")),
		"2026-09-03 08:00", "2026-09-04 08:00", "2026-09-05 08:00")

	// valid_to alone ends the course.
	src = NewSource(SourceRule, "r2", "Règle 2", KindFeeding, mustSchedule(t, `{"times":["08:00"],"anchor":"fixed","anchor_date":"2026-09-01"}`))
	src.(*Source).ValidToD = &vt
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-08-30 00:00"), day(t, "2026-09-30 23:59")),
		"2026-09-01 08:00", "2026-09-02 08:00", "2026-09-03 08:00", "2026-09-04 08:00", "2026-09-05 08:00")
}

// Window: only instants within [windowStart, windowEnd] are returned.
func TestGenerateWindowFilter(t *testing.T) {
	src := ruleSource(t, `{"times":["07:00","09:30","19:00"]}`)
	a := occAnimal(day(t, "2026-08-30 06:00"), nil)
	// Mid-day window start: 07:00 and 09:30 of 09-02 are before ws.
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-09-02 12:00"), day(t, "2026-09-03 23:59")),
		"2026-09-02 19:00", "2026-09-03 07:00", "2026-09-03 09:30", "2026-09-03 19:00")
	// Empty window → nothing.
	assertOccs(t, GenerateOccurrences(src, a, day(t, "2026-08-01 00:00"), day(t, "2026-08-02 00:00")))
}

// Degenerate inputs generate nothing (never panic).
func TestGenerateDegenerateInputs(t *testing.T) {
	ws, we := day(t, "2026-09-01 00:00"), day(t, "2026-09-03 23:59")
	// Inactive source.
	src := ruleSource(t, `{"times":["07:00"]}`)
	src.(*Source).IsActive = false
	assertOccs(t, GenerateOccurrences(src, occAnimal(day(t, "2026-09-01 06:00"), nil), ws, we))

	// intake anchor without an intake instant → nothing to anchor on.
	src = ruleSource(t, `{"times":["07:00"]}`)
	assertOccs(t, GenerateOccurrences(src, &AnimalContext{ID: 42}, ws, we))

	// Animal plan of a different source type behaves identically.
	src = NewSource(SourceAnimal, "ap1", "Plan animal", KindFeeding, mustSchedule(t, `{"times":["07:00"]}`))
	assertOccs(t, GenerateOccurrences(src, occAnimal(day(t, "2026-09-01 06:00"), nil), ws, we),
		"2026-09-01 07:00", "2026-09-02 07:00", "2026-09-03 07:00")
}

// DST boundary (§4.3/§10-B1): day arithmetic keeps wall-clock slots on
// their HH:MM across the Europe/Brussels spring-forward (2026-03-29,
// 02:00→03:00) and fall-back (2026-10-25, 03:00→02:00) transitions.
func TestGenerateDSTBoundary(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Brussels")
	if err != nil {
		t.Fatalf("tzdata missing: %v", err)
	}
	oldLocal := time.Local
	time.Local = loc
	defer func() { time.Local = oldLocal }()

	mk := func(anchor string) PlanSource {
		return ruleSource(t, `{"times":["07:00"],"anchor":"fixed","anchor_date":"`+anchor+`"}`)
	}
	a := occAnimal(time.Date(2026, 1, 1, 6, 0, 0, 0, loc), nil)
	in := func(s string) time.Time {
		d, err := time.ParseInLocation("2006-01-02 15:04", s, loc)
		if err != nil {
			t.Fatalf("bad instant %q: %v", s, err)
		}
		return d
	}

	// Spring forward: 03-28 is CET (+01), 03-29 is CEST (+02), both 07:00.
	occs := GenerateOccurrences(mk("2026-03-28"), a, in("2026-03-27 00:00"), in("2026-03-30 23:59"))
	assertOccs(t, occs, "2026-03-28 07:00", "2026-03-29 07:00", "2026-03-30 07:00")
	if off := occs[0].DueAt.Format("-07:00"); off != "+01:00" {
		t.Errorf("2026-03-28 07:00 offset = %s, want +01:00 (CET)", off)
	}
	if off := occs[1].DueAt.Format("-07:00"); off != "+02:00" {
		t.Errorf("2026-03-29 07:00 offset = %s, want +02:00 (CEST) — slot must keep wall clock", off)
	}

	// Fall back: 10-24 is CEST (+02), 10-25 is CET (+01), both 07:00.
	occs = GenerateOccurrences(mk("2026-10-24"), a, in("2026-10-23 00:00"), in("2026-10-26 23:59"))
	assertOccs(t, occs, "2026-10-24 07:00", "2026-10-25 07:00", "2026-10-26 07:00")
	if off := occs[0].DueAt.Format("-07:00"); off != "+02:00" {
		t.Errorf("2026-10-24 07:00 offset = %s, want +02:00 (CEST)", off)
	}
	if off := occs[1].DueAt.Format("-07:00"); off != "+01:00" {
		t.Errorf("2026-10-25 07:00 offset = %s, want +01:00 (CET) — no duplicated hour", off)
	}

	// Day-1 intake anchoring across the spring-forward night: intake
	// 2026-03-28 20:00 → first occurrence 03-29 07:00 CEST, not 08:00.
	src := ruleSource(t, `{"times":["07:00","22:00"]}`)
	a = occAnimal(time.Date(2026, 3, 28, 20, 0, 0, 0, loc), nil)
	occs = GenerateOccurrences(src, a, in("2026-03-28 00:00"), in("2026-03-30 23:59"))
	assertOccs(t, occs, "2026-03-28 22:00", "2026-03-29 07:00", "2026-03-29 22:00", "2026-03-30 07:00", "2026-03-30 22:00")
	if off := occs[1].DueAt.Format("-07:00"); off != "+02:00" {
		t.Errorf("day-1 slot after DST = %s offset, want +02:00", off)
	}
}
