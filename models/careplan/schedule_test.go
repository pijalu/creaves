package careplan

import (
	"strings"
	"testing"
	"time"
)

func mustSchedule(t *testing.T, raw string) Schedule {
	t.Helper()
	s, err := ParseScheduleJSON([]byte(raw))
	if err != nil {
		t.Fatalf("ParseScheduleJSON(%s): %v", raw, err)
	}
	return s
}

func TestScheduleDefaults(t *testing.T) {
	// §4.3 defaults: every_days=1, anchor=intake, grace=60, miss=24h,
	// lookahead=60 (§10-CP6a).
	s := mustSchedule(t, `{"times":["07:00"]}`)
	if len(s.Times) != 1 || s.Times[0].String() != "07:00" {
		t.Errorf("times = %v", s.Times)
	}
	if s.EveryDays != 1 || s.Anchor != AnchorIntake || s.AnchorDate != nil {
		t.Errorf("defaults = every %d, anchor %q, date %v", s.EveryDays, s.Anchor, s.AnchorDate)
	}
	if s.GraceMinutes != 60 || s.MissAfterHours != 24 || s.LookaheadMinutes != 60 {
		t.Errorf("window defaults = grace %d, miss %d, lookahead %d", s.GraceMinutes, s.MissAfterHours, s.LookaheadMinutes)
	}
	if !s.OpenEnded() {
		t.Errorf("absent duration_days must be open-ended")
	}
	if len(s.Weekdays) != 0 {
		t.Errorf("absent weekdays = all days, got %v", s.Weekdays)
	}
}

func TestScheduleFullDocument(t *testing.T) {
	// The §4.3 example shape.
	s := mustSchedule(t, `{
		"times": ["07:00", "09:30", "12:30", "15:30", "19:00"],
		"every_days": 2,
		"weekdays": [1,2,3,4,5],
		"anchor": "fixed",
		"anchor_date": "2026-09-01",
		"from_offset_days": 1,
		"duration_days": 5,
		"grace_minutes": 30,
		"miss_after_hours": 12,
		"lookahead_minutes": 15
	}`)
	if len(s.Times) != 5 || s.Times[4].String() != "19:00" {
		t.Errorf("times = %v", s.Times)
	}
	if s.EveryDays != 2 || s.FromOffsetDays != 1 || s.DurationDays != 5 || s.OpenEnded() {
		t.Errorf("cadence = every %d, offset %d, duration %d", s.EveryDays, s.FromOffsetDays, s.DurationDays)
	}
	if s.Anchor != AnchorFixed || s.AnchorDate == nil {
		t.Fatalf("anchor = %q, date %v", s.Anchor, s.AnchorDate)
	}
	if s.AnchorDate.Local().Format("2006-01-02") != "2026-09-01" {
		t.Errorf("anchor_date = %v", s.AnchorDate)
	}
	if s.GraceMinutes != 30 || s.MissAfterHours != 12 || s.LookaheadMinutes != 15 {
		t.Errorf("windows = %d/%d/%d", s.GraceMinutes, s.MissAfterHours, s.LookaheadMinutes)
	}
}

func TestScheduleTimeParsing(t *testing.T) {
	for _, bad := range []string{"7:00", "07:60", "24:00", "0700", "07.00", ""} {
		if _, err := ParseScheduleJSON([]byte(`{"times":["` + bad + `"]}`)); err == nil {
			t.Errorf("ParseScheduleJSON(times=%q): expected error", bad)
		} else if !strings.Contains(err.Error(), "times") {
			t.Errorf("error for %q should mention schedule.times: %v", bad, err)
		}
	}
	if _, err := ParseScheduleJSON([]byte(`{"times":[]}`)); err == nil {
		t.Errorf("empty times must be rejected (§4.3: exhaustive list, ≥1)")
	}
	if _, err := ParseScheduleJSON([]byte(`{}`)); err == nil {
		t.Errorf("absent times must be rejected")
	}
	// boundaries
	for _, good := range []string{"00:00", "23:59", "12:05"} {
		if _, err := ParseScheduleJSON([]byte(`{"times":["` + good + `"]}`)); err != nil {
			t.Errorf("ParseScheduleJSON(times=%q): %v", good, err)
		}
	}
}

