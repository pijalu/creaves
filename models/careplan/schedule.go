package careplan

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// Schedule is the validated §4.3 schedule value object (shared JSON schema
// of care_rules and care_animal_plans — full parity, §4.7). It is a pure
// value: defaults are applied at parse time, validation is strict (unknown
// keys are typos, not forward compatibility), and the occurrence generator
// (occurrences.go) is the only consumer of its internals.
type Schedule struct {
	// Times are the exhaustive daily slots, strictly ascending (§4.3).
	Times []TimeOfDay

	// EveryDays is the repetition step from the anchor (1 = daily).
	EveryDays int

	// Weekdays restricts occurrence days (0=Sunday..6=Saturday); nil = all.
	Weekdays []int

	// Anchor is "intake" or "fixed"; AnchorDate (local midnight) is set iff
	// Anchor == "fixed".
	Anchor     string
	AnchorDate *time.Time

	// FromOffsetDays shifts the first occurrence day from the anchor (≥0).
	FromOffsetDays int

	// DurationDays bounds the course: 0 = open-ended; N = exactly N
	// occurrence DAYS generated (§10-B3: counted in generated days, not
	// calendar days).
	DurationDays int

	// Defaults per §4.3/§10.2: late after due+grace (60), missing after
	// due+miss hours (24), "due" from this long before the instant (60,
	// §10-CP6a).
	GraceMinutes     int
	MissAfterHours   int
	LookaheadMinutes int
}

// Anchor kinds (§4.3).
const (
	AnchorIntake = "intake"
	AnchorFixed  = "fixed"
)

// TimeOfDay is one validated daily slot (HH:MM, 24h).
type TimeOfDay struct {
	Hour   int
	Minute int
}

// ParseTimeOfDay parses a strict zero-padded "HH:MM" (24h).
func ParseTimeOfDay(s string) (TimeOfDay, error) {
	var h, m int
	if _, err := fmt.Sscanf(s, "%02d:%02d", &h, &m); err != nil {
		return TimeOfDay{}, fmt.Errorf("invalid time %q: want zero-padded HH:MM", s)
	}
	if len(s) != 5 || s[2] != ':' || h < 0 || h > 23 || m < 0 || m > 59 {
		return TimeOfDay{}, fmt.Errorf("invalid time %q: want zero-padded HH:MM (00:00–23:59)", s)
	}
	return TimeOfDay{Hour: h, Minute: m}, nil
}

// Minutes returns minutes since midnight (0..1439).
func (t TimeOfDay) Minutes() int { return t.Hour*60 + t.Minute }

func (t TimeOfDay) String() string {
	return fmt.Sprintf("%02d:%02d", t.Hour, t.Minute)
}

// scheduleJSON mirrors the stored §4.3 document; pointers distinguish
// "absent" (default) from "present" (validated).
type scheduleJSON struct {
	Times            []string `json:"times"`
	EveryDays        *int     `json:"every_days"`
	Weekdays         []int    `json:"weekdays"`
	Anchor           *string  `json:"anchor"`
	AnchorDate       *string  `json:"anchor_date"`
	FromOffsetDays   *int     `json:"from_offset_days"`
	DurationDays     *int     `json:"duration_days"`
	GraceMinutes     *int     `json:"grace_minutes"`
	MissAfterHours   *int     `json:"miss_after_hours"`
	LookaheadMinutes *int     `json:"lookahead_minutes"`
}

// ParseScheduleJSON parses and validates a §4.3 schedule document. Strict:
// unknown keys and absent "times" are errors; defaults are every_days=1,
// anchor="intake", grace=60 min, miss=24 h, lookahead=60 min (§10-CP6a).
func ParseScheduleJSON(raw []byte) (Schedule, error) {
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var sj scheduleJSON
	if err := dec.Decode(&sj); err != nil {
		return Schedule{}, fmt.Errorf("careplan: invalid schedule JSON: %w", err)
	}
	// Trailing garbage check (Decode would silently accept "{}{}").
	if _, err := dec.Token(); err != io.EOF {
		return Schedule{}, fmt.Errorf("careplan: invalid schedule JSON: trailing data")
	}

	s := Schedule{
		EveryDays:        1,
		Anchor:           AnchorIntake,
		GraceMinutes:     60,
		MissAfterHours:   24,
		LookaheadMinutes: 60,
	}
	if sj.EveryDays != nil {
		s.EveryDays = *sj.EveryDays
	}
	if sj.FromOffsetDays != nil {
		s.FromOffsetDays = *sj.FromOffsetDays
	}
	if sj.DurationDays != nil {
		if *sj.DurationDays <= 0 {
			return Schedule{}, fmt.Errorf("careplan: schedule.duration_days must be ≥ 1 when set (absent = open-ended), got %d", *sj.DurationDays)
		}
		s.DurationDays = *sj.DurationDays
	}
	if sj.GraceMinutes != nil {
		s.GraceMinutes = *sj.GraceMinutes
	}
	if sj.MissAfterHours != nil {
		s.MissAfterHours = *sj.MissAfterHours
	}
	if sj.LookaheadMinutes != nil {
		s.LookaheadMinutes = *sj.LookaheadMinutes
	}
	if sj.Anchor != nil {
		s.Anchor = *sj.Anchor
	}
	s.Weekdays = sj.Weekdays

	for _, ts := range sj.Times {
		tod, err := ParseTimeOfDay(ts)
		if err != nil {
			return Schedule{}, fmt.Errorf("careplan: schedule.times: %w", err)
		}
		s.Times = append(s.Times, tod)
	}
	if sj.AnchorDate != nil {
		d, err := time.ParseInLocation("2006-01-02", *sj.AnchorDate, time.Local)
		if err != nil {
			return Schedule{}, fmt.Errorf("careplan: schedule.anchor_date: want YYYY-MM-DD, got %q", *sj.AnchorDate)
		}
		s.AnchorDate = &d
	}
	if err := s.Validate(); err != nil {
		return Schedule{}, err
	}
	return s, nil
}

