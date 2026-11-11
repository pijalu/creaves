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

// ---------------------------------------------------------------------------
// BUG-36 — repeat block ("from/to every X")
// ---------------------------------------------------------------------------

func TestScheduleRepeatParseRoundTrip(t *testing.T) {
	raw := `{"times":["09:00","14:00","19:00"],"repeat":{"from":"09:00","to":"19:00","every_minutes":300}}`
	s := mustSchedule(t, raw)
	if s.Repeat == nil {
		t.Fatalf("repeat block not parsed")
	}
	if s.Repeat.From.String() != "09:00" || s.Repeat.To.String() != "19:00" || s.Repeat.EveryMinutes != 300 {
		t.Errorf("repeat = %v→%v every %d", s.Repeat.From, s.Repeat.To, s.Repeat.EveryMinutes)
	}
	// Round-trip: marshal → parse yields the same value object.
	out, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	s2 := mustSchedule(t, string(out))
	if s2.Repeat == nil || *s2.Repeat != *s.Repeat {
		t.Errorf("round-trip repeat = %v, want %v (json %s)", s2.Repeat, s.Repeat, out)
	}
	if !timesEqual(s2.Times, s.Times) {
		t.Errorf("round-trip times = %v, want %v", s2.Times, s.Times)
	}
}

func TestExpandRepeat(t *testing.T) {
	cases := []struct {
		name         string
		from, to     string
		everyMinutes int
		want         []string
	}{
		{"normal incl. start", "08:00", "18:00", 120, []string{"08:00", "10:00", "12:00", "14:00", "16:00", "18:00"}},
		{"spec example", "09:00", "19:00", 300, []string{"09:00", "14:00", "19:00"}},
		{"start==end single slot", "10:00", "10:00", 600, []string{"10:00"}},
		{"no slot fits beyond start", "08:00", "09:00", 120, []string{"08:00"}},
		{"overnight wraps and sorts", "22:00", "06:00", 240, []string{"02:00", "06:00", "22:00"}},
		{"overnight single wrap", "23:00", "01:00", 120, []string{"01:00", "23:00"}},
		{"zero period", "08:00", "18:00", 0, nil},
		{"negative period", "08:00", "18:00", -5, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			from, _ := ParseTimeOfDay(tc.from)
			to, _ := ParseTimeOfDay(tc.to)
			got := ExpandRepeat(Repeat{From: from, To: to, EveryMinutes: tc.everyMinutes})
			if len(got) != len(tc.want) {
				t.Fatalf("ExpandRepeat = %v, want %v", got, tc.want)
			}
			for i, w := range tc.want {
				if got[i].String() != w {
					t.Fatalf("ExpandRepeat = %v, want %v", got, tc.want)
				}
			}
		})
	}
}

func TestExpandRepeatCap24(t *testing.T) {
	// 00:00–23:59 every minute would be 1440 slots: capped at 24.
	from, _ := ParseTimeOfDay("00:00")
	to, _ := ParseTimeOfDay("23:59")
	got := ExpandRepeat(Repeat{From: from, To: to, EveryMinutes: 1})
	if len(got) != 24 {
		t.Errorf("cap: got %d slots, want 24", len(got))
	}
	// A repeat that would hit the cap is rejected at validation: the
	// expansion must never silently truncate a stored document.
	if _, err := ParseScheduleJSON([]byte(`{"repeat":{"from":"00:00","to":"23:59","every_minutes":1}}`)); err == nil {
		t.Errorf("repeat expanding past 24 slots must be rejected")
	}
	// Exactly 24 slots is accepted (00:00–23:00 hourly).
	if _, err := ParseScheduleJSON([]byte(`{"repeat":{"from":"00:00","to":"23:00","every_minutes":60}}`)); err != nil {
		t.Errorf("24-slot repeat must be accepted: %v", err)
	}
}

