package actions

import (
	"time"

	"creaves/models/careplan"
)

// Item 8 of the 2026-10-07 day-plan review: granularity-aware late
// handling.
//
// A fixed "show late work for N hours" window cannot fit every schedule: a
// 30-minute gavage cadence amasses a wall of stale late toggles within an
// hour, while a 12-hour medication slot is barely late after the same
// time. The heuristic is therefore RELATIVE: a late/missing occurrence is
// STALE when its group's next open occurrence is nearer to now than the
// late one is — past the midpoint between the missed and the next
// occurrence:
//
//	now - late_due  >  next_due - now
//
// Granularity-aware by construction (a 12 h cadence tolerates ~6 h of
// lateness, a 30 min cadence ~15 min), and self-closing: when the next
// occurrence becomes due-now (next_in = 0) every pending late folds —
// "activating the future closes the late". The absolute /preferences caps
// stay the OUTER bound (work beyond them leaves the day plan entirely, as
// before).
//
// A stale occurrence is FOLDED, not deleted (user ruling): the record
// stays in the Late section, the stale toggles collapse into one "⏱ N"
// badge with a snooze (defer) action, and the next upcoming occurrence
// renders as the actionable toggle beside it. Folded occurrences are
// excluded from batch refs so a group apply never silently records them.

// lateIsStale reports whether one late/missing occurrence's reminder folds
// into the record's stale badge (item 8): its age exceeds the distance to
// the group's nearest open future occurrence.
func lateIsStale(age, nearestIn time.Duration, hasSibling bool) bool {
	return hasSibling && age > nearestIn
}

// openSiblingInDist reports the distance from now to one OPEN sibling
// occurrence usable as "the next upcoming action": 0 for a due-now slot
// (its reminder is already active), the future delay for a scheduled one.
// Late siblings never fold each other; terminal and overridden occurrences
// are not work.
func openSiblingInDist(status careplan.PlanStatus, due time.Time, now time.Time) (time.Duration, bool) {
	switch tierOrder(status) {
	case 1: // due now
		return 0, true
	case 2: // scheduled
		if due.After(now) {
			return due.Sub(now), true
		}
	}
	return 0, false
}

// staleLateFlags marks, per item, whether an open late/missing occurrence
// is stale within its (source × animal) group (item 8). The slice covers
// ONE group's items (one cleanup/feeding card's items).
func staleLateFlags(items []careplan.PlanItem, now time.Time) []bool {
	type key struct {
		typ, id string
		animal  int
	}
	nearest := map[key]time.Duration{}
	has := map[key]bool{}
	out := make([]bool, len(items))
	for i := range items {
		it := &items[i]
		src := it.Occurrence.Source
		if src == nil {
			continue
		}
		if d, ok := openSiblingInDist(it.Status, it.Occurrence.DueAt, now); ok {
			k := key{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
			if !has[k] || d < nearest[k] {
				nearest[k], has[k] = d, true
			}
		}
	}
	for i := range items {
		it := &items[i]
		src := it.Occurrence.Source
		if src == nil || tierOrder(it.Status) != 0 {
			continue
		}
		k := key{string(src.SourceType()), src.SourceID(), it.Occurrence.AnimalID}
		out[i] = lateIsStale(now.Sub(it.Occurrence.DueAt), nearest[k], has[k])
	}
	return out
}

// foldStaleItemSlots marks one row-kind group's stale late occurrence
// slots (item 8). Only ACTIONABLE lates fold — a hors-délai 🔒 marker is
// already non-interactive info and keeps its place.
//
// Rev (2026-10-07, collapsible): the stale slots STAY in `kept`, flagged
// with Stale — the record's "⏱ N" badge is a collapsible and reveals their
// toggles again on click. They are excluded from the batch refs, from the
// line's re-stamp and from the remaining count (the caller skips Stale);
// `stale` repeats the flagged subset for the badge count and refs.
func foldStaleItemSlots(slots []ItemSlotView, now time.Time) (kept, stale []ItemSlotView) {
	var nearestIn time.Duration
	hasSibling := false
	for _, s := range slots {
		if d, ok := openSiblingInDist(careplan.PlanStatus(s.Status), s.DueAt, now); ok {
			if !hasSibling || d < nearestIn {
				nearestIn, hasSibling = d, true
			}
		}
	}
	if !hasSibling {
		return slots, nil
	}
	for i, s := range slots {
		if tierOrder(careplan.PlanStatus(s.Status)) == 0 && s.Applicable &&
			lateIsStale(now.Sub(s.DueAt), nearestIn, true) {
			slots[i].Stale = true
			stale = append(stale, slots[i])
		}
	}
	return slots, stale
}

// foldStaleMedSlots is foldStaleItemSlots' medication twin: a series'
// record-late toggles are also actionable when non-applicable
// (slotLateAllowed), so both actionable late shapes fold.
func foldStaleMedSlots(slots []MedSlotView, now time.Time) (kept, stale []MedSlotView) {
	var nearestIn time.Duration
	hasSibling := false
	for _, s := range slots {
		if d, ok := openSiblingInDist(careplan.PlanStatus(s.Status), s.DueAt, now); ok {
			if !hasSibling || d < nearestIn {
				nearestIn, hasSibling = d, true
			}
		}
	}
	if !hasSibling {
		return slots, nil
	}
	for _, s := range slots {
		if tierOrder(careplan.PlanStatus(s.Status)) == 0 && (s.Applicable || s.LateAllowed) &&
			lateIsStale(now.Sub(s.DueAt), nearestIn, true) {
			stale = append(stale, s)
			continue
		}
		kept = append(kept, s)
	}
	return kept, stale
}

// hasPendingLateSlot reports whether an actionable late/missing slot is
// pending in the set — item 8's co-display trigger ("at least show in late
// the next upcoming action").
func hasPendingLateSlot(slots []MedSlotView) bool {
	for _, s := range slots {
		if !s.Done && !s.Overridden && tierOrder(careplan.PlanStatus(s.Status)) == 0 &&
			(s.Applicable || s.LateAllowed) {
			return true
		}
	}
	return false
}
