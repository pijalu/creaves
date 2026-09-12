package actions

import (
	"strconv"
	"testing"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

func userCacheReset(t *testing.T) {
	t.Helper()
	userCache.Lock()
	userCache.entries = map[string]userCacheEntry{}
	userCache.Unlock()
	t.Cleanup(func() {
		userCache.Lock()
		userCache.entries = map[string]userCacheEntry{}
		userCache.Unlock()
	})
}

// brokenUserCacheConn returns a lazily-opened MySQL connection that always
// fails on first query (nothing listens there). Skips the test when the
// driver cannot even be registered.
func brokenUserCacheConn(t *testing.T) *pop.Connection {
	t.Helper()
	cd := &pop.ConnectionDetails{
		Dialect: "mysql",
		URL:     "mysql://creaves:creaves@(127.0.0.1:1)/creaves_none?parseTime=true",
	}
	conn, err := pop.NewConnection(cd)
	if err != nil {
		t.Skipf("mysql driver unavailable: %v", err)
	}
	// Open is lazy for MySQL: it must succeed even with nothing listening.
	if err := conn.Open(); err != nil {
		t.Skipf("could not open connection handle: %v", err)
	}
	return conn
}

func TestCachedUserByID_MissIsCachedAsNil(t *testing.T) {
	tx := searchTestDB(t)
	userCacheReset(t)

	id := uuid.Must(uuid.NewV4()).String()
	u, err := cachedUserByID(tx, id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u != nil {
		t.Fatal("expected nil user for unknown id")
	}

	userCache.RLock()
	e, ok := userCache.entries[id]
	userCache.RUnlock()
	if !ok || e.user != nil {
		t.Fatal("miss should be cached as a nil-user entry")
	}
}

func TestCachedUserByID_DBErrorNotCached(t *testing.T) {
	userCacheReset(t)

	id := uuid.Must(uuid.NewV4()).String()
	u, err := cachedUserByID(brokenUserCacheConn(t), id)
	if err == nil {
		t.Fatal("expected the database error to propagate")
	}
	if u != nil {
		t.Fatal("expected nil user on error")
	}

	userCache.RLock()
	_, cached := userCache.entries[id]
	userCache.RUnlock()
	if cached {
		t.Fatal("database error must not be cached (would pin user as logged out)")
	}
}

func TestUserCachePruneLocked(t *testing.T) {
	userCacheReset(t)
	now := time.Now()

	userCache.Lock()
	defer userCache.Unlock()

	// At capacity with all-fresh entries: expired-only pruning frees nothing,
	// so the cache is dropped entirely (bounded growth guarantee).
	for i := 0; i < userCacheMaxEntries; i++ {
		userCache.entries[strconv.Itoa(i)] = userCacheEntry{fetchedAt: now}
	}
	pruneLocked(now)
	if len(userCache.entries) != 0 {
		t.Fatalf("expected full reset at capacity with no expired entries, got %d", len(userCache.entries))
	}

	// Half expired: pruning keeps only the fresh half.
	for i := 0; i < userCacheMaxEntries; i++ {
		e := userCacheEntry{fetchedAt: now}
		if i%2 == 0 {
			e.fetchedAt = now.Add(-2 * userCacheTTL)
		}
		userCache.entries[strconv.Itoa(i)] = e
	}
	pruneLocked(now)
	if len(userCache.entries) != userCacheMaxEntries/2 {
		t.Fatalf("expected expired entries pruned, got %d", len(userCache.entries))
	}
}
