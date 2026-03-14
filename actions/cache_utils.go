package actions

import (
	"sync"
	"time"

	"github.com/gobuffalo/buffalo"
)

// Global cache for weight loss data
var (
	weightLossCache     *[]AnimalWithWeight
	cacheMutex          sync.RWMutex
	cacheLastUpdate     time.Time
	cacheUpdateInterval = 12 * time.Hour // Update cache every 12 hours
)

// Dashboard cache
var (
	dashboardCache        *DashboardCacheData
	dashboardCacheMutex   sync.RWMutex
	dashboardCacheLastUpdate time.Time
	dashboardCacheInterval   = 5 * time.Minute // Update dashboard cache every 5 minutes
)

// DashboardCacheData holds cached dashboard statistics
type DashboardCacheData struct {
	AnimalCountPerType []listAnimalCountPerTypeReply
	TotalAnimalCount   int
	OpenCaresCount     int
}

// refreshWeightLossCache refreshes the weight loss cache
// Note: Cache relies on manual invalidation when animal data changes
func refreshWeightLossCache() {}

// GetWeightLossData returns weight loss data, using cache if available
func GetWeightLossData(c buffalo.Context) (*[]AnimalWithWeight, error) {
	cacheMutex.RLock()
	if weightLossCache != nil && time.Since(cacheLastUpdate) < cacheUpdateInterval && !cacheLastUpdate.IsZero() {
		result := *weightLossCache
		cacheMutex.RUnlock()
		return &result, nil
	}
	cacheMutex.RUnlock()

	// Cache is stale, empty, or invalidated, fetch fresh data
	newData, err := listAnimalWithWeightLoss(c)
	if err != nil {
		return nil, err
	}

	// Update the cache
	cacheMutex.Lock()
	weightLossCache = newData
	cacheLastUpdate = time.Now()
	cacheMutex.Unlock()

	return newData, nil
}

// InvalidateWeightLossCache marks the cache as stale (will be refreshed on next access)
func InvalidateWeightLossCache() {
	cacheMutex.Lock()
	cacheLastUpdate = time.Time{} // Set to zero time to force refresh on next access
	cacheMutex.Unlock()
	InvalidateDashboardCache()    // Also invalidate dashboard cache
}

// GetDashboardStats returns cached dashboard statistics
func GetDashboardStats(c buffalo.Context) (*DashboardCacheData, error) {
	dashboardCacheMutex.RLock()
	if dashboardCache != nil && time.Since(dashboardCacheLastUpdate) < dashboardCacheInterval && !dashboardCacheLastUpdate.IsZero() {
		result := *dashboardCache
		dashboardCacheMutex.RUnlock()
		return &result, nil
	}
	dashboardCacheMutex.RUnlock()

	// Cache is stale or empty, fetch fresh data
	// Get animal count per type
	ct, err := listAnimalCountPerType(c)
	if err != nil {
		return nil, err
	}

	// Calculate total
	totalAnimal := 0
	for _, cti := range ct {
		totalAnimal += cti.Count
	}

	// Get open cares count (simplified - just count for cache)
	oc, err := listOpenCares(c)
	if err != nil {
		return nil, err
	}

	// Update the cache
	dashboardCacheMutex.Lock()
	dashboardCache = &DashboardCacheData{
		AnimalCountPerType: ct,
		TotalAnimalCount:   totalAnimal,
		OpenCaresCount:     len(oc),
	}
	dashboardCacheLastUpdate = time.Now()
	dashboardCacheMutex.Unlock()

	return dashboardCache, nil
}

// InvalidateDashboardCache marks the dashboard cache as stale
func InvalidateDashboardCache() {
	dashboardCacheMutex.Lock()
	dashboardCacheLastUpdate = time.Time{}
	dashboardCacheMutex.Unlock()
}

// GetCachedAnimalCountPerType returns cached animal count per type
func GetCachedAnimalCountPerType(c buffalo.Context) ([]listAnimalCountPerTypeReply, int, error) {
	stats, err := GetDashboardStats(c)
	if err != nil {
		return nil, 0, err
	}
	if stats == nil {
		// Fallback to direct query if cache not available
		ct, err := listAnimalCountPerType(c)
		if err != nil {
			return nil, 0, err
		}
		total := 0
		for _, cti := range ct {
			total += cti.Count
		}
		return ct, total, nil
	}
	return stats.AnimalCountPerType, stats.TotalAnimalCount, nil
}
