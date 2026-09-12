package actions

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gobuffalo/buffalo"
)

// serveProbe registers PostCommitInvalidations plus a probe handler and
// serves one request through a real buffalo app so contexts carry a
// *buffalo.Response, as they do under popmw.Transaction.
func serveProbe(t *testing.T, status int, handlerErr error, queue func(c buffalo.Context)) int32 {
	t.Helper()
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.Use(PostCommitInvalidations)
	a.POST("/probe/", func(c buffalo.Context) error {
		queue(c)
		if handlerErr != nil {
			return handlerErr
		}
		return c.Render(status, nil)
	})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/probe/", nil)
	a.ServeHTTP(w, req)
	return int32(w.Code)
}

// TestPostCommitInvalidationsFlushOnCommit proves queued invalidations run
// after the handler chain (== transaction middleware) returns successfully.
func TestPostCommitInvalidationsFlushOnCommit(t *testing.T) {
	var ran int32
	code := serveProbe(t, http.StatusOK, nil, func(c buffalo.Context) {
		queuePostCommitInvalidation(c, func() { atomic.StoreInt32(&ran, 1) })
	})
	if code != http.StatusOK {
		t.Fatalf("probe status = %d, want 200", code)
	}
	if atomic.LoadInt32(&ran) != 1 {
		t.Error("queued invalidation did not run after successful handler")
	}
}

// TestPostCommitInvalidationsDroppedOnHandlerError proves an error returned
// by the inner chain (rollback) drops queued invalidations.
func TestPostCommitInvalidationsDroppedOnHandlerError(t *testing.T) {
	var ran int32
	serveProbe(t, http.StatusOK, errTestBoom, func(c buffalo.Context) {
		queuePostCommitInvalidation(c, func() { atomic.StoreInt32(&ran, 1) })
	})
	if atomic.LoadInt32(&ran) != 0 {
		t.Error("queued invalidation ran despite handler error (would race rollback)")
	}
}

// TestPostCommitInvalidationsDroppedOnNonSuccess proves a non-2xx/3xx
// response (popmw maps that to errNonSuccess -> rollback) drops the queue.
func TestPostCommitInvalidationsDroppedOnNonSuccess(t *testing.T) {
	var ran int32
	serveProbe(t, http.StatusUnprocessableEntity, nil, func(c buffalo.Context) {
		queuePostCommitInvalidation(c, func() { atomic.StoreInt32(&ran, 1) })
	})
	if atomic.LoadInt32(&ran) != 0 {
		t.Error("queued invalidation ran despite non-success status (would race rollback)")
	}
}

// TestQueuePostCommitFallbackRunsImmediately proves that without the
// middleware (unit-test / non-transactional route), the invalidation still
// executes instead of being silently lost.
func TestQueuePostCommitFallbackRunsImmediately(t *testing.T) {
	var ran int32
	a := buffalo.New(buffalo.Options{Env: "test"})
	a.POST("/probe/", func(c buffalo.Context) error {
		queuePostCommitInvalidation(c, func() { atomic.StoreInt32(&ran, 1) })
		return c.Render(http.StatusOK, nil)
	})
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/probe/", nil))
	if atomic.LoadInt32(&ran) != 1 {
		t.Error("fallback path did not execute queued invalidation immediately")
	}
}

// TestReferenceDeleteInvalidatorsCoverCachedTables pins the table->cache
// mapping: every deleteable reference that feeds the ref cache must be listed.
func TestReferenceDeleteInvalidatorsCoverCachedTables(t *testing.T) {
	for _, table := range []string{"animalages", "animaltypes", "caretypes", "outtaketypes", "traveltypes"} {
		if referenceDeleteInvalidators[table] == nil {
			t.Errorf("referenceDeleteInvalidators missing cached table %q", table)
		}
	}
	for _, table := range []string{"discoverers", "drugs"} {
		if _, ok := referenceDeleteInvalidators[table]; ok {
			t.Errorf("referenceDeleteInvalidators has unexpected entry for uncached table %q", table)
		}
	}
}

// errTestBoom is a sentinel error for the handler-error path.
var errTestBoom = &testError{}

type testError struct{}

func (*testError) Error() string { return "boom" }
