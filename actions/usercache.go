package actions

import (
	"creaves/models"
	"database/sql"
	"sync"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/pkg/errors"
)

// Short-TTL cache for the current-user lookup done by SetCurrentUser on
// every authenticated request. User rows change rarely and only through
// UsersResource Update/Destroy, which invalidate the entry below.
//
// Staleness window is bounded by userCacheTTL: an approval/admin/password
// change may take up to that long to be visible to other in-flight sessions
// of the same user. The cache is process-local (single app container).

const userCacheTTL = 30 * time.Second

// userCacheMaxEntries bounds the cache map. Sessions are the only source of
// lookups, but a stale/garbled session id per visitor could otherwise grow
// the map without limit. When the cap is hit, expired entries are pruned;
// if that is not enough the whole cache is dropped (safe: entries are
// advisory and at most userCacheTTL stale).
const userCacheMaxEntries = 1024

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

// pruneLocked evicts expired entries and, if the map is still at capacity,
// clears it entirely. Caller must hold the write lock.
func pruneLocked(now time.Time) {
	if len(userCache.entries) < userCacheMaxEntries {
		return
	}
	for id, e := range userCache.entries {
		if now.Sub(e.fetchedAt) >= userCacheTTL {
			delete(userCache.entries, id)
		}
	}
	if len(userCache.entries) >= userCacheMaxEntries {
		userCache.entries = map[string]userCacheEntry{}
	}
}

// cachedUserByID returns the user for id, using the short-TTL cache.
// Returns (nil, nil) when the user does not exist. Database failures are
// NOT cached and are propagated: caching them would pin every request of a
// user to a logged-out state for a full TTL window.
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
		if errors.Cause(err) == sql.ErrNoRows || errors.Is(err, sql.ErrNoRows) {
			// Cache the miss as well: deleted users keep getting requested
			// until their sessions expire.
			pruneLocked(time.Now())
			userCache.entries[id] = userCacheEntry{user: nil, fetchedAt: time.Now()}
			return nil, nil //nolint:nilnil // miss is a valid cached outcome
		}
		return nil, errors.WithStack(err)
	}
	pruneLocked(time.Now())
	userCache.entries[id] = userCacheEntry{user: u, fetchedAt: time.Now()}
	return u, nil
}
