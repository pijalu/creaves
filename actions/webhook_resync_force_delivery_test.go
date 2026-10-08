//go:build sqlite
// +build sqlite

package actions

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// BUG-7 regression: a force full rebuild re-queues events whose per-target
// event_deliveries rows still say "delivered". The pusher's queue query skips
// such events entirely, so the advertised disaster-recovery path (console-side
// cleanup → force resync) silently dropped most of the backlog. The force
// re-queue must reset the delivery rows of EVERY enabled target (and only the
// enabled ones), after which the standard delivery loop must redeliver the
// event to all of them.

// countEventDeliveries counts how many events with the given ID were received
// across all recorded requests (event IDs are unique per body occurrence).
func countEventDeliveries(rr *recordingReceiver, eventID string) int {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	n := 0
	for _, req := range rr.requests {
		n += strings.Count(string(req.body), eventID)
	}
	return n
}

func stateEventForAnimal(t *testing.T, instanceID string, animalID int) *models.EventStream {
	t.Helper()
	ev := &models.EventStream{}
	require.NoError(t, models.DB.
		Where("instance_id = ? AND animal_id = ? AND event_type = ?",
			instanceID, animalID, string(models.EventTypeAnimalState)).
		First(ev))
	return ev
}

func TestForceResyncResetsDeliveryRowsAndRedelivers(t *testing.T) {
	resetPusherState()
	seedResyncNestedFixture(t)

	const instanceID = "rsync-resync-test"

	rrA := newRecordingReceiver(http.StatusOK)
	srvA := httptest.NewServer(http.HandlerFunc(rrA.handler))
	defer srvA.Close()
	rrB := newRecordingReceiver(http.StatusOK)
	srvB := httptest.NewServer(http.HandlerFunc(rrB.handler))
	defer srvB.Close()
	rrDisabled := newRecordingReceiver(http.StatusOK)
	srvDisabled := httptest.NewServer(http.HandlerFunc(rrDisabled.handler))
	defer srvDisabled.Close()

	seedPusherConfig(t, srvA.URL)
	targetB := seedSyncTarget(t, "target-b", srvB.URL, true)
	seedSyncTarget(t, "target-disabled", srvDisabled.URL, false)

	// Run 1 — plain (non-force) resync: creates the state event and delivers
	// it to both enabled targets.
	run1 := &models.ResyncRun{
		ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID,
		Status: "running", StartedAt: time.Now(), TotalAnimals: 1,
	}
	require.NoError(t, models.DB.Create(run1))
	require.NoError(t, RunResync(context.Background(), models.DB, run1.ID, false))

	ev := stateEventForAnimal(t, instanceID, 985049)
	targetA := reloadPusherTarget(t, "test-target")
	require.NotNil(t, deliveryRow(t, ev.ID, targetA.ID).DeliveredAt, "run 1 must deliver to target A")
	require.NotNil(t, deliveryRow(t, ev.ID, targetB.ID).DeliveredAt, "run 1 must deliver to target B")
	assert.Zero(t, rrDisabled.totalEvents(), "disabled target must never receive anything")

	bRowBeforeForce := deliveryRow(t, ev.ID, targetB.ID)
	assert.Zero(t, bRowBeforeForce.Attempts, "sanity: successful deliveries do not accumulate attempts")

	// Disable target B BEFORE the force run: its delivered row must survive
	// the re-queue untouched (only ENABLED targets are re-delivered to), and
	// the event must still be re-delivered to the remaining enabled target.
	require.NoError(t, models.DB.RawQuery(
		"UPDATE sync_targets SET enabled = 0 WHERE id = ?", targetB.ID.String()).Exec())

	// Run 2 — FORCE resync: re-queues the existing event. Without the
	// event_deliveries reset this run stalls: the queue query joins the still-
	// delivered rows and finds nothing to send.
	run2 := &models.ResyncRun{
		ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID,
		Status: "running", StartedAt: time.Now(), TotalAnimals: 1,
	}
	require.NoError(t, models.DB.Create(run2))
	require.NoError(t, RunResync(context.Background(), models.DB, run2.ID, true))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run2.ID))
	assert.Equal(t, "completed", reloaded.Status,
		"force run must complete: the re-queued event must be deliverable again")
	assert.Equal(t, 1, reloaded.EventsCreated, "the existing event must be re-queued, not duplicated")
	assert.Equal(t, 1, reloaded.EventsDelivered, "the re-queued event must be delivered again")
	assert.Zero(t, reloaded.EventsFailed)

	// The event went to BOTH enabled targets again; the disabled one did not
	// receive a second copy.
	assert.Equal(t, 2, countEventDeliveries(rrA, ev.ID.String()),
		"target A must receive the event once per run")
	assert.Equal(t, 1, countEventDeliveries(rrB, ev.ID.String()),
		"target B must receive the event exactly once (it was disabled before the force run)")
	assert.Zero(t, rrDisabled.totalEvents(), "disabled target must stay silent")

	// The re-queued event is delivered again (rollup over enabled targets).
	redelivered := stateEventForAnimal(t, instanceID, 985049)
	require.NotNil(t, redelivered.DeliveredAt, "force re-queued event must be delivered again")

	// The disabled target's historical delivered row was NOT reset by the
	// force re-queue (same delivered_at as before the force run).
	bRowAfterForce := deliveryRow(t, ev.ID, targetB.ID)
	require.NotNil(t, bRowAfterForce.DeliveredAt)
	assert.Equal(t, bRowBeforeForce.DeliveredAt.Unix(), bRowAfterForce.DeliveredAt.Unix(),
		"force re-queue must not touch disabled targets' delivered rows")

	// The enabled target's delivery row was reset and then re-delivered:
	// it must be delivered again with a fresh acknowledgment-pending state.
	aRow := deliveryRow(t, ev.ID, targetA.ID)
	require.NotNil(t, aRow.DeliveredAt)
	assert.Nil(t, aRow.AcknowledgedAt, "the re-delivery must await a fresh console acknowledgement")
}

