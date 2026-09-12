package actions

import (
	"github.com/gobuffalo/buffalo"
)

// Post-commit invalidation queue.
//
// popmw.Transaction commits (or rolls back) the request transaction only
// after the whole inner handler chain returns, so cache invalidations issued
// inside a handler race the commit: another request could reload the cache
// from the still-uncommitted (old) state. Queuing the invalidation and
// flushing it after the transaction middleware unwinds closes that window —
// by then the commit has either landed (flush) or been dropped (rollback,
// nothing changed).
//
// Residual window (bounded): a reader that loaded a cached slice just before
// the flush keeps serving it until the next invalidation. refCacheTTL and
// userCacheTTL bound that staleness.

const refInvalidationQueueKey = "creaves.refcache.postcommit"

type postCommitQueue []func()

// queuePostCommitInvalidation defers fn until the request transaction has
// committed. If no queue middleware ran (unit tests, a route outside the
// transaction middleware) fn executes immediately rather than being lost.
func queuePostCommitInvalidation(c buffalo.Context, fn func()) {
	q, ok := c.Value(refInvalidationQueueKey).(*postCommitQueue)
	if !ok || q == nil {
		fn()
		return
	}
	*q = append(*q, fn)
}

// PostCommitInvalidations must be registered BEFORE popmw.Transaction so it
// wraps it: by the time this middleware resumes, the transaction middleware
// has already committed or rolled back. Queued invalidations are flushed on
// commit only; the commit-vs-status mapping mirrors popmw.Transaction
// (2xx/3xx == committed, anything else == rolled back via errNonSuccess).
func PostCommitInvalidations(h buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		q := postCommitQueue{}
		c.Set(refInvalidationQueueKey, &q)
		err := h(c)
		if err != nil {
			return err // rolled back — drop queued invalidations
		}
		if res, ok := c.Response().(*buffalo.Response); ok && (res.Status < 200 || res.Status >= 400) {
			return nil // non-success: popmw rolled back — drop queue
		}
		for _, fn := range q {
			fn()
		}
		return nil
	}
}
