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
	switch s.EveryDays {
	case 1:
		b.WriteString(tr("care_plan.schedule_human.every_day_at", map[string]interface{}{"times": at}))
	default:
		b.WriteString(tr("care_plan.schedule_human.every_n_days_at", map[string]interface{}{"n": fmt.Sprint(s.EveryDays), "times": at}))
	}

	if len(s.Weekdays) > 0 {
		days := make([]int, len(s.Weekdays))
		copy(days, s.Weekdays)
		sort.Ints(days)
		names := make([]string, 0, len(days))
		for _, d := range days {
			names = append(names, tr(fmt.Sprintf("care_plan.schedule_human.weekday_%d", d), nil))
		}
		b.WriteString(tr("care_plan.schedule_human.on_weekdays", map[string]interface{}{"days": strings.Join(names, ", ")}))
	}

	switch s.Anchor {
	case careplan.AnchorIntake:
		b.WriteString(tr("care_plan.schedule_human.from_intake", nil))
	case careplan.AnchorFixed:
		if s.AnchorDate != nil {
			b.WriteString(tr("care_plan.schedule_human.from_date", map[string]interface{}{"date": s.AnchorDate.Format("2006-01-02")}))
		}
	}

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
