package careplan

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
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

	// Repeat, when present, is the authoring form of Times (BUG-36):
	// "from/to every X" expanded at save time. Times always stays
	// materialized and remains the engine contract; Repeat is metadata
	// for the editor round-trip.
	Repeat *Repeat

	// BUG-45: purely presentational per-tier display caps — how many
	// occurrence buttons each tier (Late / Now / Later) renders on the
	// plan lines before folding the overflow into a "+N more" badge.
	// 0 = uncapped (default). No effect on scheduling or apply logic:
	// every occurrence stays recordable (batch apply covers ALL).
	DisplayMaxLate   int
	DisplayMaxNow    int
	DisplayMaxFuture int
}

// Repeat is the optional "from/to every X" schedule authoring form
// (BUG-36). From and To are day times (HH:MM); To < From reads as an
// overnight window (end += 24 h); EveryMinutes is the period (≥ 1).
// Expansion semantics match the legacy DeriveFeedingTimes: start slot
// included, start + n×period ≤ end, cap 24 slots.
type Repeat struct {
	From         TimeOfDay
	To           TimeOfDay
	EveryMinutes int
}

// maxRepeatSlots is the sanity cap on repeat expansion (§11.7 parity).
const maxRepeatSlots = 24

// ExpandRepeat expands a repeat definition into daily slots: From +
// n×EveryMinutes ≤ To (To wrapped +24 h when before From), From included,
// at most maxRepeatSlots slots. The result is sorted ascending (the §4.3
// times invariant). A zero/negative period yields nil — Validate rejects
// it before expansion matters.
func ExpandRepeat(r Repeat) []TimeOfDay {
	if r.EveryMinutes <= 0 {
		return nil
	}
	start := r.From.Minutes()
	end := r.To.Minutes()
	if end < start {
		end += 24 * 60 // overnight window
	}
	var out []TimeOfDay
	for m := start; m <= end; m += r.EveryMinutes {
		if len(out) >= maxRepeatSlots {
			break
		}
		out = append(out, TimeOfDay{Hour: (m % (24 * 60)) / 60, Minute: m % 60})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Minutes() < out[j].Minutes() })
	return out
}

// FitsCap reports whether the repeat expansion stays within the sanity
// slot cap — a repeat beyond the cap fails validate(). Callers building a
// document from external data (the legacy converter) must check this
// before emitting a repeat block: a capped expansion would silently drop
// slots, so an over-cap repeat must take the flagged fallback path instead.
func (r Repeat) FitsCap() bool {
	return r.EveryMinutes >= 1 && r.WindowMinutes()/r.EveryMinutes+1 <= maxRepeatSlots
}

// WindowMinutes returns the span covered by the repeat (To wrapped +24 h
// when before From).
func (r Repeat) WindowMinutes() int {
	end := r.To.Minutes()
	if end < r.From.Minutes() {
		end += 24 * 60
	}
	return end - r.From.Minutes()
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
	Times            []string    `json:"times"`
	EveryDays        *int        `json:"every_days,omitempty"`
	Weekdays         []int       `json:"weekdays,omitempty"`
	Anchor           *string     `json:"anchor,omitempty"`
	AnchorDate       *string     `json:"anchor_date,omitempty"`
	FromOffsetDays   *int        `json:"from_offset_days,omitempty"`
	DurationDays     *int        `json:"duration_days,omitempty"`
	GraceMinutes     *int        `json:"grace_minutes,omitempty"`
	MissAfterHours   *int        `json:"miss_after_hours,omitempty"`
	LookaheadMinutes *int        `json:"lookahead_minutes,omitempty"`
	Repeat           *repeatJSON `json:"repeat,omitempty"`
	// BUG-45 display caps (0/absent = uncapped).
	DisplayMaxLate   *int `json:"display_max_late,omitempty"`
	DisplayMaxNow    *int `json:"display_max_now,omitempty"`
	DisplayMaxFuture *int `json:"display_max_future,omitempty"`
}

