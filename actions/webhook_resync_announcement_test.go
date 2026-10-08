//go:build sqlite
// +build sqlite

package actions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-9 regression: a resync run whose every event is skipped unchanged
// completes with events_delivered == 0 and never puts a batch on the wire —
// but announcements only travel on delivered batches, so the console never
// learned the producer's expected sync set (Expected(producer) stayed "–" and
// the "matches producer" badge could never render after a console-side
// cleanup followed by an all-in-sync resync). On completion such a run must
// send ONE announcement-only batch (normal envelope + "sync" header +
// "events": []) to every enabled sync target, carrying exactly the
// total/checksum/announced_at the run computed, without counting as a
// delivered event.
func TestAllSkippedResyncSendsAnnouncementOnlyBatch(t *testing.T) {
	resetPusherState()
	seedResyncNestedFixture(t)

	const instanceID = "rsync-resync-test"

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	newRun := func() *models.ResyncRun {
		return &models.ResyncRun{
			ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID,
			Status: "running", StartedAt: time.Now(), TotalAnimals: 1,
		}
	}

	// Run 1 — normal resync: creates the state event and delivers it (its
	// announcement rides along on that delivered batch — existing behavior).
	// This brings the receiver "in sync" so run 2 can skip everything.
	run1 := newRun()
	require.NoError(t, models.DB.Create(run1))
	require.NoError(t, RunResync(context.Background(), models.DB, run1.ID, false))
	require.NoError(t, models.DB.Find(run1, run1.ID))
	require.Equal(t, "completed", run1.Status)
	require.Equal(t, 1, run1.EventsDelivered)

	// Run 2 — normal (non-force) resync over the unchanged animal: the event
	// already exists with the same content hash, so everything is skipped and
	// nothing is delivered. Pre-BUG-9 this run sent no webhook at all.
	run2 := newRun()
	require.NoError(t, models.DB.Create(run2))
	require.NoError(t, RunResync(context.Background(), models.DB, run2.ID, false))
	require.NoError(t, models.DB.Find(run2, run2.ID))

	// The run completed all-skipped and computed its announcement.
	assert.Equal(t, "completed", run2.Status)
	assert.Equal(t, 1, run2.EventsSkippedUnchanged)
	assert.Zero(t, run2.EventsCreated)
	assert.Zero(t, run2.EventsDelivered, "the announcement batch must not count as a delivered event")
	assert.Zero(t, run2.EventsFailed)
	assert.Equal(t, 1, run2.AnnouncedExpectedTotal)
	require.NotNil(t, run2.AnnouncedExpectedChecksum)
	require.NotEmpty(t, *run2.AnnouncedExpectedChecksum)
	require.NotNil(t, run2.AnnouncedAt)

	// Exactly two requests reached the receiver: run 1's event batch and
	// run 2's announcement-only batch.
	rr.mu.Lock()
	require.Len(t, rr.requests, 2)
	first, ann := rr.requests[0], rr.requests[1]
	rr.mu.Unlock()

	assert.Equal(t, 1, first.count, "sanity: run 1 delivered its event batch")
	assert.Zero(t, ann.count, "the announcement batch must carry zero events")
	assert.Equal(t, "Bearer creaves_testkey", ann.auth, "announcement batch authenticates like any batch")

	var envelope struct {
		ContractVersion int `json:"contract_version"`
		Instance        *struct {
			ID string `json:"id"`
		} `json:"instance"`
		Sync *struct {
			ExpectedTotal    int        `json:"expected_total"`
			ExpectedChecksum string     `json:"expected_checksum"`
			AnnouncedAt      *time.Time `json:"announced_at"`
		} `json:"sync"`
		Events []json.RawMessage `json:"events"`
	}
	require.NoError(t, json.Unmarshal(ann.body, &envelope))
	assert.Equal(t, 2, envelope.ContractVersion)
	require.NotNil(t, envelope.Instance)
	assert.Equal(t, "test-instance", envelope.Instance.ID)
	require.NotNil(t, envelope.Sync, "announcement batch must carry the sync header")
	assert.Equal(t, run2.AnnouncedExpectedTotal, envelope.Sync.ExpectedTotal,
		"sync.expected_total must be what the run computed")
	assert.Equal(t, *run2.AnnouncedExpectedChecksum, envelope.Sync.ExpectedChecksum,
		"sync.expected_checksum must be what the run computed")
	require.NotNil(t, envelope.Sync.AnnouncedAt)
	assert.WithinDuration(t, *run2.AnnouncedAt, *envelope.Sync.AnnouncedAt, time.Second,
		"sync.announced_at must be what the run computed")
	assert.Empty(t, envelope.Events, "announcement batch must contain no events")

	// The receiver's total event count stays at run 1's single event: the
	// announcement batch delivered no event.
	assert.Equal(t, 1, rr.totalEvents())

	// And the run still attributes zero events to itself (nothing created,
	// nothing delivered — the empty batch created nothing anywhere).
	total2, delivered2, err := countResyncRunEvents(models.DB, run2.ID)
	require.NoError(t, err)
	assert.Zero(t, total2)
	assert.Zero(t, delivered2)
}
