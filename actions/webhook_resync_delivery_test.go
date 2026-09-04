//go:build sqlite
// +build sqlite

package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for resync delivery accounting (bugs.md #1): a resync run may only
// report "completed" when every event it created was accepted by the
// receiver; partial batch accepts (e.g. 97/100) must be retried; impossible
// delivery must mark the run failed with delivered/failed diagnostics.

// seedResyncDeliveryRun inserts a fresh running resync run row and wipes it
// (plus its events) afterwards.
func seedResyncDeliveryRun(t *testing.T) *models.ResyncRun {
	t.Helper()
	cleanup := func() {
		models.DB.RawQuery("DELETE FROM event_streams WHERE instance_id = 'test-instance'").Exec()
		models.DB.RawQuery("DELETE FROM resync_runs WHERE instance_id = 'test-instance'").Exec()
	}
	cleanup()
	t.Cleanup(cleanup)
	run := &models.ResyncRun{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Status:     "running",
		StartedAt:  time.Now(),
	}
	require.NoError(t, models.DB.Create(run))
	return run
}

// seedResyncDeliveryEvent inserts an undelivered event attributed to run.
func seedResyncDeliveryEvent(t *testing.T, run *models.ResyncRun, animalID int) *models.EventStream {
	t.Helper()
	ev := &models.EventStream{
		ID:           uuid.Must(uuid.NewV4()),
		InstanceID:   "test-instance",
		AnimalID:     animalID,
		EventType:    string(models.EventTypeAnimalState),
		ResyncRunID:  &run.ID,
		Payload:      []byte(`{"animal":{"species":"Fox"}}`),
		CreatedAt:    time.Now(),
	}
	require.NoError(t, models.DB.Create(ev))
	return ev
}

// tightenResyncDeliveryBounds shrinks the delivery loop's sleep and stall
// bound so tests stay fast; restores the originals on cleanup.
func tightenResyncDeliveryBounds(t *testing.T, maxStalled int) {
	t.Helper()
	oldDelay := resyncDeliveryPollDelay
	oldStalled := resyncDeliveryMaxStalled
	resyncDeliveryPollDelay = time.Millisecond
	resyncDeliveryMaxStalled = maxStalled
	t.Cleanup(func() {
		resyncDeliveryPollDelay = oldDelay
		resyncDeliveryMaxStalled = oldStalled
	})
}

// countResyncReceiver is a webhook receiver that accepts every batch except
// a configurable blacklist of event IDs; when rejecting it answers with the
// console partial-response shape (processed_ids lists only accepted events).
type countResyncReceiver struct {
	mu       sync.Mutex
	rejects  map[string]bool
	batches  [][]string // event IDs per received request
	rejected int
}

func (cr *countResyncReceiver) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var payload struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	_ = json.Unmarshal(body, &payload)
	cr.mu.Lock()
	defer cr.mu.Unlock()
	accepted := make([]string, 0, len(payload.Events))
	for _, ev := range payload.Events {
		if cr.rejects[ev.ID] {
			cr.rejected++
			continue
		}
		accepted = append(accepted, ev.ID)
	}
	cr.batches = append(cr.batches, accepted)
	w.Header().Set("Content-Type", "application/json")
	if len(accepted) == len(payload.Events) {
		fmt.Fprintf(w, `{"processed":%d,"total":%d}`, len(accepted), len(payload.Events))
		return
	}
	resp := map[string]interface{}{"processed": len(accepted), "total": len(payload.Events), "errors": []string{"receiver rejected some events"}}
	resp["processed_ids"] = accepted
	_ = json.NewEncoder(w).Encode(resp)
}

func (cr *countResyncReceiver) requestCount() int {
	cr.mu.Lock()
	defer cr.mu.Unlock()
	return len(cr.batches)
}