// TestForceResyncChangedPayloadCreatesFreshEvent pins the non-force-adjacent
// path: a force run on an event whose content hash DIFFERS creates a NEW
// event (new deterministic UUID) instead of re-queueing — its delivery rows
// do not exist yet, so every enabled target receives it without any reset.
func TestForceResyncChangedPayloadCreatesFreshEvent(t *testing.T) {
	resetPusherState()
	seedResyncNestedFixture(t)

	const instanceID = "rsync-resync-test"

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	// An event from an EARLIER state of animal 985049 (different content
	// hash, already delivered with a delivered delivery row).
	oldPayload := models.EventPayload{
		Timestamp:     time.RFC3339,
		CurrentStatus: "in_care",
		Animal:        models.AnimalPayload{ID: 985049, Species: "RSYNC_Hedgehog", Cage: "OLD-CAGE"},
	}
	oldHash := StateContentHashPayload(instanceID, oldPayload)
	oldPayload.StateHash = oldHash
	oldEvent := &models.EventStream{
		ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID, AnimalID: 985049,
		EventType: string(models.EventTypeAnimalState), ContentHash: &oldHash,
		DeliveredAt: ptrTime(time.Now().Add(-time.Hour)),
	}
	require.NoError(t, oldEvent.SetPayload(oldPayload))
	require.NoError(t, models.DB.Create(oldEvent))
	target := reloadPusherTarget(t, "test-target")
	require.NoError(t, models.DB.RawQuery(
		"INSERT INTO event_deliveries (id, event_id, target_id, attempts, delivered_at, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?, ?)",
		uuid.Must(uuid.NewV4()).String(), oldEvent.ID.String(), target.ID.String(),
		time.Now().Add(-time.Hour), time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)).Exec())

	// Force resync: the current state of the fixture animal has no outtake-
	// type-identified cage "OLD-CAGE", so the freshly built payload hashes
	// differently → a NEW event must be created and delivered.
	run := &models.ResyncRun{
		ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID,
		Status: "running", StartedAt: time.Now(), TotalAnimals: 1,
	}
	require.NoError(t, models.DB.Create(run))
	require.NoError(t, RunResync(context.Background(), models.DB, run.ID, true))

	reloaded := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(reloaded, run.ID))
	assert.Equal(t, "completed", reloaded.Status)
	assert.Equal(t, 1, reloaded.EventsCreated, "changed content must create a new event")

	// Two events now exist for the animal: the stale one (old hash) and the
	// freshly created one (current hash). Both must be delivered.
	var rows []struct {
		ID          string     `db:"id"`
		ContentHash *string    `db:"content_hash"`
		DeliveredAt *time.Time `db:"delivered_at"`
	}
	require.NoError(t, models.DB.RawQuery(
		"SELECT id, content_hash, delivered_at FROM event_streams WHERE instance_id = ? AND animal_id = 985049",
		instanceID).All(&rows))
	require.Len(t, rows, 2)
	for _, row := range rows {
		require.NotNil(t, row.DeliveredAt, "event %s must be delivered", row.ID)
	}
	hashes := hashList(rows)
	assert.Len(t, uniq(hashes), 2, "the force run must add a second, differently-hashed event")
	assert.Contains(t, hashes, oldHash, "the stale event must remain untouched")

	// The old event's delivery row was untouched by the force run (the reset
	// predicate only matches the re-queued event's hash).
	var oldDeliveries []struct {
		DeliveredAt *time.Time `db:"delivered_at"`
		Attempts    int        `db:"attempts"`
	}
	require.NoError(t, models.DB.RawQuery(
		"SELECT delivered_at, attempts FROM event_deliveries WHERE event_id = ?",
		oldEvent.ID.String()).All(&oldDeliveries))
	require.Len(t, oldDeliveries, 1)
	require.NotNil(t, oldDeliveries[0].DeliveredAt, "old event's delivered row must stay delivered")
	assert.Zero(t, oldDeliveries[0].Attempts)
}

func hashList(rows []struct {
	ID          string     `db:"id"`
	ContentHash *string    `db:"content_hash"`
	DeliveredAt *time.Time `db:"delivered_at"`
}) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		if r.ContentHash != nil {
			out = append(out, *r.ContentHash)
		}
	}
	return out
}

func uniq(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}

func ptrTime(t time.Time) *time.Time { return &t }
