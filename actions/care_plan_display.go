package actions

import (
	"time"

	"creaves/models/careplan"
)

// Round-2 display layer (docs/care-plan-ux-fix-plan-round2.md §6.2): pure
// functions deriving the current-vs-superseded display semantics on top of
// the unchanged engine statuses. No persistence, no migration — everything
// here derives from PlanItem fields at render time.
//
// One mechanism only (§6.2-4, the user's grace rule): an occurrence leaves
// the work set when its successor becomes imminent (successor due minus
// lookahead ≤ now) or when the window lapses with no successor. The time an
// occurrence stays "late but current" is therefore bounded by the source's
// cadence, not by a fixed window.

// IsCurrent reports whether an OPEN occurrence is still an actionable work
// item at `now` (§6.2-1): applicable (§10-A1 window) AND its successor is
// not imminent — successor due time minus the source's lookahead > now. The
// window's last occurrence (zero NextDue) stays current while applicable.
// Terminal and overridden items are never current.
func IsCurrent(item *careplan.PlanItem, now time.Time) bool {
	if item == nil || item.Occurrence.Source == nil {
		return false
	}
	switch item.Status {
	case careplan.StatusOverridden, careplan.StatusApplied,
		careplan.StatusSkipped, careplan.StatusDeferred:
		return false
	}
	if !item.Applicable {
		return false
	}
	if item.NextDue.IsZero() {
		return true
	}
	_, _, lookahead := careplan.ScheduleWindows(item.Occurrence.Source)
	return item.NextDue.Sub(now) > lookahead
}

// SupersededReason explains why an open occurrence left the work set
// (§6.2-2): "replaced" when a successor of the same source exists, "window"
// when the occurrence lapsed without one (single daily occurrence past the
// window end). Empty for current items and for terminal statuses (applied /
// skipped / deferred stay truthful, never "superseded").
func SupersededReason(item *careplan.PlanItem, now time.Time) string {
	if item == nil || item.Occurrence.Source == nil {
		return ""
	}
	switch item.Status {
	case careplan.StatusOverridden, careplan.StatusApplied,
		careplan.StatusSkipped, careplan.StatusDeferred:
		return ""
	}
	if IsCurrent(item, now) {
		return ""
	}
	if item.NextDue.IsZero() {
		return "window"
	}
	return "replaced"
}

// DueLabelParts is the date-aware due label of an occurrence (§6.2-5,
// fixes "En retard 08:00" at 07:54): bare time only for today, day-word
// prefix for yesterday/tomorrow, short date for bigger gaps. The caller
// localizes DayKey / composes the final string — FormatDueLabel does it
// from the two localized day words.
type DueLabelParts struct {
	DayKey    string // "" (today) | "care_plan.time.yesterday" | "care_plan.time.tomorrow" | "" with ShortDate set
	ShortDate string // "02/10" when the gap exceeds one day (DayKey empty)
	TimeHM    string // "15:04"
}

// DueLabelPartsOf derives the label parts of one occurrence at `now`.
func DueLabelPartsOf(dueAt, now time.Time) DueLabelParts {
	p := DueLabelParts{TimeHM: dueAt.Format("15:04")}
	loc := now.Location()
	today := time.Date(now.In(loc).Year(), now.In(loc).Month(), now.In(loc).Day(), 0, 0, 0, 0, loc)
	day := time.Date(dueAt.In(loc).Year(), dueAt.In(loc).Month(), dueAt.In(loc).Day(), 0, 0, 0, 0, loc)
	// bugs.md BUG-19: hour-delta arithmetic mislabels DST days — a 23-hour
	// "day" truncates tomorrow's gap to 0 ("today") and a 25-hour day
	// misreads as 2 days out. Count calendar days via AddDate steps.
	days := 0
	for d := today; d.Before(day); d = d.AddDate(0, 0, 1) {
		days++
	}
	for d := day; d.Before(today); d = d.AddDate(0, 0, 1) {
		days--
	}
	switch days {
	case 0:
		// today — time only
	case -1:
		p.DayKey = "care_plan.time.yesterday"
	case 1:
		p.DayKey = "care_plan.time.tomorrow"
	default:
		p.ShortDate = dueAt.Format("02/01")
	}
	return p
}

// FormatDueLabel composes the final label from localized day words:
// "hier 08:00" / "08:00" / "demain 08:00" / "02/10 08:00" (§6.2-5). Empty
// words degrade to the bare time (Postel: never a broken label).
func FormatDueLabel(p DueLabelParts, yesterday, tomorrow string) string {
	day := ""
	switch {
	case p.ShortDate != "":
		day = p.ShortDate
	case p.DayKey == "care_plan.time.yesterday":
		day = yesterday
	case p.DayKey == "care_plan.time.tomorrow":
		day = tomorrow
	}
	if day == "" {
		return p.TimeHM
	}
	return day + " " + p.TimeHM
}
