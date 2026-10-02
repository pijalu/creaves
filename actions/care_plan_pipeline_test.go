package actions

import (
	"fmt"
	"io"
	"net/http"

	"testing"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Round-2 §7.2 pipeline tests (docs/care-plan-ux-fix-plan-round2.md):
// CP2 (unfiltered engine → one zone×kind display pass), CP3/D3 (compact
// and detailed share one build path), CP4/D6 (nav badges == rendered
// cards) and the zone whitelist (unknown zone → flash + reset).

// pipelinePlan builds a plan touching every tier and every section:
//
//	Z1: weighing late (tier 0), feeding due, medication due + applied,
//	    cleanup due (care section), weighing applied (done) + overridden
//	Z2: care due (tier 1), feeding due, medication applied-only,
//	    weighing scheduled tomorrow (tier 2)
func pipelinePlan() *DayPlan {
	plan := testPlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)
	plan.Now = now

	weighL := testSource(careplan.KindWeighing, "w1", "Weigh L", nil)
	weighD := testSource(careplan.KindWeighing, "w3", "Weigh D", nil)
	weighS := testSource(careplan.KindWeighing, "w2", "Weigh S", nil)
	careSrc := testSource(careplan.KindCare, "c1", "Care C", map[string]interface{}{"note": "wound"})
	feed := testSource(careplan.KindFeeding, "f1", "Feed F", map[string]interface{}{"food": "crickets"})
	clean := testSource(careplan.KindCleanup, "cl1", "Clean C1", map[string]interface{}{"note": "disinfect"})
	med := testSource(careplan.KindMedication, "m1", "Med M", map[string]interface{}{"drug": "Citramox", "dosage": "0.5 ml"})

	day := func(h, m int) time.Time { return time.Date(2026, 9, 28, h, m, 0, 0, time.Local) }

	late := testItem(weighL, 1, careplan.StatusLate)
	late.Occurrence.DueAt = day(7, 0)
	done := testItem(weighD, 2, careplan.StatusApplied)
	done.Occurrence.DueAt = day(9, 0)
	sched := testItem(weighS, 3, careplan.StatusScheduled)
	sched.Occurrence.DueAt = day(8, 0).AddDate(0, 0, 1)
	dueCare := testItem(careSrc, 3, careplan.StatusDue)
	dueCare.Occurrence.DueAt = day(9, 30)
	ovr := testItem(weighD, 2, careplan.StatusOverridden)
	ovr.Occurrence.DueAt = day(10, 0)
	ovr.OverriddenBy = "manual-plan-x"

	medDue := testItem(med, 1, careplan.StatusDue)
	medDue.Occurrence.DueAt = day(8, 30)
	medDone := testItem(med, 1, careplan.StatusApplied)
	medDone.Occurrence.DueAt = day(7, 30)
	medAppliedOnly := testItem(med, 3, careplan.StatusApplied)
	medAppliedOnly.Occurrence.DueAt = day(9, 0)

	feed1 := testItem(feed, 1, careplan.StatusDue)
	feed1.Occurrence.DueAt = day(11, 0)
	feed3 := testItem(feed, 3, careplan.StatusDue)
	feed3.Occurrence.DueAt = day(11, 30)

	clean1 := testItem(clean, 1, careplan.StatusDue)
	clean2 := testItem(clean, 2, careplan.StatusDue)

	plan.Items = []careplan.PlanItem{
		late, done, sched, dueCare, ovr,
		medDue, medDone, medAppliedOnly,
		feed1, feed3, clean1, clean2,
	}
	return plan
}

// openTotal counts the OPEN cards a rendered view shows — the number a
// nav badge must equal when it was clicked (CP4 invariant): the three
// urgency tiers (rows — medication, care, weighing, observation) plus
// the grouped feeding/cleanup lists. History rows are never counted:
// they are past work, not workload.
func openTotal(v *DayPlanView) int {
	n := 0
	for _, tier := range v.Tiers {
		n += len(tier.Cards)
	}
	return n + len(v.Feedings) + len(v.Cares)
}

