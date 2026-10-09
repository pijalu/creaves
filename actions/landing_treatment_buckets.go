package actions

import (
	"creaves/models"
	"sync"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"

	"creaves/models/careplan"
)

// The landing "Treatments" column summarizes TODAY'S medication per
// morning/noon/evening bucket. Legacy treatments are covered by
// Treatments.TodayStatitics() over the rows the landing enrichment loads —
// but protocol-driven medication only creates its treatments ROW (and its
// per-time entries) on the day's first apply (writePlanApplication), so
// before that the column read "not required" all day for protocol animals
// even though the protocol expected its doses (2026-10-09 user report).
//
// landingTreatmentBuckets computes the protocol side: today's MEDICATION
// occurrences per animal, bucketed with the same §10-M1 mapping the legacy
// entries use (<11:00 morning, 11:00–15:59 noon, ≥16:00 evening) and the
// same all-done semantics (a bucket reads done only when ALL of its
// occurrences are applied). The landing action merges this with the legacy
// stat; skipped/deferred occurrences read as not done, overridden ones
// impose nothing (the exception suppresses the requirement).

const landingBucketTTL = 30 * time.Second

var landingBucketCache = struct {
	sync.Mutex
	valid bool
	at    time.Time
	stats map[int]models.TreatmentStatusStatistics
}{}

// InvalidateLandingBucketCache drops the cached per-animal bucket summary.
// It is wired into InvalidateDayPlanBadgeCache so the apply/unapply
// post-commit invalidation (the hook every protocol action already calls)
// covers both cached landing computations.
func InvalidateLandingBucketCache() {
	landingBucketCache.Lock()
	landingBucketCache.valid = false
	landingBucketCache.Unlock()
}

// LandingTreatmentBucketsCached is landingTreatmentBuckets behind the same
// short-TTL cache the landing badge uses (the landing is the most-hit page
// and the computation is a full plan assembly). A planning failure degrades
// to an empty map — the column falls back to legacy treatments only — and
// must not poison the cache.
func LandingTreatmentBucketsCached(tx *pop.Connection, now time.Time) map[int]models.TreatmentStatusStatistics {
	landingBucketCache.Lock()
	defer landingBucketCache.Unlock()
	if landingBucketCache.valid && now.Sub(landingBucketCache.at) < landingBucketTTL {
		return landingBucketCache.stats
	}
	stats, err := landingTreatmentBuckets(tx, now)
	if err != nil {
		return map[int]models.TreatmentStatusStatistics{}
	}
	landingBucketCache.valid, landingBucketCache.stats, landingBucketCache.at = true, stats, now
	return stats
}

func landingTreatmentBuckets(tx *pop.Connection, now time.Time) (map[int]models.TreatmentStatusStatistics, error) {
	from, to := DefaultPlanWindow(now)
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	todayEnd := todayStart.AddDate(0, 0, 1)

	pa, err := loadAnimalContextsIncludingTodayOuttaken(tx, now, nil)
	if err != nil {
		return nil, err
	}
	sources, planAnimal, err := loadPlanSources(tx)
	if err != nil {
		return nil, err
	}
	members, err := buildMemberships(tx, sources, pa, from)
	if err != nil {
		return nil, err
	}

	var occs []careplan.Occurrence
	for _, src := range sources {
		if src.ActionKind() != careplan.KindMedication {
			continue
		}
		for _, animalID := range sourceAnimals(src, planAnimal, members, pa) {
			occs = append(occs, careplan.GenerateOccurrences(src, pa.ctxs[animalID], from, to)...)
		}
	}
	apps, err := loadApplications(tx, from, to)
	if err != nil {
		return nil, err
	}

	// bucketAcc accumulates one (animal × bucket): set = at least one
	// today-occurrence landed here; done = every one of them applied.
	type bucketAcc struct {
		set  bool
		done bool
	}
	acc := map[int][3]bucketAcc{}
	for _, it := range careplan.BuildPlanItems(occs, apps, now, to) {
		if it.Occurrence.Source.ActionKind() != careplan.KindMedication {
			continue
		}
		if it.Status == careplan.StatusOverridden {
			continue // the exception suppresses the requirement entirely
		}
		due := it.Occurrence.DueAt
		if due.Before(todayStart) || !due.Before(todayEnd) {
			continue // the column summarizes TODAY's buckets only
		}
		idx := 0 // morning
		switch h := due.Hour(); {
		case h >= 16:
			idx = 2 // evening
		case h >= 11:
			idx = 1 // noon
		}
		a := acc[it.Occurrence.AnimalID]
		if !a[idx].set {
			a[idx] = bucketAcc{set: true, done: true}
		}
		if it.Status != careplan.StatusApplied {
			a[idx].done = false
		}
		acc[it.Occurrence.AnimalID] = a
	}

	stats := make(map[int]models.TreatmentStatusStatistics, len(acc))
	for animalID, a := range acc {
		s := models.TreatmentStatusStatistics{}
		bucket := func(b bucketAcc) nulls.Bool {
			if !b.set {
				return nulls.Bool{}
			}
			return nulls.NewBool(b.done)
		}
		s.Morning, s.Noon, s.Evening = bucket(a[0]), bucket(a[1]), bucket(a[2])
		stats[animalID] = s
	}
	return stats, nil
}

// andBuckets mirrors the models-internal and(): a dst without data adopts
// src; two known values AND (a bucket is done only when every side is).
func andBuckets(dst *nulls.Bool, src nulls.Bool) {
	if dst.Valid {
		if src.Valid {
			*dst = nulls.NewBool(dst.Bool && src.Bool)
		}
	} else {
		*dst = src
	}
}

// mergedTreatmentStats folds the protocol bucket summary into each animal's
// legacy Treatments stat — the value the landing "Treatments" column
// renders. Animals missing from either side keep the other side untouched.
func mergedTreatmentStats(animals *models.Animals, protocol map[int]models.TreatmentStatusStatistics) map[int]models.TreatmentStatusStatistics {
	out := make(map[int]models.TreatmentStatusStatistics, len(*animals))
	for i := range *animals {
		a := &(*animals)[i]
		s := a.Treatments.TodayStatitics()
		if pb, ok := protocol[a.ID]; ok {
			andBuckets(&s.Morning, pb.Morning)
			andBuckets(&s.Noon, pb.Noon)
			andBuckets(&s.Evening, pb.Evening)
		}
		out[a.ID] = s
	}
	return out
}
