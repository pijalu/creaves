package careplan

import (
	"sort"
	"time"
)

// Compact view (bugs.md U7/U8): the work screen's default lens over the day
// plan. Caretakers under time pressure need the *next* open action per
// (source × animal) — not the full occurrence list — with the remaining
// open work signalled as a count badge ("+n") instead of rows.

// NextOpenItem is the compact-view row: the earliest non-terminal
// occurrence of one (source × animal) group, plus counts of the group's
// other in-window occurrences.
type NextOpenItem struct {
	Item PlanItem
	// Remaining counts the group's OTHER open occurrences in the window
	// (scheduled|due|late|missing) — rendered as a "+n" badge.
	Remaining int
	// Done counts the group's terminal occurrences in the window
	// (applied|skipped|deferred) — feeds the "done" tier counters.
	Done int
}

// openStatus reports whether a status still requires action (§6.1).
func openStatus(s PlanStatus) bool {
	switch s {
	case StatusScheduled, StatusDue, StatusLate, StatusMissing:
		return true
	}
	return false
}

// urgencyRank orders the compact list most-urgent-first (Selective
// Attention / Serial Position — laws-of-ux skill): missed work leads,
// future work trails.
func urgencyRank(s PlanStatus) int {
	switch s {
	case StatusMissing:
		return 0
	case StatusLate:
		return 1
	case StatusDue:
		return 2
	case StatusScheduled:
		return 3
	}
	return 4
}

// NextOpenPerGroup reduces a full plan item list to one NextOpenItem per
// (source_type, source_id, animal_id) group: the earliest open occurrence
// (the "next" action) plus Remaining/Done counts. Overridden occurrences
// (§4.7) never surface in the work view — they stay visible on the animal
// Plan tab. Groups with no open occurrence are omitted (their Done count
// still feeds tier counters via the caller's full list).
//
// Input order does not matter; output is sorted by urgency rank, then
// DueAt, then animal id, then source id — fully deterministic.
func NextOpenPerGroup(items []PlanItem) []NextOpenItem {
	type key struct {
		typ    SourceType
		id     string
		animal int
	}
	type acc struct {
		next  *PlanItem // earliest open occurrence (the group's action)
		open  int       // open occurrences in-window (incl. next)
		done  int       // terminal occurrences in-window
	}
	groups := map[key]*acc{}
	order := []key{}
	for i := range items {
		it := &items[i]
		src := it.Occurrence.Source
		if src == nil || it.Status == StatusOverridden {
			continue // overridden never surfaces in the work view (§4.7)
		}
		k := key{src.SourceType(), src.SourceID(), it.Occurrence.AnimalID}
		g, ok := groups[k]
		if !ok {
			g = &acc{}
			groups[k] = g
			order = append(order, k)
		}
		if openStatus(it.Status) {
			g.open++
			if g.next == nil || it.Occurrence.DueAt.Before(g.next.Occurrence.DueAt) {
				g.next = it // keep the EARLIEST open occurrence
			}
		} else {
			g.done++
		}
	}
	out := make([]NextOpenItem, 0, len(groups))
	for _, k := range order {
		g := groups[k]
		if g.next == nil {
			continue // no open occurrence in-window — nothing to act on
		}
		out = append(out, NextOpenItem{Item: *g.next, Remaining: g.open - 1, Done: g.done})
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ra, rb := urgencyRank(a.Item.Status), urgencyRank(b.Item.Status); ra != rb {
			return ra < rb
		}
		if !a.Item.Occurrence.DueAt.Equal(b.Item.Occurrence.DueAt) {
			return a.Item.Occurrence.DueAt.Before(b.Item.Occurrence.DueAt)
		}
		if a.Item.Occurrence.AnimalID != b.Item.Occurrence.AnimalID {
			return a.Item.Occurrence.AnimalID < b.Item.Occurrence.AnimalID
		}
		return sourceIDOf(a.Item.Occurrence.Source) < sourceIDOf(b.Item.Occurrence.Source)
	})
	return out
}

// BadgeCap renders a count badge capped at 99+ (bugs.md U2): a three-digit
// badge breaks pill layout and carries no actionable information.
func BadgeCap(n int) string {
	if n > 99 {
		return "99+"
	}
	if n < 0 {
		return "0"
	}
	return itoa(n)
}

// itoa avoids importing strconv for two call sites.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [8]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

// WorkWindow returns the default HTML work-screen window (bugs.md U4):
// yesterday 00:00 (overdue carry-over) through end of today. The spec
// window (§6.1, −1d…+2d) stays the JSON/report default — the work screen
// is a "what do I do next" surface, not a multi-day trace.
func WorkWindow(now time.Time) (from, to time.Time) {
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	from = dayStart.AddDate(0, 0, -1)
	to = dayStart.Add(24*time.Hour - time.Nanosecond)
	return from, to
}
