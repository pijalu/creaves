package careplan

import (
	"fmt"
	"sort"
	"time"
)

// Plan items and status computation (§6.1 steps 3–4, §10-A1, §10-H1):
// virtual occurrences joined with stored applications and the override
// resolution yield the day-plan read model. All statuses derive at render
// time — no cron, no sweeper (§6.1).

// PlanStatus is one of the 8 §6.1 statuses.
type PlanStatus string

const (
	StatusScheduled  PlanStatus = "scheduled"  // due in the future
	StatusDue        PlanStatus = "due"        // due ≤ now+lookahead (§10-CP6a)
	StatusLate       PlanStatus = "late"       // now > due+grace, no application
	StatusMissing    PlanStatus = "missing"    // now > due+miss, no application
	StatusApplied    PlanStatus = "applied"    // application exists (applied)
	StatusSkipped    PlanStatus = "skipped"    // application exists (skipped)
	StatusDeferred   PlanStatus = "deferred"   // snoozed, resurfaces at deferred_until
	StatusOverridden PlanStatus = "overridden" // suppressed by an animal plan (§4.7)
)

// ApplicationView is the read-model projection of a care_plan_applications
// row (§4.5) the engine joins occurrences with.
type ApplicationView struct {
	Status        string // applied | skipped | deferred (§4.5)
	AppliedAt     time.Time
	DeferredUntil *time.Time // set only when Status=deferred (§10-A2/CP4)
	// FulfillmentDeleted (§10-CP1): the linked care/treatment row was
	// deleted; the application stays for audit and renders "record
	// deleted". Kept as a flag — status remains applied.
	FulfillmentDeleted bool
}

// PlanItem is one row of the day plan / animal Plan tab: an occurrence
// joined with its computed status, apply-window and override explanation.
type PlanItem struct {
	Occurrence   Occurrence
	Status       PlanStatus
	Applicable   bool   // §10-A1 apply window (🔒 hors délai when false)
	OverriddenBy string // animal-plan name when Status == overridden
	Application  *ApplicationView
}

// ComputeStatus computes the §6.1 status of one occurrence at `now`,
// ignoring overrides (the caller resolves those first). Windows come from
// the source's schedule: grace_minutes, miss_after_hours, lookahead_minutes.
//
// Application precedence: applied/skipped are terminal; an active defer
// (now < deferred_until) snoozes; an expired defer is ignored and the
// status recomputes from due_at as if no application existed (§10-H1).
func ComputeStatus(o Occurrence, app *ApplicationView, now time.Time) PlanStatus {
	if app != nil {
		switch app.Status {
		case "applied":
			return StatusApplied
		case "skipped":
			return StatusSkipped
		case "deferred":
			if app.DeferredUntil != nil && now.Before(*app.DeferredUntil) {
				return StatusDeferred
			}
			// Expired defer (§10-H1): fall through and recompute.
		}
	}
	grace, miss, lookahead := scheduleWindows(o.Source)
	since := now.Sub(o.DueAt)
	if since > miss {
		return StatusMissing
	}
	if since > grace {
		return StatusLate
	}
	if o.DueAt.Sub(now) <= lookahead {
		return StatusDue
	}
	return StatusScheduled
}

// scheduleWindows returns the occurrence's grace/miss/lookahead durations
// with spec defaults (§4.3) for a missing source.
func scheduleWindows(src PlanSource) (grace, miss, lookahead time.Duration) {
	if src == nil {
		return 60 * time.Minute, 24 * time.Hour, 60 * time.Minute
	}
	s := src.Schedule()
	return time.Duration(s.GraceMinutes) * time.Minute,
		time.Duration(s.MissAfterHours) * time.Hour,
		time.Duration(s.LookaheadMinutes) * time.Minute
}

// OccurrenceKey renders the applications UNIQUE key (§4.5):
// (source_type, source_id, animal_id, due_at).
func OccurrenceKey(o Occurrence) string {
	typ, id := "", ""
	if o.Source != nil {
		typ, id = string(o.Source.SourceType()), o.Source.SourceID()
	}
	return fmt.Sprintf("%s|%s|%d|%d", typ, id, o.AnimalID, o.DueAt.UnixNano())
}

// BuildPlanItems joins occurrences with applications and `now` into the
// day-plan read model (§6.1 steps 2–4): overrides resolved first, then
// per-item status, then the apply window. Items are sorted chronologically
// (ties: animal, then rule-before-plan, then source id) for deterministic
// rendering.
func BuildPlanItems(occs []Occurrence, apps map[string]*ApplicationView, now, windowEnd time.Time) []PlanItem {
	items := ResolveOverrides(occs)
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i].Occurrence, items[j].Occurrence
		if !a.DueAt.Equal(b.DueAt) {
			return a.DueAt.Before(b.DueAt)
		}
		if a.AnimalID != b.AnimalID {
			return a.AnimalID < b.AnimalID
		}
		ta, tb := sourceSortRank(a.Source), sourceSortRank(b.Source)
		if ta != tb {
			return ta < tb
		}
		return sourceIDOf(a.Source) < sourceIDOf(b.Source)
	})

	// nextDue[i]: the next occurrence of the same source for the same
	// animal (§10-A1), or nil when i is the last one in the window.
	type groupKey struct {
		typ    SourceType
		id     string
		animal int
	}
	nextDue := make([]*time.Time, len(items))
	pending := make(map[groupKey]time.Time, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		src := items[i].Occurrence.Source
		if src == nil {
			continue
		}
		k := groupKey{src.SourceType(), src.SourceID(), items[i].Occurrence.AnimalID}
		if nd, ok := pending[k]; ok {
			d := nd
			nextDue[i] = &d
		}
		pending[k] = items[i].Occurrence.DueAt
	}

	for i := range items {
		it := &items[i]
		if it.Occurrence.Source == nil {
			continue
		}
		if apps != nil {
			it.Application = apps[OccurrenceKey(it.Occurrence)]
		}
		// §6.1 order: applications are joined AFTER overrides, so a
		// historical application keeps its truthful status (the override
		// note stays as explanation).
		if it.Status != StatusOverridden || it.Application != nil {
			it.Status = ComputeStatus(it.Occurrence, it.Application, now)
		}
		it.Applicable = itemApplicable(it, nextDue[i], now, windowEnd)
	}
	return items
}

// itemApplicable implements the apply window (§10-A1): applicable until
// the next occurrence of the same source becomes due; the window's last
// occurrence stays applicable while the display window lasts. Terminal and
// suppressed states are never actionable; past-window items render
// read-only with the 🔒 hors délai badge (§10-H2).
func itemApplicable(it *PlanItem, nextDue *time.Time, now, windowEnd time.Time) bool {
	switch it.Status {
	case StatusOverridden, StatusApplied, StatusSkipped, StatusDeferred:
		return false
	}
	if nextDue != nil {
		return now.Before(*nextDue)
	}
	return !now.After(windowEnd)
}

func sourceSortRank(src PlanSource) int {
	if src != nil && src.SourceType() == SourceAnimal {
		return 1
	}
	return 0
}

func sourceIDOf(src PlanSource) string {
	if src == nil {
		return ""
	}
	return src.SourceID()
}
