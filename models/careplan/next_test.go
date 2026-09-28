package careplan

import (
	"testing"
	"time"
)

// nextSrc is a minimal rule source for compact-view tests; schedule content
// is irrelevant — NextOpenPerGroup consumes already-computed PlanItems.
func nextSrc(id string) *Source {
	return &Source{Type: SourceRule, ID: id, NameStr: "Rule " + id, Kind: KindFeeding, IsActive: true}
}

func nextItem(src PlanSource, animalID int, due time.Time, st PlanStatus) PlanItem {
	return PlanItem{Occurrence: Occurrence{Source: src, AnimalID: animalID, DueAt: due}, Status: st}
}

func at(day, hour, min int) time.Time {
	return time.Date(2026, 9, day, hour, min, 0, 0, time.Local)
}

// Earliest-open pick: a group with several open occurrences surfaces the
// EARLIEST as the action row and counts the rest in Remaining.
func TestNextOpenEarliestPick(t *testing.T) {
	src := nextSrc("r1")
	items := []PlanItem{
		nextItem(src, 1, at(15, 19, 0), StatusScheduled),
		nextItem(src, 1, at(15, 8, 0), StatusLate), // earliest open
		nextItem(src, 1, at(15, 12, 30), StatusDue),
	}
	got := NextOpenPerGroup(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if !g.Item.Occurrence.DueAt.Equal(at(15, 8, 0)) {
		t.Fatalf("next = %v, want earliest open 08:00", g.Item.Occurrence.DueAt)
	}
	if g.Item.Status != StatusLate {
		t.Fatalf("next status = %q, want late", g.Item.Status)
	}
	if g.Remaining != 2 {
		t.Fatalf("Remaining = %d, want 2 (12:30 due + 19:00 scheduled)", g.Remaining)
	}
	if g.Done != 0 {
		t.Fatalf("Done = %d, want 0", g.Done)
	}
}

// All-terminal group: every occurrence applied/skipped/deferred → no row,
// but the Done count is still tracked internally (caller's tier counters
// read the full list; the compact view simply omits the group).
func TestNextOpenAllTerminal(t *testing.T) {
	src := nextSrc("r1")
	items := []PlanItem{
		nextItem(src, 1, at(15, 8, 0), StatusApplied),
		nextItem(src, 1, at(15, 12, 30), StatusSkipped),
		nextItem(src, 1, at(15, 19, 0), StatusDeferred),
	}
	got := NextOpenPerGroup(items)
	if len(got) != 0 {
		t.Fatalf("expected no rows for all-terminal group, got %d", len(got))
	}
}

// Mixed group: terminal occurrences count toward Done, not Remaining.
func TestNextOpenMixedCounts(t *testing.T) {
	src := nextSrc("r1")
	items := []PlanItem{
		nextItem(src, 1, at(15, 8, 0), StatusApplied),  // terminal
		nextItem(src, 1, at(15, 12, 30), StatusDue),    // open (earliest)
		nextItem(src, 1, at(15, 19, 0), StatusSkipped), // terminal
	}
	got := NextOpenPerGroup(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if g.Remaining != 0 {
		t.Fatalf("Remaining = %d, want 0 (only one open occurrence)", g.Remaining)
	}
	if g.Done != 2 {
		t.Fatalf("Done = %d, want 2 (applied + skipped)", g.Done)
	}
}

// Overridden exclusion: overridden occurrences never surface as the action
// row and are not counted in Remaining either (§4.7 — they stay visible on
// the animal Plan tab, not the work screen).
func TestNextOpenOverriddenExcluded(t *testing.T) {
	src := nextSrc("r1")
	items := []PlanItem{
		nextItem(src, 1, at(15, 8, 0), StatusOverridden),
		nextItem(src, 1, at(15, 12, 30), StatusOverridden),
		nextItem(src, 1, at(15, 19, 0), StatusDue),
	}
	got := NextOpenPerGroup(items)
	if len(got) != 1 {
		t.Fatalf("expected 1 group, got %d", len(got))
	}
	g := got[0]
	if !g.Item.Occurrence.DueAt.Equal(at(15, 19, 0)) {
		t.Fatalf("next = %v, want 19:00 (overridden occurrences excluded)", g.Item.Occurrence.DueAt)
	}
	if g.Remaining != 0 {
		t.Fatalf("Remaining = %d, want 0 (overridden not counted)", g.Remaining)
	}
	if g.Done != 0 {
		t.Fatalf("Done = %d, want 0 (overridden is neither open nor terminal)", g.Done)
	}
}

// All-overridden group disappears entirely.
func TestNextOpenAllOverridden(t *testing.T) {
	src := nextSrc("r1")
	items := []PlanItem{
		nextItem(src, 1, at(15, 8, 0), StatusOverridden),
		nextItem(src, 1, at(15, 12, 30), StatusOverridden),
	}
	if got := NextOpenPerGroup(items); len(got) != 0 {
		t.Fatalf("expected no rows for all-overridden group, got %d", len(got))
	}
}

// Urgency ordering: missing < late < due < scheduled, whatever the input
// order; equal ranks fall back to DueAt, then animal id, then source id.
func TestNextOpenUrgencySort(t *testing.T) {
	srcA := nextSrc("r1")
	srcB := nextSrc("r2")
	items := []PlanItem{
		nextItem(srcA, 1, at(15, 19, 0), StatusScheduled),
		nextItem(srcA, 2, at(15, 8, 0), StatusDue),
		nextItem(srcB, 3, at(15, 9, 0), StatusMissing),
		nextItem(srcA, 4, at(15, 10, 0), StatusLate),
	}
	got := NextOpenPerGroup(items)
	if len(got) != 4 {
		t.Fatalf("expected 4 groups, got %d", len(got))
	}
	wantStatus := []PlanStatus{StatusMissing, StatusLate, StatusDue, StatusScheduled}
	wantAnimal := []int{3, 4, 2, 1}
	for i := range wantStatus {
		if got[i].Item.Status != wantStatus[i] || got[i].Item.Occurrence.AnimalID != wantAnimal[i] {
			t.Fatalf("row %d = (%q, animal %d), want (%q, animal %d)",
				i, got[i].Item.Status, got[i].Item.Occurrence.AnimalID, wantStatus[i], wantAnimal[i])
		}
	}
}

// Same-urgency tie-break: DueAt ascending, then animal id, then source id.
func TestNextOpenTieBreak(t *testing.T) {
	srcA := nextSrc("r1")
	srcB := nextSrc("r2")
	same := at(15, 8, 0)
	items := []PlanItem{
		nextItem(srcB, 2, same, StatusDue),   // same due, animal 2, source r2
		nextItem(srcA, 2, same, StatusDue),   // same due, animal 2, source r1 → first
		nextItem(srcA, 1, at(15, 9, 0), StatusDue), // later due → last
	}
	got := NextOpenPerGroup(items)
	if len(got) != 3 {
		t.Fatalf("expected 3 groups, got %d", len(got))
	}
	if got[0].Item.Occurrence.Source.SourceID() != "r1" || got[0].Item.Occurrence.AnimalID != 2 {
		t.Fatalf("row 0 = (src %s, animal %d), want (r1, 2) — source id tie-break",
			got[0].Item.Occurrence.Source.SourceID(), got[0].Item.Occurrence.AnimalID)
	}
	if got[1].Item.Occurrence.Source.SourceID() != "r2" {
		t.Fatalf("row 1 src = %s, want r2", got[1].Item.Occurrence.Source.SourceID())
	}
	if got[2].Item.Occurrence.AnimalID != 1 {
		t.Fatalf("row 2 animal = %d, want 1 (later DueAt)", got[2].Item.Occurrence.AnimalID)
	}
}

// Determinism: shuffling the input yields byte-identical output order.
func TestNextOpenDeterminism(t *testing.T) {
	src := nextSrc("r1")
	base := []PlanItem{
		nextItem(src, 1, at(15, 8, 0), StatusLate),
		nextItem(src, 1, at(15, 12, 30), StatusDue),
		nextItem(src, 2, at(15, 8, 0), StatusMissing),
		nextItem(src, 3, at(15, 8, 0), StatusScheduled),
		nextItem(src, 2, at(15, 19, 0), StatusScheduled),
	}
	rev := make([]PlanItem, len(base))
	for i, it := range base {
		rev[len(base)-1-i] = it
	}
	a, b := NextOpenPerGroup(base), NextOpenPerGroup(rev)
	if len(a) != len(b) {
		t.Fatalf("length mismatch: %d vs %d", len(a), len(b))
	}
	for i := range a {
		oa, ob := a[i].Item.Occurrence, b[i].Item.Occurrence
		if oa.AnimalID != ob.AnimalID || !oa.DueAt.Equal(ob.DueAt) || a[i].Item.Status != b[i].Item.Status ||
			a[i].Remaining != b[i].Remaining || a[i].Done != b[i].Done {
			t.Fatalf("row %d differs between input orders: %+v vs %+v", i, a[i], b[i])
		}
	}
}

// Nil-source items (defensive) are skipped without panic.
func TestNextOpenNilSource(t *testing.T) {
	items := []PlanItem{{Occurrence: Occurrence{Source: nil, AnimalID: 1, DueAt: at(15, 8, 0)}, Status: StatusDue}}
	if got := NextOpenPerGroup(items); len(got) != 0 {
		t.Fatalf("expected nil-source item skipped, got %d rows", len(got))
	}
}

// WorkWindow: yesterday 00:00 → today 23:59:59.999… (bugs.md U4).
func TestWorkWindow(t *testing.T) {
	now := time.Date(2026, 9, 15, 14, 30, 0, 0, time.Local)
	from, to := WorkWindow(now)
	wantFrom := time.Date(2026, 9, 14, 0, 0, 0, 0, time.Local)
	wantTo := time.Date(2026, 9, 15, 23, 59, 59, int(time.Second-time.Nanosecond), time.Local)
	if !from.Equal(wantFrom) {
		t.Fatalf("from = %v, want %v", from, wantFrom)
	}
	if !to.Equal(wantTo) {
		t.Fatalf("to = %v, want %v", to, wantTo)
	}
}

// BadgeCap: three-digit counts collapse to "99+" (bugs.md U2).
func TestBadgeCap(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", 9: "9", 99: "99", 100: "99+", 250: "99+", -3: "0"}
	for n, want := range cases {
		if got := BadgeCap(n); got != want {
			t.Fatalf("BadgeCap(%d) = %q, want %q", n, got, want)
		}
	}
}
