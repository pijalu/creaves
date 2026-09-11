package actions

import (
	"sync"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
)

// Global cache for weight loss data
var (
	weightLossCache     *[]AnimalWithWeight
	cacheMutex          sync.RWMutex
	cacheLastUpdate     time.Time
	cacheUpdateInterval = 12 * time.Hour // Update cache every 12 hours
)

// weightAffectsLossCache reports whether a care change can alter weight-loss results.
// Both sides matter on update: clearing a previous weight must invalidate cached data.
func weightAffectsLossCache(oldWeight, newWeight nulls.String) bool {
	return (oldWeight.Valid && len(oldWeight.String) > 0) ||
		(newWeight.Valid && len(newWeight.String) > 0)
}

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

// WeightLossCachedAt returns when the currently cached weight loss data was computed.
// Zero time if the cache has never been populated.
func WeightLossCachedAt() time.Time {
	cacheMutex.RLock()
	defer cacheMutex.RUnlock()
	return cacheLastUpdate
}

// InvalidateWeightLossCache marks the cache as stale (will be refreshed on next access)
func InvalidateWeightLossCache() {
	cacheMutex.Lock()
	cacheLastUpdate = time.Time{} // Set to zero time to force refresh on next access
	cacheMutex.Unlock()
}
