package actions

import (
	"io"
	"net/http"
	"strings"
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
// nav badge must equal when it was clicked (CP4 invariant).
func openTotal(v *DayPlanView) int {
	n := 0
	for ti := 0; ti < TierDoneIdx; ti++ {
		n += len(v.Tiers[ti].Cards)
	}
	n += len(v.Feedings) + len(v.Cares)
	for _, m := range v.Meds {
		if m.OpenCount > 0 {
			n++
		}
	}
	return n
}

// TestFilterStatsMatchVisibleSet: for EVERY view × zone × kind
// combination the summary strip (Stats), the tier sections and the
// rendered cards agree with an independent projection of the unfiltered
// reference view — impossible states like "0 future (17)" cannot be
// built, and every zone/kind badge equals what the screen shows after
// clicking it (CP4/D6).
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
				ctx := "view=%s zone=%q kind=%q"

				// every tier card satisfies the active filter
				for ti := range v.Tiers {
					exp := 0
					for _, cv := range ref.Tiers[ti].Cards {
						if zoneMatch(cv, zone) && kindMatch(cv, kind) {
							exp++
						}
					}
					require.Len(t, v.Tiers[ti].Cards, exp, ctx+" tier "+v.Tiers[ti].Key, view, zone, kind)
					for _, cv := range v.Tiers[ti].Cards {
						require.True(t, zoneMatch(cv, zone), ctx+" card zone", view, zone, kind)
						require.True(t, kindMatch(cv, kind), ctx+" card kind", view, zone, kind)
					}
				}

				// summary strip counts exactly the rendered tiers
				require.Equal(t, len(v.Tiers[0].Cards), v.Stats.Late, ctx, view, zone, kind)
				require.Equal(t, len(v.Tiers[1].Cards), v.Stats.Now, ctx, view, zone, kind)
				require.Equal(t, len(v.Tiers[2].Cards), v.Stats.Later, ctx, view, zone, kind)
				require.Equal(t, len(v.Tiers[3].Cards), v.Stats.Done, ctx, view, zone, kind)

				// sections respect zone × kind
				expFeeds := 0
				if kind == "" || kind == careplan.KindFeeding {
					for _, f := range ref.Feedings {
						if inZone(zone, f.Zone) {
							expFeeds++
						}
					}
				}
				require.Len(t, v.Feedings, expFeeds, ctx+" feedings", view, zone, kind)
				expCares := 0
				if kind == "" || kind == careplan.KindCleanup {
					for _, cv := range ref.Cares {
						if inZone(zone, cv.Zone) {
							expCares++
						}
					}
				}
				require.Len(t, v.Cares, expCares, ctx+" cares", view, zone, kind)
				expMeds := 0
				if kind == "" || kind == careplan.KindMedication {
					for _, m := range ref.Meds {
						if inZone(zone, m.Zone) {
							expMeds++
						}
					}
				}
				require.Len(t, v.Meds, expMeds, ctx+" meds", view, zone, kind)

				// hour chips reference VISIBLE later cards only
				byHour := map[string]int{}
				for _, cv := range v.Tiers[2].Cards {
					byHour[cv.HourKey]++
				}
				require.Len(t, v.Hours, len(byHour), ctx+" hour chips", view, zone, kind)
				for _, h := range v.Hours {
					require.Equal(t, byHour[strings.TrimPrefix(h.Anchor, "h-")], h.Count,
						ctx+" hour chip "+h.Anchor, view, zone, kind)
				}

				// CP4: each zone tab badge equals the open cards rendered
				// after clicking it; each kind chip likewise.
				sumZ := 0
				for _, zt := range v.Zones {
					fv := BuildDayPlanView(plan, view, zt.Name, kind, now)
					require.Equal(t, zt.Count, openTotal(fv), ctx+" zone tab "+zt.Name, view, zone, kind)
					sumZ += zt.Count
				}
				require.Equal(t, v.ZoneAll, sumZ, ctx+" zone total", view, zone, kind)
				sumK := 0
				for _, kc := range v.Kinds {
					fv := BuildDayPlanView(plan, view, zone, kc.Kind, now)
					require.Equal(t, kc.Count, openTotal(fv), ctx+" kind chip "+kc.Kind, view, zone, kind)
					sumK += kc.Count
				}
				require.Equal(t, v.KindAll, sumK, ctx+" kind total", view, zone, kind)
			}
		}
	}
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

	// feedings: same cards, same chips, same batch payloads
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

	// cares: same cage cards, same batch payloads
	require.Len(t, d.Cares, len(c.Cares))
	for i := range c.Cares {
		cc, dc := c.Cares[i], d.Cares[i]
		require.Equal(t, cc.Zone, dc.Zone)
		require.Equal(t, cc.Cage, dc.Cage)
		require.Equal(t, cc.SourceName, dc.SourceName)
		require.Equal(t, cc.Count, dc.Count)
		require.Equal(t, cc.ChipRefsJSON, dc.ChipRefsJSON)
	}

	// medication: same per-animal cards, same slots, same open counts
	require.Len(t, d.Meds, len(c.Meds))
	for i := range c.Meds {
		cm, dm := c.Meds[i], d.Meds[i]
		require.Equal(t, cm.AnimalID, dm.AnimalID)
		require.Equal(t, cm.OpenCount, dm.OpenCount)
		require.Len(t, dm.Slots, len(cm.Slots))
		for j := range cm.Slots {
			require.Equal(t, cm.Slots[j].DueAt, dm.Slots[j].DueAt)
			require.Equal(t, cm.Slots[j].Status, dm.Slots[j].Status)
			require.Equal(t, cm.Slots[j].Overridden, dm.Slots[j].Overridden)
		}
	}

	// the nav matrix covers the same zones/kinds in both views. Counts
	// are per-view by design (CP4: a badge counts the cards ITS view
	// renders — compact groups vs detailed occurrences); the click-through
	// equality is proven in TestFilterStatsMatchVisibleSet.
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

	// density difference: detailed surfaces the overridden occurrence in
	// the done tier (debug surface); compact never renders overridden.
	var dOvr, cOvr int
	for _, cv := range d.Tiers[TierDoneIdx].Cards {
		if cv.OverriddenBy != "" {
			dOvr++
		}
	}
	for _, cv := range c.Tiers[TierDoneIdx].Cards {
		if cv.OverriddenBy != "" {
			cOvr++
		}
	}
	require.Equal(t, 1, dOvr, "detailed shows the overridden row with its suppressing plan")
	require.Equal(t, 0, cOvr, "compact hides overridden occurrences")
}

