package actions

import (
	"creaves/models"
	"sync"
	"time"

	"github.com/gobuffalo/pop/v6"
)

// Short-TTL cache for the current-user lookup done by SetCurrentUser on
// every authenticated request. User rows change rarely and only through
// UsersResource Update/Destroy, which invalidate the entry below.
//
// Staleness window is bounded by userCacheTTL: an approval/admin/password
// change may take up to that long to be visible to other in-flight sessions
// of the same user. The cache is process-local (single app container).

const userCacheTTL = 30 * time.Second

type userCacheEntry struct {
	user      *models.User // nil user == known-missing id
	fetchedAt time.Time
}

var userCache = struct {
	sync.RWMutex
	entries map[string]userCacheEntry
}{entries: map[string]userCacheEntry{}}

// InvalidateUserCache drops one user from the cache (call after update/delete).
func InvalidateUserCache(id string) {
	userCache.Lock()
	delete(userCache.entries, id)
	userCache.Unlock()
}

// cachedUserByID returns the user for id, using the short-TTL cache.
// Returns (nil, nil) when the user does not exist.
func cachedUserByID(tx *pop.Connection, id string) (*models.User, error) {
	userCache.RLock()
	if e, ok := userCache.entries[id]; ok && time.Since(e.fetchedAt) < userCacheTTL {
		userCache.RUnlock()
		return e.user, nil
	}
	userCache.RUnlock()

	userCache.Lock()
	defer userCache.Unlock()
	if e, ok := userCache.entries[id]; ok && time.Since(e.fetchedAt) < userCacheTTL {
		return e.user, nil
	}

	u := &models.User{}
	if err := tx.Find(u, id); err != nil {
		// Cache the miss as well: deleted users keep getting requested
		// until their sessions expire.
		userCache.entries[id] = userCacheEntry{user: nil, fetchedAt: time.Now()}
		return nil, nil //nolint:nilnil // miss is a valid cached outcome
	}
	userCache.entries[id] = userCacheEntry{user: u, fetchedAt: time.Now()}
	return u, nil
}