// TestResyncDeliveryRetriesPartialAccept proves the delivery loop retries a
// partially accepted batch (e.g. 97/100): the receiver rejects ONE event of
// the first batch, accepts everything afterwards; the run must end
// "completed" with all events delivered and zero failed.
func TestResyncDeliveryRetriesPartialAccept(t *testing.T) {
	resetPusherState()
	tightenResyncDeliveryBounds(t, 10)

	run := seedResyncDeliveryRun(t)
	var events []*models.EventStream
	for i := 0; i < 4; i++ {
		ev := seedResyncDeliveryEvent(t, run, 100+i)
		events = append(events, ev)
	}
	// Reject exactly one event on the FIRST batch only.
	rejectedID := events[2].ID.String()
	var mu sync.Mutex
	batches := 0
	first := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		batches++
		isFirst := first
		first = false
		mu.Unlock()
		if isFirst {
			accepted := make([]string, 0, len(payload.Events))
			for _, ev := range payload.Events {
				if ev.ID != rejectedID {
					accepted = append(accepted, ev.ID)
				}
			}
			resp := map[string]interface{}{"processed": len(accepted), "total": len(payload.Events), "errors": []string{"transient rejection"}}
			resp["processed_ids"] = accepted
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		fmt.Fprintf(w, `{"processed":%d,"total":%d}`, len(payload.Events), len(payload.Events))
	}))
	defer srv.Close()

	saved := CurrentConfig
	seedPusherConfig(t, srv.URL)
	t.Cleanup(func() { CurrentConfig = saved })

	require.NoError(t, completeResyncDelivery(context.Background(), models.DB, run))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run.ID))
	assert.Equal(t, "completed", reloaded.Status, "run must complete once everything is delivered")
	assert.Equal(t, 4, reloaded.EventsDelivered)
	assert.Equal(t, 0, reloaded.EventsFailed)

	for _, ev := range events {
		got := &models.EventStream{}
		require.NoError(t, models.DB.Find(got, ev.ID))
		assert.NotNil(t, got.DeliveredAt, "event %s must be delivered", ev.ID)
	}
	// The receiver saw more than one batch: the rejected event was retried.
	mu.Lock()
	gotBatches := batches
	mu.Unlock()
	assert.Greater(t, gotBatches, 1, "rejected event must be retried in a follow-up batch")
}

// TestResyncDeliveryHardFailMarksRunFailed proves a receiver that permanently
// rejects some events cannot yield a green run: after the bounded stall limit
// the run is "failed" with delivered/failed counts and a diagnostic.
func TestResyncDeliveryHardFailMarksRunFailed(t *testing.T) {
	resetPusherState()
	tightenResyncDeliveryBounds(t, 2)

	run := seedResyncDeliveryRun(t)
	reject := seedResyncDeliveryEvent(t, run, 7).ID.String()
	for i := 0; i < 3; i++ {
		seedResyncDeliveryEvent(t, run, 100+i)
	}

	cr := &countResyncReceiver{rejects: map[string]bool{reject: true}}
	srv := httptest.NewServer(http.HandlerFunc(cr.handler))
	defer srv.Close()

	saved := CurrentConfig
	seedPusherConfig(t, srv.URL)
	t.Cleanup(func() { CurrentConfig = saved })

	require.NoError(t, completeResyncDelivery(context.Background(), models.DB, run))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run.ID))
	assert.Equal(t, "failed", reloaded.Status)
	assert.Equal(t, 3, reloaded.EventsDelivered)
	assert.Equal(t, 1, reloaded.EventsFailed)
	assert.Contains(t, reloaded.Errors, "delivery incomplete")
	assert.Contains(t, reloaded.Errors, "1 events not delivered")
	// The rejected event was retried (more than one request hit the receiver).
	assert.Greater(t, cr.requestCount(), 1)
}

// TestResyncDeliveryNoReceiverMarksRunFailed proves delivery is impossible
// without a configured webhook: the run must be marked failed, never
// completed, with a diagnostic.
func TestResyncDeliveryNoReceiverMarksRunFailed(t *testing.T) {
	resetPusherState()
	tightenResyncDeliveryBounds(t, 2)

	run := seedResyncDeliveryRun(t)
	seedResyncDeliveryEvent(t, run, 1)

	saved := CurrentConfig
	CurrentConfig = nil
	t.Cleanup(func() { CurrentConfig = saved })

	require.NoError(t, completeResyncDelivery(context.Background(), models.DB, run))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run.ID))
	assert.Equal(t, "failed", reloaded.Status)
	assert.Equal(t, 0, reloaded.EventsDelivered)
	assert.Equal(t, 1, reloaded.EventsFailed)
	assert.Contains(t, reloaded.Errors, "delivery incomplete")
}

// TestResyncDeliveryNothingCreatedCompletes proves a run that skipped every
// animal (no events created) still completes.
func TestResyncDeliveryNothingCreatedCompletes(t *testing.T) {
	resetPusherState()
	tightenResyncDeliveryBounds(t, 2)

	run := seedResyncDeliveryRun(t)
	require.NoError(t, completeResyncDelivery(context.Background(), models.DB, run))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run.ID))
	assert.Equal(t, "completed", reloaded.Status)
	assert.Equal(t, 0, reloaded.EventsDelivered)
	assert.Equal(t, 0, reloaded.EventsFailed)
}
