package careplan

import (
	"strings"
	"testing"
	"time"
)

// testRule builds a rule source with fixed daily slots 08:00 / 12:30 /
// 19:00 and the given windows (minutes/hours), §4.3 defaults otherwise.
func testRule(id, kind string, graceMin, missH, lookaheadMin int) *Source {
	return &Source{
		Type: SourceRule, ID: id, NameStr: "Règle " + id, Kind: kind,
		Sched: Schedule{
			Times: []TimeOfDay{{8, 0}, {12, 30}, {19, 0}}, EveryDays: 1, Anchor: AnchorIntake,
			GraceMinutes: graceMin, MissAfterHours: missH, LookaheadMinutes: lookaheadMin,
		},
		IsActive: true,
	}
}

func occAt(src PlanSource, animalID int, due time.Time) Occurrence {
	return Occurrence{Source: src, AnimalID: animalID, DueAt: due}
}

func statusInstant(s string) time.Time {
	d, err := time.ParseInLocation("2006-01-02 15:04:05", s, time.Local)
	if err != nil {
		panic("bad test instant: " + err.Error())
	}
	return d
}

// The §6.1 status table, temporal half: scheduled → due (lookahead) →
// late (grace) → missing (miss_after_hours).
func TestComputeStatusTemporal(t *testing.T) {
	src := testRule("r1", KindFeeding, 60, 24, 60)
	due := statusInstant("2026-09-15 08:00:00")
	o := occAt(src, 42, due)

	cases := []struct {
		now  string
		want PlanStatus
	}{
		{"2026-09-14 08:00:00", StatusScheduled}, // far future
		{"2026-09-15 06:59:59", StatusScheduled}, // just before lookahead
		{"2026-09-15 07:00:00", StatusDue},       // due − lookahead (inclusive)
		{"2026-09-15 08:00:00", StatusDue},       // at instant
		{"2026-09-15 09:00:00", StatusDue},       // at due+grace (not yet late)
		{"2026-09-15 09:00:01", StatusLate},      // strictly after due+grace
		{"2026-09-16 08:00:00", StatusLate},      // at due+miss (not yet missing)
		{"2026-09-16 08:00:01", StatusMissing},   // strictly after due+miss
		{"2026-10-01 08:00:00", StatusMissing},   // still displayed, §4.3
	}
	for _, c := range cases {
		if got := ComputeStatus(o, nil, statusInstant(c.now)); got != c.want {
			t.Errorf("now %s: status = %s, want %s", c.now, got, c.want)
		}
	}
}

// Applications beat temporal status; defer expiry recomputes (§10-H1).
func TestComputeStatusApplications(t *testing.T) {
	src := testRule("r1", KindFeeding, 60, 24, 60)
	due := statusInstant("2026-09-15 08:00:00")
	o := occAt(src, 42, due)
	longAfter := statusInstant("2026-09-20 08:00:00")

	if got := ComputeStatus(o, &ApplicationView{Status: "applied"}, longAfter); got != StatusApplied {
		t.Errorf("applied beats missing, got %s", got)
	}
	if got := ComputeStatus(o, &ApplicationView{Status: "skipped"}, longAfter); got != StatusSkipped {
		t.Errorf("skipped is terminal, got %s", got)
	}

	// Active defer (§10-A2): snoozed until deferred_until.
	deferredUntil := statusInstant("2026-09-15 10:00:00")
	app := &ApplicationView{Status: "deferred", DeferredUntil: &deferredUntil}
	if got := ComputeStatus(o, app, statusInstant("2026-09-15 09:30:00")); got != StatusDeferred {
		t.Errorf("active defer must be deferred, got %s", got)
	}

	// Defer expiry (§10-H1): at now ≥ deferred_until the defer row is
	// ignored and status recomputes from due_at (→ late: 10:00 > 09:00).
	if got := ComputeStatus(o, app, statusInstant("2026-09-15 10:00:00")); got != StatusLate {
		t.Errorf("expired defer must recompute to late, got %s", got)
	}
	// Recompute can also land on missing (now past due+miss).
	if got := ComputeStatus(o, app, statusInstant("2026-09-16 09:00:00")); got != StatusMissing {
		t.Errorf("expired defer past miss window must be missing, got %s", got)
	}

	// Deferred row without deferred_until is malformed → treated as expired.
	if got := ComputeStatus(o, &ApplicationView{Status: "deferred"}, longAfter); got != StatusMissing {
		t.Errorf("defer without deferred_until must recompute, got %s", got)
	}
}