// expectedStats recomputes the occurrence summary INDEPENDENTLY of the
// viewmodel (§12.1): open current occurrences after the zone×kind filter,
// bucketed by urgency. Section kinds count per occurrence — every counted
// occurrence shows as its own button/chip/row or folds into a "+N" badge.
func expectedStats(plan *DayPlan, zone, kind string, now time.Time) FilterStats {
	var buckets [3]int
	for i := range plan.Items {
		it := &plan.Items[i]
		if !statsCandidate(plan, it, zone, kind, now) {
			continue
		}
		buckets[tierOrder(it.Status)]++
	}
	st := FilterStats{Late: buckets[0], Now: buckets[1], Later: buckets[2]}
	st.LateCap = BadgeCap(st.Late)
	st.NowCap = BadgeCap(st.Now)
	st.LaterCap = BadgeCap(st.Later)
	return st
}

// statsCandidate mirrors statsOf's inclusion test — kept SEPARATE from the
// viewmodel on purpose: the test recomputes the bucketing independently.
func statsCandidate(plan *DayPlan, it *careplan.PlanItem, zone, kind string, now time.Time) bool {
	src := it.Occurrence.Source
	if src == nil || it.Status == careplan.StatusOverridden {
		return false
	}
	if kind != "" && src.ActionKind() != kind {
		return false
	}
	a, ok := plan.AnimalRow(it.Occurrence.AnimalID)
	if !ok || (zone != "" && a.Zone.String != zone) {
		return false
	}
	return openStatusAction(it.Status) && IsCurrent(it, now)
}

// TestFilterStatsMatchVisibleSet: for EVERY view × zone × kind
// combination the summary strip (Stats) matches an independent
// occurrence-level projection, every section card satisfies the active
// filter, and the nav badges equal what the screen shows after clicking
// them (CP4/D6) — impossible states like "0 future (17)" cannot be built.
func TestFilterStatsMatchVisibleSet(t *testing.T) {
	plan := pipelinePlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	zones := []string{"", "Z1", "Z2", "Z9"} // Z9 exists as a filter value but has no cards
	kinds := append([]string{""}, actionKinds...)

	for _, view := range []string{ViewCompact, ViewDetailed} {
		ref := BuildDayPlanView(plan, view, "", "", now)
		for _, zone := range zones {
			for _, kind := range kinds {
				v := BuildDayPlanView(plan, view, zone, kind, now)
				ctx := fmt.Sprintf("view=%s zone=%q kind=%q", view, zone, kind)
				assertRowsFiltered(t, v, zone, kind, ctx)
				require.Equal(t, expectedStats(plan, zone, kind, now), v.Stats, ctx+" summary strip vs independent count")
				assertSectionsFiltered(t, ref, v, zone, kind, ctx)
				assertNavBadges(t, plan, v, view, zone, kind, now, ctx)
			}
		}
	}
}

// assertRowsFiltered: every tier row and history row satisfies the
// active zone × kind filter — medication rows included (fix 6: medication
// renders INSIDE the tiers, never as a separate collapsible section).
func assertRowsFiltered(t *testing.T, v *DayPlanView, zone, kind string, ctx string) {
	t.Helper()
	for ti := range v.Tiers {
		for _, cv := range v.Tiers[ti].Cards {
			require.True(t, zoneMatch(cv, zone), ctx+" tier row zone")
			require.True(t, kindMatch(cv, kind), ctx+" tier row kind")
		}
	}
	for _, cv := range v.History {
		require.True(t, zoneMatch(cv, zone), ctx+" history row zone")
		require.True(t, kindMatch(cv, kind), ctx+" history row kind")
	}
}

// sectionZones lists the zones of ONE section's cards (unfiltered view).
func sectionZones(ref *DayPlanView, sectionKind string) []string {
	var zones []string
	switch sectionKind {
	case careplan.KindFeeding:
		for _, f := range ref.Feedings {
			zones = append(zones, f.Zone)
		}
	case careplan.KindCleanup:
		for _, cv := range ref.Cares {
			zones = append(zones, cv.Zone)
		}
	}
	return zones
}

