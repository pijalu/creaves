package actions

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	popmw "github.com/gobuffalo/buffalo-pop/v3/pop/popmw"
	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// Round 10 performance pins (docs/performance-assessment-2026-10-04.md).

// R10-P1: the landing badge used to re-run the FULL day-plan assembly
// (§6.1) on every request just for two advisory numbers. The cached
// variant must serve the cached pair within the TTL and recompute after
// InvalidateDayPlanBadgeCache — the exact contract the apply/unapply
// handlers rely on via queuePostCommitInvalidation.
func TestCountOpenItemsCachedSemantics(t *testing.T) {
	tx := models.DB
	InvalidateDayPlanBadgeCache()

	wantOpen, wantLate, err := CountOpenItems(tx, time.Now())
	require.NoError(t, err)

	// Poison the cache: a hit must serve the poisoned pair WITHOUT the DB.
	dayPlanBadgeCache.Lock()
	dayPlanBadgeCache.valid, dayPlanBadgeCache.open, dayPlanBadgeCache.late = true, 1234, 567
	dayPlanBadgeCache.at = time.Now()
	dayPlanBadgeCache.Unlock()

	gotOpen, gotLate := CountOpenItemsCached(tx, time.Now())
	require.Equal(t, 1234, gotOpen, "within TTL the cached pair is served as-is")
	require.Equal(t, 567, gotLate)

	// After invalidation the value is recomputed from the DB.
	InvalidateDayPlanBadgeCache()
	gotOpen, gotLate = CountOpenItemsCached(tx, time.Now())
	require.Equal(t, wantOpen, gotOpen)
	require.Equal(t, wantLate, gotLate)
}

// R10-P2: the former tx.Eager() on the today-treatments query made Pop
// load EVERY level-2 association PER PARENT ROW (2N queries for N
// treatments). The bulk path must attach Entries in one extra query and
// keep the read model (TodayStatitics) working.
func TestEnrichAnimalsOptimizedBulkLoadsEntries(t *testing.T) {
	f := setupPlanFixture(t)
	animalID := f.animalIDs[0]
	tx := models.DB

	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	tr := &models.Treatment{
		Date:       today,
		AnimalID:   animalID,
		Drug:       "PerfBulEntries drug",
		Dosage:     "1 ml",
		Timebitmap: models.Treatement_MORNING | models.Treatement_NOON,
	}
	require.NoError(t, tx.Create(tr))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM treatments WHERE id = ?", tr.ID).Exec() })

	// Entry hours are pinned in UTC: parseTime returns UTC datetimes, and
	// the 3-bucket classification reads DueAt.Hour() — the fixture must
	// say which clock it means (the app's convention) instead of relying
	// on the runner's local zone.
	utcToday := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
	done := &models.TreatmentTimeEntry{
		TreatmentID: tr.ID, AnimalID: animalID,
		DueAt: utcToday.Add(8 * time.Hour), TimeLabel: "08:00",
		Status: models.TreatmentEntryStatusDone,
	}
	done.AppliedAt = nulls.NewTime(utcToday.Add(8 * time.Hour))
	pending := &models.TreatmentTimeEntry{
		TreatmentID: tr.ID, AnimalID: animalID,
		DueAt: utcToday.Add(12 * time.Hour), TimeLabel: "12:00",
		Status: models.TreatmentEntryStatusPending,
	}
	for _, e := range []*models.TreatmentTimeEntry{done, pending} {
		require.NoError(t, tx.Create(e))
		id := e.ID
		t.Cleanup(func() { tx.RawQuery("DELETE FROM treatment_time_entries WHERE id = ?", id).Exec() })
	}

	// Run the enrichment inside a buffalo context: the probe app mounts
	// popmw.Transaction like the real app so c.Value("tx") is live.
	var got models.Animals
	var morning nulls.Bool
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(popmw.Transaction(models.DB))
	a.GET("/probe", func(c buffalo.Context) error {
		animals := models.Animals{}
		require.NoError(t, tx.Where("id in (?)", f.animalIDs).All(&animals))
		if _, err := EnrichAnimalsOptimized(&animals, c); err != nil {
			return err
		}
		got = animals
		for _, an := range animals {
			if an.ID == animalID && len(an.Treatments) > 0 {
				morning = an.Treatments.TodayStatitics().Morning
			}
		}
		return c.Render(http.StatusOK, nil)
	})
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)

	var target models.Animal
	for _, an := range got {
		if an.ID == animalID {
			target = an
		}
	}
	require.NotNil(t, target.Treatments, "today treatment must be loaded")
	require.Len(t, target.Treatments, 1)
	require.Len(t, target.Treatments[0].Entries, 2, "entries must be bulk-attached")
	// The read model derives its 3-bucket status from the entries.
	require.True(t, morning.Valid, "a bucket with entries reports a status")
	require.True(t, morning.Bool, "all-done morning bucket reports done")
}

// R10-P3: the animals index uses the NoTreatments enrichment (its template
// never renders Treatments) — the pin keeps the cheaper variant in place.
func TestAnimalsListUsesNoTreatmentsEnrichment(t *testing.T) {
	src := readTemplate(t, "../actions/animals.go")
	require.Contains(t, src, "EnrichAnimalsOptimizedNoTreatments(animals, c)",
		"the animals list must not pay the today-treatments query")
}

// R10-P4: the plan-assembly animal load must not use the OR form that
// full-scanned all ~10k animals on every request (two indexed loads now).
func TestPlanAnimalLoadHasNoORScan(t *testing.T) {
	src := readTemplate(t, "../actions/care_plan_service.go")
	require.NotContains(t, src, "outtake_id IS NULL OR outtake_id IN (SELECT id FROM outtakes",
		"the OR form defeats the outtake_id index — use the two-query split")
}