// lookahead is per-schedule (§10-CP6a), not a global.
func TestComputeStatusLookaheadFromSchedule(t *testing.T) {
	src := testRule("r1", KindFeeding, 60, 24, 15) // 15 min lookahead
	due := statusInstant("2026-09-15 08:00:00")
	o := occAt(src, 42, due)
	if got := ComputeStatus(o, nil, statusInstant("2026-09-15 07:50:00")); got != StatusDue {
		t.Errorf("10 min before due with 15 min lookahead must be due, got %s", got)
	}
	if got := ComputeStatus(o, nil, statusInstant("2026-09-15 07:30:00")); got != StatusScheduled {
		t.Errorf("30 min before due with 15 min lookahead must be scheduled, got %s", got)
	}
}

// PlanItem carries the 8 §6.1 statuses.
func TestPlanStatusesComplete(t *testing.T) {
	want := []PlanStatus{StatusScheduled, StatusDue, StatusLate, StatusMissing,
		StatusApplied, StatusSkipped, StatusDeferred, StatusOverridden}
	if len(want) != 8 {
		t.Fatalf("spec §6.1 defines exactly 8 statuses")
	}
	seen := map[PlanStatus]bool{}
	for _, s := range want {
		if s == "" || seen[s] {
			t.Errorf("status %q empty or duplicated", s)
		}
		seen[s] = true
	}
}

// Apply window (§10-A1): an occurrence stays applicable until the NEXT
// occurrence of the same source becomes due; the window's last occurrence
// stays applicable while the display window lasts; past-window items are
// read-only (§10-H2).
func TestBuildPlanItemsApplyWindow(t *testing.T) {
	src := testRule("r1", KindFeeding, 60, 24, 60)
	a := &AnimalContext{ID: 42, IntakeDate: statusInstant("2026-09-14 06:00:00")}
	ws, we := statusInstant("2026-09-15 00:00:00"), statusInstant("2026-09-15 23:59:00")
	occs := GenerateOccurrences(src, a, ws, we)
	if len(occs) != 3 {
		t.Fatalf("want 3 occurrences, got %d", len(occs))
	}

	byDue := func(items []PlanItem) map[string]PlanItem {
		m := map[string]PlanItem{}
		for _, it := range items {
			m[it.Occurrence.DueAt.Format("15:04")] = it
		}
		return m
	}

	// 09:30 — the 08:00 slot is late (grace 60 expired at 09:00) but its
	// successor is not due yet (12:30 − lookahead 60 = 11:30).
	items := byDue(BuildPlanItems(occs, nil, statusInstant("2026-09-15 09:30:00"), we))
	if it := items["08:00"]; it.Status != StatusLate || !it.Applicable {
		t.Errorf("08:00 at 09:00 = %s applicable=%v, want late+applicable", it.Status, it.Applicable)
	}
	if it := items["12:30"]; it.Status != StatusScheduled || !it.Applicable {
		t.Errorf("12:30 at 09:00 = %s applicable=%v, want scheduled+applicable", it.Status, it.Applicable)
	}

	// 13:31 — the 12:30 slot went late (grace expired at 13:30); the 08:00
	// slot's apply window closed when its successor became due (§10-A1).
	items = byDue(BuildPlanItems(occs, nil, statusInstant("2026-09-15 13:31:00"), we))
	if it := items["08:00"]; it.Applicable {
		t.Errorf("08:00 at 13:31 must be read-only (§10-A1), got applicable")
	}
	if it := items["08:00"]; it.Status != StatusLate {
		t.Errorf("read-only item keeps its computed status, got %s", it.Status)
	}
	if it := items["12:30"]; it.Status != StatusLate || !it.Applicable {
		t.Errorf("12:30 at 13:31 = %s applicable=%v, want late+applicable", it.Status, it.Applicable)
	}

	// 20:00 — last slot of the window: still applicable while displayed.
	items = byDue(BuildPlanItems(occs, nil, statusInstant("2026-09-15 20:00:00"), we))
	if it := items["19:00"]; it.Status != StatusDue || !it.Applicable {
		t.Errorf("19:00 at 20:00 = %s applicable=%v, want due+applicable", it.Status, it.Applicable)
	}

	// Now past windowEnd: every item is history (§10-H2 🔒 hors délai).
	items = byDue(BuildPlanItems(occs, nil, statusInstant("2026-09-16 10:00:00"), we))
	for _, k := range []string{"08:00", "12:30", "19:00"} {
		if it := items[k]; it.Applicable {
			t.Errorf("%s past windowEnd must not be applicable", k)
		}
	}
}

