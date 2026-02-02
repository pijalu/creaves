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

// Initialize the cache at startup
func init() {
	// Start a goroutine to refresh the cache periodically
	go func() {
		for {
			time.Sleep(cacheUpdateInterval)
			refreshWeightLossCache()
		}
	}()
}

// refreshWeightLossCache refreshes the weight loss cache
func refreshWeightLossCache() {
	// Note: Since this runs outside of a Buffalo context, we can't directly access the DB
	// This would need to be called from a context where DB is available
	// For now, we'll implement a manual refresh mechanism
}

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
