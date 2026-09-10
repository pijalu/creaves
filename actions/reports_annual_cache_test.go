package actions

import (
	"testing"
	"time"

	"creaves/models"
)

// TestAnnualStatsCacheHitAndIsolation proves, against the reports-annual
// fixtures: (1) a fresh computation is cached and served on the next call,
// (2) every hit returns a deep copy so request-time localization can never
// corrupt the stored canonical values, (3) InvalidateAnnualStatsCache
// forces a recompute.
func TestAnnualStatsCacheHitAndIsolation(t *testing.T) {
	setupReportsAnnualFixtures(t)
	InvalidateAnnualStatsCache()

	first, err := runAnnualStatsCached(models.DB, raftYear)
	if err != nil {
		t.Fatalf("runAnnualStatsCached failed: %v", err)
	}
	marker := first[0].Total

	// Poison the stored entry: if the next call returns the poisoned value
	// it was served from cache, not recomputed.
	annualStatsMu.Lock()
	annualStatsCache[raftYear][0].Total = 424242
	annualStatsMu.Unlock()

	second, err := runAnnualStatsCached(models.DB, raftYear)
	if err != nil {
		t.Fatalf("cached runAnnualStatsCached failed: %v", err)
	}
	if second[0].Total != 424242 {
		t.Fatalf("expected cache hit (poisoned total 424242), got %d", second[0].Total)
	}

	// Mutate the returned copy the way localizeAnnualSections would; the
	// cache and earlier returned copies must stay untouched.
	second[0].Total = 1
	third, err := runAnnualStatsCached(models.DB, raftYear)
	if err != nil {
		t.Fatalf("third runAnnualStatsCached failed: %v", err)
	}
	if third[0].Total != 424242 {
		t.Errorf("caller mutation leaked into cache: %d", third[0].Total)
	}
	if first[0].Total != marker {
		t.Errorf("earlier returned copy was mutated: %d, want %d", first[0].Total, marker)
	}

	// Invalidation drops the poisoned entry and a recompute restores the
	// real value.
	InvalidateAnnualStatsCache()
	fourth, err := runAnnualStatsCached(models.DB, raftYear)
	if err != nil {
		t.Fatalf("post-invalidation runAnnualStatsCached failed: %v", err)
	}
	if fourth[0].Total == 424242 {
		t.Errorf("invalidation ignored: poisoned value still served")
	}
	if fourth[0].Total != marker {
		t.Errorf("recomputed total = %d, want %d", fourth[0].Total, marker)
	}
}

// TestAnnualStatsCacheTTLExpiry proves an expired entry is recomputed
// rather than served.
func TestAnnualStatsCacheTTLExpiry(t *testing.T) {
	setupReportsAnnualFixtures(t)
	InvalidateAnnualStatsCache()

	if _, err := runAnnualStatsCached(models.DB, raftYear); err != nil {
		t.Fatalf("runAnnualStatsCached failed: %v", err)
	}

	annualStatsMu.Lock()
	annualStatsCache[raftYear][0].Total = 777
	annualStatsCacheAt[raftYear] = time.Now().Add(-annualStatsCacheTTL - time.Second)
	annualStatsMu.Unlock()

	second, err := runAnnualStatsCached(models.DB, raftYear)
	if err != nil {
		t.Fatalf("expired runAnnualStatsCached failed: %v", err)
	}
	if second[0].Total == 777 {
		t.Errorf("expired cache entry was served")
	}
}