// Full join (§6.1 steps 2–4): overrides + applications + statuses together.
func TestBuildPlanItemsEndToEnd(t *testing.T) {
	rule := testRule("r-feed", KindFeeding, 60, 24, 60)
	plan := testRule("ap-1", KindFeeding, 60, 24, 60)
	plan.Type, plan.NameStr, plan.Replaces = SourceAnimal, "Gavage 14h", true
	plan.Sched.Times = []TimeOfDay{{12, 0}} // distinct slot, no due collision
	other := testRule("r-med", KindMedication, 60, 24, 60)

	a := &AnimalContext{ID: 42, IntakeDate: statusInstant("2026-09-14 06:00:00")}
	ws := statusInstant("2026-09-15 00:00:00")
	we := statusInstant("2026-09-15 23:59:00")
	now := statusInstant("2026-09-15 13:00:00")

	occs := GenerateOccurrences(rule, a, ws, we)                  // 08:00, 12:30, 19:00
	occs = append(occs, GenerateOccurrences(plan, a, ws, we)...)  // 12:00 feeding plan
	occs = append(occs, GenerateOccurrences(other, a, ws, we)...) // medication rule survives

	appliedAt := now
	deferUntil := statusInstant("2026-09-15 14:00:00") // still snoozed at now=13:00
	items := BuildPlanItems(occs, map[string]*ApplicationView{
		OccurrenceKey(occAt(rule, 42, statusInstant("2026-09-15 08:00:00"))): {Status: "applied", AppliedAt: appliedAt},
		OccurrenceKey(occAt(rule, 42, statusInstant("2026-09-15 12:30:00"))): {Status: "deferred", DeferredUntil: &deferUntil},
	}, now, we)

	byDue := map[string]PlanItem{}
	for _, it := range items {
		byDue[it.Occurrence.DueAt.Format("15:04")+"-"+it.Occurrence.Source.ActionKind()] = it
	}
	if it := byDue["08:00-feeding"]; it.Status != StatusApplied || it.Application == nil || !it.Application.AppliedAt.Equal(appliedAt) {
		t.Errorf("08:00 feeding = %+v, want applied with application", it)
	}
	if it := byDue["12:30-feeding"]; it.Status != StatusDeferred {
		t.Errorf("12:30 feeding = %s, want deferred (until future)", it.Status)
	}
	// Both feeding rule slots suppressed by the replaces_kind plan…
	for _, k := range []string{"19:00-feeding"} {
		if it := byDue[k]; it.Status != StatusOverridden || !strings.Contains(it.OverriddenBy, "Gavage 14h") {
			t.Errorf("%s = %+v, want overridden by the animal plan", k, it)
		}
	}
	// …and the 12:30 slot is deferred AND overridden: application wins (§6.1 order).
	if it := byDue["12:30-feeding"]; it.Status != StatusDeferred || it.OverriddenBy == "" {
		t.Errorf("12:30 feeding = %+v, want deferred with override note", it)
	}
	// Different kind at the same hour: untouched (§4.7); 12:30 + grace 60
	// has not expired at 13:00, so it is still due.
	if it := byDue["12:30-medication"]; it.Status != StatusDue || it.OverriddenBy != "" {
		t.Errorf("12:30 medication = %+v, want due, not overridden", it)
	}
	// The plan's own occurrence is due (12:00 + 60 grace = 13:00, not yet
	// late) and applicable (its source has no later occurrence in window).
	if it := byDue["12:00-feeding"]; it.Status != StatusDue || !it.Applicable {
		t.Errorf("12:00 plan occurrence = %+v, want due+applicable", it)
	}
}

// OccurrenceKey mirrors the applications UNIQUE key (§4.5) and is unique
// per (source_type, source_id, animal_id, due_at).
func TestOccurrenceKey(t *testing.T) {
	r1 := testRule("r1", KindFeeding, 60, 24, 60)
	r2 := testRule("r2", KindFeeding, 60, 24, 60)
	due := statusInstant("2026-09-15 08:00:00")
	base := OccurrenceKey(occAt(r1, 42, due))
	if again := OccurrenceKey(occAt(r1, 42, due)); again != base {
		t.Errorf("key must be stable, got %q then %q", base, again)
	}
	for desc, o := range map[string]Occurrence{
		"other source": occAt(r2, 42, due),
		"other animal": occAt(r1, 43, due),
		"other due_at": occAt(r1, 42, due.Add(time.Minute)),
		"other type":   occAt(func() *Source { s := testRule("r1", KindFeeding, 60, 24, 60); s.Type = SourceAnimal; return s }(), 42, due),
	} {
		if OccurrenceKey(o) == base {
			t.Errorf("%s must change the key", desc)
		}
	}
}