// TestKindZoneCombinationFilters: the filter is a CONJUNCTION — kind=med
// × zone=Z1 shows only Z1 medication work; kind=feeding × zone=Z2 only
// the Z2 feeding card; the nav matrix narrows with the active sibling
// dimension (zone tabs under an active kind, kind chips under an active
// zone).
func TestKindZoneCombinationFilters(t *testing.T) {
	plan := pipelinePlan()
	now := time.Date(2026, 9, 28, 10, 30, 0, 0, time.Local)

	// medication × Z1: exactly animal 1's med card; every other section
	// and every tier card is filtered away
	v := BuildDayPlanView(plan, ViewCompact, "Z1", careplan.KindMedication, now)
	require.Len(t, v.Meds, 1)
	require.Equal(t, 1, v.Meds[0].AnimalID)
	require.Equal(t, 1, v.Meds[0].OpenCount)
	require.Empty(t, v.Feedings)
	require.Empty(t, v.Cares)
	for _, tier := range v.Tiers {
		require.Empty(t, tier.Cards, "weighing/care cards never survive a medication filter")
	}

	// feeding × Z2: exactly the Z2 feeding card
	v = BuildDayPlanView(plan, ViewCompact, "Z2", careplan.KindFeeding, now)
	require.Len(t, v.Feedings, 1)
	require.Equal(t, "Z2", v.Feedings[0].Zone)
	require.Equal(t, "C9", v.Feedings[0].Cage)
	require.Empty(t, v.Meds)
	require.Empty(t, v.Cares)
	for _, tier := range v.Tiers {
		require.Empty(t, tier.Cards)
	}

	// zone tabs under an active kind count only that kind's cards
	v = BuildDayPlanView(plan, ViewCompact, "", careplan.KindFeeding, now)
	require.Len(t, v.Zones, 2)
	byZone := map[string]int{}
	for _, zt := range v.Zones {
		byZone[zt.Name] = zt.Count
	}
	require.Equal(t, map[string]int{"Z1": 1, "Z2": 1}, byZone)

	// kind chips under an active zone count only that zone's cards
	v = BuildDayPlanView(plan, ViewCompact, "Z1", "", now)
	chips := map[string]int{}
	for _, kc := range v.Kinds {
		chips[kc.Kind] = kc.Count
	}
	require.Equal(t, map[string]int{
		careplan.KindFeeding:     1, // one feeding card
		careplan.KindMedication:  1, // one med card with open work
		careplan.KindCleanup:     1, // one cage cleanup card
		careplan.KindWeighing:    1, // one late weighing card
		careplan.KindCare:        0,
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

	// unknown zone → redirect + flash
	req, err := http.NewRequest("GET", baseURL+"/care_plan?zone=NOPE&view=compact", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	raw, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "body: %s", raw)
	require.Equal(t, "/care_plan?view=compact", resp.Header.Get("Location"))

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