func TestScheduleTimesMustBeStrictlyAscending(t *testing.T) {
	for _, times := range []string{`["09:30","07:00"]`, `["07:00","07:00"]`} {
		if _, err := ParseScheduleJSON([]byte(`{"times":` + times + `}`)); err == nil {
			t.Errorf("times %s must be rejected (sorted, unique, §4.3)", times)
		}
	}
}

func TestScheduleWeekdays(t *testing.T) {
	s := mustSchedule(t, `{"times":["08:00"],"weekdays":[1,3,5]}`)
	if !s.AllowsWeekday(time.Monday) || s.AllowsWeekday(time.Tuesday) || s.AllowsWeekday(time.Sunday) {
		t.Errorf("AllowsWeekday wrong for %v", s.Weekdays)
	}
	s = mustSchedule(t, `{"times":["08:00"]}`)
	if !s.AllowsWeekday(time.Sunday) || !s.AllowsWeekday(time.Saturday) {
		t.Errorf("nil weekdays must allow every day")
	}
	for _, bad := range []string{`[7]`, `[-1]`, `[1,1]`} {
		if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"],"weekdays":` + bad + `}`)); err == nil {
			t.Errorf("weekdays %s must be rejected", bad)
		}
	}
}

func TestScheduleAnchorRules(t *testing.T) {
	// fixed requires anchor_date
	if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"],"anchor":"fixed"}`)); err == nil {
		t.Errorf("anchor=fixed without anchor_date must be rejected")
	}
	// intake forbids anchor_date
	if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"],"anchor":"intake","anchor_date":"2026-09-01"}`)); err == nil {
		t.Errorf("anchor=intake with anchor_date must be rejected")
	}
	// unknown anchor
	if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"],"anchor":"admission"}`)); err == nil {
		t.Errorf("unknown anchor must be rejected")
	}
}

func TestScheduleStrictness(t *testing.T) {
	// Unknown keys are typos → rejected at save time.
	if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"],"evry_days":2}`)); err == nil {
		t.Errorf("unknown key must be rejected")
	}
	if _, err := ParseScheduleJSON([]byte(`{"times":["08:00"]}{"times":["09:00"]}`)); err == nil {
		t.Errorf("trailing data must be rejected")
	}
	// Bounds.
	for _, raw := range []string{
		`{"times":["08:00"],"every_days":0}`,
		`{"times":["08:00"],"from_offset_days":-1}`,
		`{"times":["08:00"],"duration_days":0}`, // absent = open; explicit 0 is a mistake
		`{"times":["08:00"],"duration_days":-5}`,
		`{"times":["08:00"],"grace_minutes":-1}`,
		`{"times":["08:00"],"miss_after_hours":0}`,
		`{"times":["08:00"],"lookahead_minutes":-1}`,
		`{"times":["08:00"],"anchor_date":"01-09-2026"}`,
	} {
		if _, err := ParseScheduleJSON([]byte(raw)); err == nil {
			t.Errorf("%s must be rejected", raw)
		}
	}
}

func TestScheduleSlotsAfterDayOneAnchoring(t *testing.T) {
	// §10-B3 helper: the first slot strictly after the intake instant.
	s := mustSchedule(t, `{"times":["07:00","09:30","12:30","15:30","19:00"]}`)
	if got := s.SlotsAfter(8 * 60); s.Times[got].String() != "09:30" {
		t.Errorf("SlotsAfter(08:00) = %d (%s), want 09:30", got, s.Times[got])
	}
	if got := s.SlotsAfter(19 * 60); got != len(s.Times) {
		t.Errorf("SlotsAfter(19:00) = %d, want len (no slot after the last)", got)
	}
	if got := s.SlotsAfter(-1); got != 0 {
		t.Errorf("SlotsAfter(before midnight) = %d, want 0", got)
	}
}

func TestScheduleValidateStandalone(t *testing.T) {
	// Built-in-code schedules validate without a JSON round-trip.
	s := Schedule{}
	if err := s.Validate(); err == nil {
		t.Errorf("empty schedule must not validate")
	}
	s = Schedule{
		Times:            []TimeOfDay{{8, 0}, {17, 0}},
		EveryDays:        1,
		Anchor:           AnchorIntake,
		GraceMinutes:     60,
		MissAfterHours:   24,
		LookaheadMinutes: 60,
	}
	if err := s.Validate(); err != nil {
		t.Errorf("valid built schedule rejected: %v", err)
	}
}