func TestScheduleRepeatValidation(t *testing.T) {
	for _, raw := range []string{
		// Bad times / period.
		`{"repeat":{"from":"9:00","to":"19:00","every_minutes":300}}`,
		`{"repeat":{"from":"09:00","to":"25:00","every_minutes":300}}`,
		`{"repeat":{"from":"09:00","to":"19:00","every_minutes":0}}`,
		`{"repeat":{"from":"09:00","to":"19:00","every_minutes":-30}}`,
		// times present but inconsistent with the repeat expansion.
		`{"times":["09:00","14:00"],"repeat":{"from":"09:00","to":"19:00","every_minutes":300}}`,
		`{"times":["09:00","15:00","19:00"],"repeat":{"from":"09:00","to":"19:00","every_minutes":300}}`,
		// Overnight expansion sorted ≠ raw wrap order (times must be the
		// canonical ascending expansion).
		`{"times":["22:00","02:00","06:00"],"repeat":{"from":"22:00","to":"06:00","every_minutes":240}}`,
	} {
		if _, err := ParseScheduleJSON([]byte(raw)); err == nil {
			t.Errorf("%s must be rejected", raw)
		}
	}
}

func TestScheduleRepeatTimesDerivedWhenAbsent(t *testing.T) {
	// Repeat without times: the parser derives the slots (normalize path).
	s := mustSchedule(t, `{"repeat":{"from":"09:00","to":"19:00","every_minutes":300}}`)
	if len(s.Times) != 3 || s.Times[0].String() != "09:00" || s.Times[2].String() != "19:00" {
		t.Errorf("derived times = %v", s.Times)
	}
	// Overnight derivation.
	s = mustSchedule(t, `{"repeat":{"from":"22:00","to":"06:00","every_minutes":240}}`)
	if len(s.Times) != 3 || s.Times[0].String() != "02:00" || s.Times[2].String() != "22:00" {
		t.Errorf("overnight derived times = %v", s.Times)
	}
}

func TestScheduleAbsentRepeatUnchanged(t *testing.T) {
	// Backward compatibility: no repeat block → Repeat nil, behavior as before.
	s := mustSchedule(t, `{"times":["07:00","09:30"]}`)
	if s.Repeat != nil {
		t.Errorf("absent repeat must stay nil, got %v", s.Repeat)
	}
	out, err := s.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON: %v", err)
	}
	if strings.Contains(string(out), "repeat") {
		t.Errorf("marshal must not emit a repeat key when absent: %s", out)
	}
	s2 := mustSchedule(t, string(out))
	if s2.Repeat != nil || !timesEqual(s2.Times, s.Times) {
		t.Errorf("slots-mode round-trip drift: %s", out)
	}
}

func TestNormalizeScheduleJSON(t *testing.T) {
	// Repeat present: client times are ignored, re-derived from the block.
	out, err := NormalizeScheduleJSON([]byte(
		`{"times":["03:00"],"repeat":{"from":"09:00","to":"19:00","every_minutes":300},"every_days":2}`))
	if err != nil {
		t.Fatalf("NormalizeScheduleJSON: %v", err)
	}
	s := mustSchedule(t, string(out))
	if len(s.Times) != 3 || s.Times[1].String() != "14:00" {
		t.Errorf("normalized times = %v, want expansion [09:00 14:00 19:00]", s.Times)
	}
	if s.Repeat == nil || s.EveryDays != 2 {
		t.Errorf("normalized doc lost repeat/every_days: %s", out)
	}
	// Tampered overnight times are overwritten too.
	out, err = NormalizeScheduleJSON([]byte(
		`{"times":["22:00"],"repeat":{"from":"22:00","to":"06:00","every_minutes":240}}`))
	if err != nil {
		t.Fatalf("NormalizeScheduleJSON overnight: %v", err)
	}
	s = mustSchedule(t, string(out))
	if len(s.Times) != 3 || s.Times[0].String() != "02:00" {
		t.Errorf("overnight normalized times = %v", s.Times)
	}
	// No repeat: document re-marshaled unchanged in content (times kept).
	out, err = NormalizeScheduleJSON([]byte(`{"times":["08:00","20:00"]}`))
	if err != nil {
		t.Fatalf("NormalizeScheduleJSON slots: %v", err)
	}
	s = mustSchedule(t, string(out))
	if s.Repeat != nil || len(s.Times) != 2 {
		t.Errorf("slots doc drifted: %s", out)
	}
	// Invalid input propagates the parse error.
	if _, err := NormalizeScheduleJSON([]byte(`{"times":["8:00"]}`)); err == nil {
		t.Errorf("invalid schedule must not normalize")
	}
}
