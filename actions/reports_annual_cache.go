package actions

import (
	"sync"
	"time"

	"github.com/gobuffalo/pop/v6"
)

// Annual statistics cache. One report view costs 24 grouped/total queries
// (12 sections x 2) and the numbers only change when animals or outtakes
// are written, so canonical (pre-localization) sections are cached per year
// with a TTL and invalidated on animal/outtake writes. localizeAnnualSections
// rewrites Category strings in place, therefore every cache hit hands out a
// deep copy; the stored values stay language-neutral.
var (
	annualStatsMu       sync.RWMutex
	annualStatsCache    = map[string][]annualStatSection{}
	annualStatsCacheAt  = map[string]time.Time{}
	annualStatsCacheTTL = 30 * time.Minute
)

// InvalidateAnnualStatsCache drops all cached annual statistics so the next
// report view recomputes them. Called after successful animal and outtake
// writes (the only inputs of the annual tables).
func InvalidateAnnualStatsCache() {
	annualStatsMu.Lock()
	annualStatsCache = map[string][]annualStatSection{}
	annualStatsCacheAt = map[string]time.Time{}
	annualStatsMu.Unlock()
}

// copyAnnualSections deep-copies sections so caller mutations (request-time
// localization) can never reach the cached values.
func copyAnnualSections(src []annualStatSection) []annualStatSection {
	out := make([]annualStatSection, len(src))
	for i, s := range src {
		rows := make([]annualStatRow, len(s.Rows))
		copy(rows, s.Rows)
		out[i] = annualStatSection{ID: s.ID, Rows: rows, Total: s.Total}
	}
	return out
}

// runAnnualStatsCached returns the canonical annual statistics for year,
// serving from the per-year in-process cache while fresh (TTL) and
// recomputing via runAnnualStats otherwise. Nothing is cached on error.
func runAnnualStatsCached(tx *pop.Connection, year string) ([]annualStatSection, error) {
	annualStatsMu.RLock()
	if secs, ok := annualStatsCache[year]; ok && time.Since(annualStatsCacheAt[year]) < annualStatsCacheTTL {
		annualStatsMu.RUnlock()
		return copyAnnualSections(secs), nil
	}
	annualStatsMu.RUnlock()

	secs, err := runAnnualStats(tx, year)
	if err != nil {
		return nil, err
	}

	annualStatsMu.Lock()
	annualStatsCache[year] = secs
	annualStatsCacheAt[year] = time.Now()
	annualStatsMu.Unlock()
	return copyAnnualSections(secs), nil
}
