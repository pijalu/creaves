package actions

// Schedule humanizer (bugs.md U12): renders the §4.3 JSON document as a
// localized sentence — "Every day at 08:00, 12:00, from 2026-09-29,
// for 5 days" — so no caretaker ever has to decode raw JSON. The raw
// document stays reachable in the UI behind a disclosure for admins.
//
// Also home of the displayPlanName plush helper (bugs.md U15): every plan/
// rule name render goes through DisplayName so migration markers never
// leak into the UI.

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
)

// humanizeSchedule renders a stored §4.3 schedule JSON document as a short
// localized sentence for the request locale.
func humanizeSchedule(c buffalo.Context, raw string) string {
	return humanizeScheduleWith(raw, func(id string, args map[string]interface{}) string {
		if args == nil {
			return T.Translate(c, id)
		}
		return T.Translate(c, id, args)
	})
}

// humanizeScheduleWith is the locale-independent core: tr resolves a
// translation id (optionally with named args) so tests can inject a
// language without a request context. Unparseable input is returned
// verbatim (the strict validator rejected it at write time, so this is
// only a display fallback for legacy rows).
func humanizeScheduleWith(raw string, tr func(id string, args map[string]interface{}) string) string {
	s, err := careplan.ParseScheduleJSON([]byte(raw))
	if err != nil {
		return raw
	}

	times := make([]string, len(s.Times))
	for i, t := range s.Times {
		times[i] = t.String()
	}
	at := strings.Join(times, ", ")

	var b strings.Builder
	// BUG-51: a schedule authored in repeat mode carries BOTH the repeat
	// block and its full slot expansion. The details views must show the
	// COMPACT window form — "Chaque jour de 09:00 à 17:00 toutes les 04:00 h
	// (3 repas)" — never the (potentially huge) expanded slot list; the
	// meal count rides along so the caregiver still sees the cadence.
	b.WriteString(humanizeRepeatOrSlots(s, at, tr))
	b.WriteString(humanizeWeekdaysSuffix(s, tr))
	b.WriteString(humanizeAnchorSuffix(s, tr))

	if s.FromOffsetDays > 0 {
		b.WriteString(tr("care_plan.schedule_human.offset_days", map[string]interface{}{"n": s.FromOffsetDays}))
	}

	if s.DurationDays > 0 {
		b.WriteString(tr("care_plan.schedule_human.for_n_days", map[string]interface{}{"n": s.DurationDays}))
	} else {
		b.WriteString(tr("care_plan.schedule_human.open_ended", nil))
	}

	return b.String()
}

// humanizeRepeatOrSlots renders the schedule head: the compact repeat-window
// form (BUG-51) when the schedule was authored in repeat mode, the slot list
// otherwise.
func humanizeRepeatOrSlots(s careplan.Schedule, at string, tr func(id string, args map[string]interface{}) string) string {
	if s.Repeat != nil {
		every := fmt.Sprintf("%02d:%02d", s.Repeat.EveryMinutes/60, s.Repeat.EveryMinutes%60)
		args := map[string]interface{}{
			"from":  s.Repeat.From.String(),
			"to":    s.Repeat.To.String(),
			"every": every,
			"count": len(s.Times),
			"n":     s.EveryDays,
		}
		if s.EveryDays == 1 {
			return tr("care_plan.schedule_human.repeat_window", args)
		}
		return tr("care_plan.schedule_human.repeat_window_n", args)
	}
	if s.EveryDays == 1 {
		return tr("care_plan.schedule_human.every_day_at", map[string]interface{}{"times": at})
	}
	return tr("care_plan.schedule_human.every_n_days_at", map[string]interface{}{"n": fmt.Sprint(s.EveryDays), "times": at})
}

// humanizeWeekdaysSuffix renders the optional "on Mon, Tue" restriction.
func humanizeWeekdaysSuffix(s careplan.Schedule, tr func(id string, args map[string]interface{}) string) string {
	if len(s.Weekdays) == 0 {
		return ""
	}
	days := make([]int, len(s.Weekdays))
	copy(days, s.Weekdays)
	sort.Ints(days)
	names := make([]string, 0, len(days))
	for _, d := range days {
		names = append(names, tr(fmt.Sprintf("care_plan.schedule_human.weekday_%d", d), nil))
	}
	return tr("care_plan.schedule_human.on_weekdays", map[string]interface{}{"days": strings.Join(names, ", ")})
}

// humanizeAnchorSuffix renders the optional anchor clause (intake or date).
func humanizeAnchorSuffix(s careplan.Schedule, tr func(id string, args map[string]interface{}) string) string {
	switch s.Anchor {
	case careplan.AnchorIntake:
		return tr("care_plan.schedule_human.from_intake", nil)
	case careplan.AnchorFixed:
		if s.AnchorDate != nil {
			return tr("care_plan.schedule_human.from_date", map[string]interface{}{"date": s.AnchorDate.Format("2006-01-02")})
		}
	}
	return ""
}

// contentLabel derives a "says the thing" label from a §4.2 action payload
// (bugs.md U15): drug+dosage for medication, diet text for feeding, the
// prompt for observation, instructions otherwise. Empty when the payload
// carries no displayable content.
func contentLabel(kind string, payload []byte) string {
	var m map[string]interface{}
	if err := json.Unmarshal(payload, &m); err != nil {
		return ""
	}
	str := func(k string) string {
		if v, ok := m[k].(string); ok {
			return strings.TrimSpace(v)
		}
		return ""
	}
	switch kind {
	case "medication":
		drug, dosage := str("drug"), str("dosage")
		if drug == "" {
			return ""
		}
		if dosage != "" {
			return drug + " — " + dosage
		}
		return drug
	case "feeding":
		// Food text only — force-feed is surfaced as its own explicit
		// badge where it matters (feeding cards), not as an icon here.
		return str("food")
	case "observation":
		return str("prompt")
	default:
		if s := str("instructions"); s != "" {
			return s
		}
		return str("note")
	}
}

// richPlanName renders a plan/rule name with content (bugs.md U15): the
// DisplayName-stripped name first; when the stored name carries no content
// of its own (converter artifacts like "nb 1/2"), the payload-derived
// content label is appended — "nb 1/2 — Nutribird A21". When the name
// already mentions the content (or is empty), no duplication.
func richPlanName(name, kind string, payload []byte) string {
	base := DisplayName(name)
	content := contentLabel(kind, payload)
	if content == "" {
		return base
	}
	// Main token = text before the " — " suffix (drug name, diet text).
	main := strings.SplitN(content, " — ", 2)[0]
	if base == "" {
		return content
	}
	if strings.Contains(strings.ToLower(base), strings.ToLower(main)) {
		return base
	}
	return base + " — " + content
}