// sectionCountInZone counts the cards of ONE section kind the active
// zone × kind filter keeps, from the unfiltered reference view.
func sectionCountInZone(ref *DayPlanView, zone, kind, sectionKind string) int {
	if kind != "" && kind != sectionKind {
		return 0
	}
	n := 0
	for _, z := range sectionZones(ref, sectionKind) {
		if inZone(zone, z) {
			n++
		}
	}
	return n
}

// assertSectionsFiltered: each grouped section shows exactly the cards
// the active filter keeps (unfiltered reference × zone × kind).
// Medication has no grouped section — it is tier rows, filtered and
// asserted by assertRowsFiltered.
func assertSectionsFiltered(t *testing.T, ref, v *DayPlanView, zone, kind string, ctx string) {
	t.Helper()
	require.Len(t, v.Feedings, sectionCountInZone(ref, zone, kind, careplan.KindFeeding), ctx+" feedings")
	require.Len(t, v.Cares, sectionCountInZone(ref, zone, kind, careplan.KindCleanup), ctx+" cares")
}

// assertNavBadges (CP4/D6): each zone tab badge equals the open cards
// rendered after clicking it; each kind chip likewise. The kind chips
// PARTITION the active zone's workload (there is no "all/TOUT" chip —
// fix 5), so their counts sum to the zone's open total.
func assertNavBadges(t *testing.T, plan *DayPlan, v *DayPlanView, view, zone, kind string, now time.Time, ctx string) {
	t.Helper()
	sumZ := 0
	for _, zt := range v.Zones {
		fv := BuildDayPlanView(plan, view, zt.Name, kind, now)
		require.Equal(t, zt.Count, openTotal(fv), ctx+" zone tab "+zt.Name)
		sumZ += zt.Count
	}
	require.Equal(t, v.ZoneAll, sumZ, ctx+" zone total")
	sumK := 0
	for _, kc := range v.Kinds {
		fv := BuildDayPlanView(plan, view, zone, kc.Kind, now)
		require.Equal(t, kc.Count, openTotal(fv), ctx+" kind chip "+kc.Kind)
		sumK += kc.Count
	}
	require.Equal(t, openTotal(BuildDayPlanView(plan, view, zone, "", now)), sumK,
		ctx+" kind chips partition the active zone's workload")
}

// TestCompactAndDetailedSameGroups (CP3/D3): one build path — compact and
// detailed render the SAME sections/groups (feedings, cares, medication
// cards, nav matrix); only the tier density differs (detailed adds
// per-occurrence cards and surfaces overridden rows).
func TestCompactAndDetailedSameGroups(t *testing.T) {
	plan := pipelinePlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	c := BuildDayPlanView(plan, ViewCompact, "", "", now)
	d := BuildDayPlanView(plan, ViewDetailed, "", "", now)
	require.False(t, c.Detailed)
	require.True(t, d.Detailed)

	assertSameFeedingGroups(t, c, d)
	assertSameCareGroups(t, c, d)
	assertSameNavCoverage(t, c, d)
	assertSameTierMembership(t, c, d)

	// density difference: detailed surfaces the overridden occurrence in
	// the history section (debug surface); compact collapses the group to
	// its applied row and never renders overridden.
	var dOvr, cOvr int
	for _, cv := range d.History {
		if cv.OverriddenBy != "" {
			dOvr++
		}
	}
	for _, cv := range c.History {
		if cv.OverriddenBy != "" {
			cOvr++
		}
	}
	require.Equal(t, 1, dOvr, "detailed shows the overridden row with its suppressing plan")
	require.Equal(t, 0, cOvr, "compact hides overridden occurrences")
	// history rows are most-recent-first and carry undo affordance for
	// rows with an application record
	require.NotEmpty(t, d.History)
	for i := 1; i < len(d.History); i++ {
		require.False(t, d.History[i-1].DueAt.Before(d.History[i].DueAt), "history sorted most recent first")
	}
}