// Validate checks the invariants of a Schedule (also used directly by model
// validation when a Schedule was built in code rather than parsed from JSON).
func (s Schedule) Validate() error {
	if len(s.Times) == 0 {
		return fmt.Errorf("careplan: schedule.times needs at least one HH:MM slot (§4.3)")
	}
	for i, t := range s.Times {
		if t.Hour < 0 || t.Hour > 23 || t.Minute < 0 || t.Minute > 59 {
			return fmt.Errorf("careplan: schedule.times[%d]: invalid time of day", i)
		}
		if i > 0 && t.Minutes() <= s.Times[i-1].Minutes() {
			return fmt.Errorf("careplan: schedule.times must be strictly ascending, got %s after %s", t, s.Times[i-1])
		}
	}
	if s.EveryDays < 1 {
		return fmt.Errorf("careplan: schedule.every_days must be ≥ 1, got %d", s.EveryDays)
	}
	seen := make(map[int]bool, len(s.Weekdays))
	for _, w := range s.Weekdays {
		if w < 0 || w > 6 {
			return fmt.Errorf("careplan: schedule.weekdays must be 0 (Sunday)..6 (Saturday), got %d", w)
		}
		if seen[w] {
			return fmt.Errorf("careplan: schedule.weekdays has duplicate day %d", w)
		}
		seen[w] = true
	}
	switch s.Anchor {
	case AnchorIntake:
		if s.AnchorDate != nil {
			return fmt.Errorf("careplan: schedule.anchor_date must be null when anchor=intake")
		}
	case AnchorFixed:
		if s.AnchorDate == nil {
			return fmt.Errorf("careplan: schedule.anchor_date is required when anchor=fixed")
		}
	default:
		return fmt.Errorf("careplan: schedule.anchor must be %q or %q, got %q", AnchorIntake, AnchorFixed, s.Anchor)
	}
	if s.FromOffsetDays < 0 {
		return fmt.Errorf("careplan: schedule.from_offset_days must be ≥ 0, got %d", s.FromOffsetDays)
	}
	if s.DurationDays < 0 {
		return fmt.Errorf("careplan: schedule.duration_days must be ≥ 1 when set, got %d", s.DurationDays)
	}
	if s.GraceMinutes < 0 {
		return fmt.Errorf("careplan: schedule.grace_minutes must be ≥ 0, got %d", s.GraceMinutes)
	}
	if s.MissAfterHours < 1 {
		return fmt.Errorf("careplan: schedule.miss_after_hours must be ≥ 1, got %d", s.MissAfterHours)
	}
	if s.LookaheadMinutes < 0 {
		return fmt.Errorf("careplan: schedule.lookahead_minutes must be ≥ 0, got %d", s.LookaheadMinutes)
	}
	return nil
}

// OpenEnded reports whether the schedule generates occurrences until the
// source stops matching (duration_days null, §4.3).
func (s Schedule) OpenEnded() bool { return s.DurationDays == 0 }

// AllowsWeekday reports whether w is an occurrence weekday (nil = all).
func (s Schedule) AllowsWeekday(w time.Weekday) bool {
	if len(s.Weekdays) == 0 {
		return true
	}
	for _, d := range s.Weekdays {
		if int(w) == d {
			return true
		}
	}
	return false
}

// SlotsAfter returns the first Times index strictly after the given time of
// day in minutes; len(Times) when none (§10-B3 day-1 anchoring).
func (s Schedule) SlotsAfter(minutes int) int {
	for i, t := range s.Times {
		if t.Minutes() > minutes {
			return i
		}
	}
	return len(s.Times)
}
