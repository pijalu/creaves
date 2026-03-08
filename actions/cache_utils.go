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
}