// assertSameFeedingGroups: same feeding cards, same chips, same batch
// payloads in both densities.
func assertSameFeedingGroups(t *testing.T, c, d *DayPlanView) {
	t.Helper()
	require.Len(t, d.Feedings, len(c.Feedings))
	for i := range c.Feedings {
		cf, df := c.Feedings[i], d.Feedings[i]
		require.Equal(t, cf.Zone, df.Zone)
		require.Equal(t, cf.Cage, df.Cage)
		require.Equal(t, cf.Food, df.Food)
		require.Equal(t, cf.ApplicableCount, df.ApplicableCount)
		require.Equal(t, cf.ChipRefsJSON, df.ChipRefsJSON)
		require.Len(t, df.Chips, len(cf.Chips))
	}
}

// assertSameCareGroups: same cage cards, same batch payloads.
func assertSameCareGroups(t *testing.T, c, d *DayPlanView) {
	t.Helper()
	require.Len(t, d.Cares, len(c.Cares))
	for i := range c.Cares {
		cc, dc := c.Cares[i], d.Cares[i]
		require.Equal(t, cc.Zone, dc.Zone)
		require.Equal(t, cc.Cage, dc.Cage)
		require.Equal(t, cc.SourceName, dc.SourceName)
		require.Equal(t, cc.Count, dc.Count)
		require.Equal(t, cc.ChipRefsJSON, dc.ChipRefsJSON)
	}
}

// assertSameTierMembership: medication/care/weighing/observation are TIER
// rows in both densities (fix 6 — no separate medication section).
// Compact folds each (source × animal) group to its next open action,
// detailed lists every open current occurrence — so compact has at most
// one card per group and detailed at least as many cards per tier.
func assertSameTierMembership(t *testing.T, c, d *DayPlanView) {
	t.Helper()
	for ti := range c.Tiers {
		require.LessOrEqual(t, len(c.Tiers[ti].Cards), len(d.Tiers[ti].Cards),
			"tier "+c.Tiers[ti].Key+": compact folds, detailed lists")
	}
	require.NotEmpty(t, d.Tiers[0].Cards, "medication late rows are tier rows, not a collapsible med section")
}
// assertSameNavCoverage: the nav matrix covers the same zones/kinds in
// both views. Counts are per-view by design (CP4: a badge counts the
// cards ITS view renders — compact groups vs detailed occurrences); the
// click-through equality is proven in TestFilterStatsMatchVisibleSet.
func assertSameNavCoverage(t *testing.T, c, d *DayPlanView) {
	t.Helper()
	zoneNames := func(v *DayPlanView) []string {
		out := make([]string, 0, len(v.Zones))
		for _, zt := range v.Zones {
			out = append(out, zt.Name)
		}
		return out
	}
	openKinds := func(v *DayPlanView) map[string]bool {
		out := map[string]bool{}
		for _, kc := range v.Kinds {
			if kc.Count > 0 {
				out[kc.Kind] = true
			}
		}
		return out
	}
	require.Equal(t, zoneNames(c), zoneNames(d))
	require.Equal(t, openKinds(c), openKinds(d))
}