// repeatJSON mirrors the optional "repeat" block (BUG-36).
type repeatJSON struct {
	From         string `json:"from"`
	To           string `json:"to"`
	EveryMinutes int    `json:"every_minutes"`
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
	// BUG-45: negative caps are clamped to 0 (uncapped), never rejected —
	// the counters are UX-only, a bad value must not block a save.
	if sj.DisplayMaxLate != nil && *sj.DisplayMaxLate > 0 {
		s.DisplayMaxLate = *sj.DisplayMaxLate
	}
	if sj.DisplayMaxNow != nil && *sj.DisplayMaxNow > 0 {
		s.DisplayMaxNow = *sj.DisplayMaxNow
	}
	if sj.DisplayMaxFuture != nil && *sj.DisplayMaxFuture > 0 {
		s.DisplayMaxFuture = *sj.DisplayMaxFuture
	}
	if sj.Anchor != nil {
		s.Anchor = *sj.Anchor
	}
	s.Weekdays = sj.Weekdays

	if sj.Repeat != nil {
		r, err := sj.Repeat.parse()
		if err != nil {
			return Schedule{}, err
		}
		s.Repeat = &r
	}

	for _, ts := range sj.Times {
		tod, err := ParseTimeOfDay(ts)
		if err != nil {
			return Schedule{}, fmt.Errorf("careplan: schedule.times: %w", err)
		}
		s.Times = append(s.Times, tod)
	}
	// Repeat without explicit times: derive the slots from the repeat
	// (NormalizeScheduleJSON canonical form; the times array stays the
	// engine contract).
	if s.Repeat != nil && sj.Times == nil {
		s.Times = ExpandRepeat(*s.Repeat)
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
	if s.Repeat != nil {
		if err := s.Repeat.validate(); err != nil {
			return err
		}
		// Repeat is the authoring form of Times: stored times must be
		// exactly its expansion (the server re-derives at save, so a
		// mismatch means a hand-edited or stale document).
		want := ExpandRepeat(*s.Repeat)
		if !timesEqual(s.Times, want) {
			return fmt.Errorf("careplan: schedule.times inconsistent with schedule.repeat (want expansion %s)", formatTimes(want))
		}
	}
	return nil
}

// parse converts and validates the stored repeat block.
func (rj *repeatJSON) parse() (Repeat, error) {
	from, err := ParseTimeOfDay(rj.From)
	if err != nil {
		return Repeat{}, fmt.Errorf("careplan: schedule.repeat.from: %w", err)
	}
	to, err := ParseTimeOfDay(rj.To)
	if err != nil {
		return Repeat{}, fmt.Errorf("careplan: schedule.repeat.to: %w", err)
	}
	return Repeat{From: from, To: to, EveryMinutes: rj.EveryMinutes}, nil
}

// validate checks the repeat invariants: period ≥ 1, and the window must
// not run into the 24-slot cap (a capped expansion would silently drop
// slots — reject instead).
func (r Repeat) validate() error {
	if r.From.Hour < 0 || r.From.Hour > 23 || r.From.Minute < 0 || r.From.Minute > 59 ||
		r.To.Hour < 0 || r.To.Hour > 23 || r.To.Minute < 0 || r.To.Minute > 59 {
		return fmt.Errorf("careplan: schedule.repeat: invalid time of day")
	}
	if r.EveryMinutes < 1 {
		return fmt.Errorf("careplan: schedule.repeat.every_minutes must be ≥ 1, got %d", r.EveryMinutes)
	}
	if r.WindowMinutes()/r.EveryMinutes+1 > maxRepeatSlots {
		return fmt.Errorf("careplan: schedule.repeat expands to more than %d slots (from %s to %s every %d min)",
			maxRepeatSlots, r.From, r.To, r.EveryMinutes)
	}
	return nil
}

// timesEqual compares two slot lists element-wise.
func timesEqual(a, b []TimeOfDay) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// formatTimes renders a slot list for error messages.
func formatTimes(ts []TimeOfDay) string {
	parts := make([]string, len(ts))
	for i, t := range ts {
		parts[i] = t.String()
	}
	return "[" + strings.Join(parts, " ") + "]"
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

// MarshalJSON renders the canonical §4.3 document of a validated Schedule:
// times always materialized, non-default fields present, repeat block when
// set (BUG-36). This is the save-time serialization counterpart of
// ParseScheduleJSON.
func (s Schedule) MarshalJSON() ([]byte, error) {
	sj := scheduleJSON{
		Times:    make([]string, len(s.Times)),
		Weekdays: s.Weekdays,
	}
	for i, t := range s.Times {
		sj.Times[i] = t.String()
	}
	if s.EveryDays != 1 {
		sj.EveryDays = &s.EveryDays
	}
	if s.Anchor != AnchorIntake {
		sj.Anchor = &s.Anchor
	}
	if s.AnchorDate != nil {
		d := s.AnchorDate.Format("2006-01-02")
		sj.AnchorDate = &d
	}
	if s.FromOffsetDays != 0 {
		sj.FromOffsetDays = &s.FromOffsetDays
	}
	if s.DurationDays != 0 {
		sj.DurationDays = &s.DurationDays
	}
	if s.GraceMinutes != 60 {
		sj.GraceMinutes = &s.GraceMinutes
	}
	if s.MissAfterHours != 24 {
		sj.MissAfterHours = &s.MissAfterHours
	}
	if s.LookaheadMinutes != 60 {
		sj.LookaheadMinutes = &s.LookaheadMinutes
	}
	if s.DisplayMaxLate > 0 {
		sj.DisplayMaxLate = &s.DisplayMaxLate
	}
	if s.DisplayMaxNow > 0 {
		sj.DisplayMaxNow = &s.DisplayMaxNow
	}
	if s.DisplayMaxFuture > 0 {
		sj.DisplayMaxFuture = &s.DisplayMaxFuture
	}
	if s.Repeat != nil {
		sj.Repeat = &repeatJSON{
			From:         s.Repeat.From.String(),
			To:           s.Repeat.To.String(),
			EveryMinutes: s.Repeat.EveryMinutes,
		}
	}
	return json.Marshal(sj)
}

// ScheduleErrorMessage renders a schedule parse/validate error for user
// display: the internal "careplan: schedule[…]:" prefixes are stripped so
// the message never leaks Go/JSON internals, and opaque spec/bug
// references (§-paragraphs, BUG-nn) are removed — those mean nothing to
// a caretaker reading a 422 field error.
func ScheduleErrorMessage(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if i := strings.Index(msg, ": "); i >= 0 && strings.HasPrefix(msg, "careplan:") {
		msg = msg[i+2:]
	}
	if i := strings.Index(msg, " (§"); i >= 0 {
		msg = msg[:i]
	}
	msg = strings.ReplaceAll(msg, "§", "")
	for strings.Contains(msg, "BUG-") {
		i := strings.Index(msg, "BUG-")
		j := i + 4
		for j < len(msg) && msg[j] >= '0' && msg[j] <= '9' {
			j++
		}
		msg = msg[:i] + msg[j:]
	}
	return strings.TrimSpace(msg)
}

// NormalizeScheduleJSON returns the canonical form of a §4.3 schedule
// document (BUG-36 decision 3): when the document carries a repeat block,
// Times is re-derived from it — repeat is the source of truth and
// client-supplied times are ignored; without a repeat block the document
// is re-marshaled as-is (canonical key set, defaults dropped). The result
// is what save/update must persist.
func NormalizeScheduleJSON(raw []byte) ([]byte, error) {
	// Strict-decode the raw document first: a repeat doc with stale or
	// tampered times must be fixable here, so the consistency check runs
	// only on the re-derived document (via ParseScheduleJSON below).
	dec := json.NewDecoder(strings.NewReader(string(raw)))
	dec.DisallowUnknownFields()
	var sj scheduleJSON
	if err := dec.Decode(&sj); err != nil {
		return nil, fmt.Errorf("careplan: invalid schedule JSON: %w", err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("careplan: invalid schedule JSON: trailing data")
	}
	if sj.Repeat != nil {
		r, err := sj.Repeat.parse()
		if err != nil {
			return nil, err
		}
		if err := r.validate(); err != nil {
			return nil, err
		}
		expanded := ExpandRepeat(r)
		sj.Times = make([]string, len(expanded))
		for i, t := range expanded {
			sj.Times[i] = t.String()
		}
	}
	canon, err := json.Marshal(sj)
	if err != nil {
		return nil, err
	}
	s, err := ParseScheduleJSON(canon)
	if err != nil {
		return nil, err
	}
	return s.MarshalJSON()
}