// TestKindZoneCombinationFilters: the filter is a CONJUNCTION — kind=med
// × zone=Z1 shows only Z1 medication work; kind=feeding × zone=Z2 only
// the Z2 feeding card; the nav matrix narrows with the active sibling
// dimension (zone tabs under an active kind, kind chips under an active
// zone).
func TestKindZoneCombinationFilters(t *testing.T) {
	plan := pipelinePlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	// medication × Z1: exactly animal 1's med ROW in the "now" tier
	// (fix 6 — medication is tier rows, no med section); its applied
	// occurrence surfaces as a history row; no feeding/cleanup section.
	v := BuildDayPlanView(plan, ViewCompact, "Z1", careplan.KindMedication, now)
	require.Len(t, v.Tiers[1].Cards, 1)
	require.Equal(t, 1, v.Tiers[1].Cards[0].AnimalID)
	require.Empty(t, v.Tiers[0].Cards)
	require.Empty(t, v.Tiers[2].Cards)
	require.Empty(t, v.Feedings)
	require.Empty(t, v.Cares)
	require.Len(t, v.History, 1, "the applied Z1 med occurrence is history")
	require.Equal(t, 1, v.History[0].AnimalID)

	// feeding × Z2: exactly the Z2 feeding card; feeding never yields
	// tier rows nor history rows (grouped kinds stay on their section)
	v = BuildDayPlanView(plan, ViewCompact, "Z2", careplan.KindFeeding, now)
	require.Len(t, v.Feedings, 1)
	require.Equal(t, "Z2", v.Feedings[0].Zone)
	require.Equal(t, "C9", v.Feedings[0].Cage)
	for ti := range v.Tiers {
		require.Empty(t, v.Tiers[ti].Cards)
	}
	require.Empty(t, v.Cares)
	require.Empty(t, v.History)

	// zone tabs under an active kind count only that kind's cards
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	require.Len(t, v.Zones, 2)
	byZone := map[string]int{}
	for _, zt := range v.Zones {
		byZone[zt.Name] = zt.Count
	}
	require.Equal(t, map[string]int{"Z1": 1, "Z2": 1}, byZone)

	// kind chips under an active zone count only that zone's cards —
	// CARE and MEDICATION included (both are tier rows now)
	v = BuildDayPlanView(plan, ViewCompact, "Z1", "", now)
	chips := map[string]int{}
	for _, kc := range v.Kinds {
		chips[kc.Kind] = kc.Count
	}
	require.Equal(t, map[string]int{
		careplan.KindFeeding:     1, // one feeding card
		careplan.KindMedication:  1, // one med row with open work
		careplan.KindCleanup:     1, // one cage cleanup card
		careplan.KindWeighing:    1, // one late weighing row
		careplan.KindCare:        0, // the care row is Z2 (animal 3)
		careplan.KindObservation: 0,
	}, chips)

	// kind chips under NO zone count the whole day's workload — the care
	// row (animal 3) and the second weighing row (animal 3) show up here
	v = BuildDayPlanView(plan, ViewCompact, "", "", now)
	chips = map[string]int{}
	for _, kc := range v.Kinds {
		chips[kc.Kind] = kc.Count
	}
	require.Equal(t, map[string]int{
		careplan.KindFeeding:     2, // C1 + C9 feeding cards
		careplan.KindMedication:  1, // one folded med row (animal 1, "+1" later)
		careplan.KindCleanup:     1, // C1 cage card
		careplan.KindWeighing:    2, // late animal 1 + scheduled animal 3
		careplan.KindCare:        1, // the due care row (animal 3)
		careplan.KindObservation: 0,
	}, chips)
}

// TestUnknownZoneRedirects: an unknown ?zone= whitelist-fails → 302 back
// to the unfiltered self path with a warning flash (CP2: a stale link
// never silently hides every card); a known zone renders the filtered
// screen.
func TestUnknownZoneRedirects(t *testing.T) {
	f := setupPlanFixture(t)
	_ = f
	client, baseURL := planAdminClient(t)

	// unknown zone → redirect + flash (to the DEFAULT kind screen — fix 5:
	// the tabs have no "all" entry, so the reset target carries the kind)
	req, err := http.NewRequest("GET", baseURL+"/care_plan?zone=NOPE&view=compact", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "body: %s", raw)
	require.Equal(t, "/care_plan?kind=feeding&view=compact", resp.Header.Get("Location"))

	// the flash renders on the followed redirect (same session)
	resp2, err := client.Get(baseURL + resp.Header.Get("Location"))
	require.NoError(t, err)
	raw2, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()
	require.Equal(t, http.StatusOK, resp2.StatusCode)
	require.Contains(t, string(raw2), "Unknown zone — filter reset.")

	// a zone that exists renders the filtered screen (no redirect)
	z := &models.Zone{ID: uuid.Must(uuid.NewV4()), Zone: "CP2-PIPELINE-ZONE", Type: "care"}
	require.NoError(t, models.DB.Create(z))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM zones WHERE id = ?", z.ID).Exec()
	})
	resp3, err := client.Get(baseURL + "/care_plan?zone=CP2-PIPELINE-ZONE")
	require.NoError(t, err)
	raw3, _ := io.ReadAll(resp3.Body)
	resp3.Body.Close()
	require.Equal(t, http.StatusOK, resp3.StatusCode)
	require.NotContains(t, string(raw3), "Unknown zone — filter reset.")
}
