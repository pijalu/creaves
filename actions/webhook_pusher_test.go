//go:build sqlite
// +build sqlite

package actions

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/events"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pusherTestDB holds the test database connection for the creaves pusher tests.
var pusherTestDB *pop.Connection

// TestMain sets up a SQLite-backed test database for the webhook pusher tests.
// The pusher reads undelivered events from models.DB and writes back delivery
// timestamps, so models.DB must point at a real (SQLite) connection compiled
// with the "sqlite" build tag.
func TestMain(m *testing.M) {
	var err error
	pusherTestDB, err = pop.NewConnection(
		&pop.ConnectionDetails{
			Dialect:  "sqlite",
			Driver:   driverNameCreavesSQLite,
			Database: "./pusher_test.db",
		})
	if err != nil {
		fmt.Printf("Failed to create test database: %v\n", err)
		os.Exit(1)
	}
	if err := pusherTestDB.Open(); err != nil {
		fmt.Printf("Failed to open test database: %v\n", err)
		os.Exit(1)
	}

	// The pusher uses the package-global models.DB.
	models.DB = pusherTestDB

	createPusherTables()
	createReferenceTables()

	code := m.Run()

	pusherTestDB.Close()
	os.Remove("./pusher_test.db")
	os.Exit(code)
}

func createPusherTables() {
	pusherTestDB.RawQuery(`
		CREATE TABLE IF NOT EXISTS event_streams (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			animal_id INTEGER NOT NULL,
			event_type TEXT NOT NULL,
			payload TEXT,
			processed_at TIMESTAMP,
			delivered_at TIMESTAMP,
			acknowledged_at TIMESTAMP,
			content_hash TEXT,
			resync_run_id TEXT,
			delivery_attempts INTEGER NOT NULL DEFAULT 0,
			last_delivery_error TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Exec()

	pusherTestDB.RawQuery(`
		CREATE TABLE IF NOT EXISTS resync_runs (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			status TEXT NOT NULL,
			started_at TIMESTAMP NOT NULL,
			finished_at TIMESTAMP,
			total_animals INTEGER NOT NULL DEFAULT 0,
			animals_processed INTEGER NOT NULL DEFAULT 0,
			events_created INTEGER NOT NULL DEFAULT 0,
			events_skipped_unchanged INTEGER NOT NULL DEFAULT 0,
			events_delivered INTEGER NOT NULL DEFAULT 0,
			events_failed INTEGER NOT NULL DEFAULT 0,
			announced_expected_total INTEGER NOT NULL DEFAULT 0,
			announced_expected_checksum TEXT,
			announced_at TIMESTAMP,
			errors TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Exec()

	pusherTestDB.RawQuery(`
		CREATE TABLE IF NOT EXISTS config (
			id TEXT PRIMARY KEY,
			instance_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT,
			active BOOLEAN DEFAULT 1,
			settings TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Exec()

	pusherTestDB.RawQuery(`
		CREATE TABLE IF NOT EXISTS sync_targets (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			enabled BOOLEAN NOT NULL DEFAULT 0,
			webhook_url TEXT NOT NULL DEFAULT '',
			webhook_api_key TEXT NOT NULL DEFAULT '',
			webhook_batch_size INTEGER NOT NULL DEFAULT 1,
			webhook_max_per_min INTEGER NOT NULL DEFAULT 60,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Exec()

	pusherTestDB.RawQuery(`
		CREATE TABLE IF NOT EXISTS event_deliveries (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL,
			target_id TEXT NOT NULL,
			attempts INTEGER NOT NULL DEFAULT 0,
			delivered_at TIMESTAMP,
			acknowledged_at TIMESTAMP,
			last_error TEXT,
			created_at TIMESTAMP NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		)
	`).Exec()
}

// resetPusherState clears DB tables and resets the package-global pusher
// (rate limiter counters + circuit breaker) between tests. It also stops the
// webhook worker so a leaked worker from a previous test cannot deliver
// events (or read config) mid-test.
func resetPusherState() {
	StopWebhookWorker()
	pusherTestDB.RawQuery("DELETE FROM event_streams").Exec()
	pusherTestDB.RawQuery("DELETE FROM resync_runs").Exec()
	pusherTestDB.RawQuery("DELETE FROM config").Exec()
	pusherTestDB.RawQuery("DELETE FROM sync_targets").Exec()
	pusherTestDB.RawQuery("DELETE FROM event_deliveries").Exec()
	SetSyncTargetsKnown(false)

	webhookPusher.mu.Lock()
	webhookPusher.targets = make(map[uuid.UUID]*targetDeliveryState)
	webhookPusher.mu.Unlock()
}

// seedPusherConfig installs CurrentConfig plus one enabled sync target
// pointing webhook delivery at url, and returns the target.
func seedPusherConfig(t *testing.T, url string) *models.SyncTarget {
	t.Helper()
	settings := models.ConfigSettings{EnableEventStream: true}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	require.NoError(t, pusherTestDB.Create(cfg))
	CurrentConfigSet(cfg)
	return seedSyncTarget(t, "test-target", url, true)
}

// seedSyncTarget inserts one sync target row (batch 10, max/min 100 to
// mirror the historical seedPusherConfig settings).
func seedSyncTarget(t *testing.T, name, url string, enabled bool) *models.SyncTarget {
	t.Helper()
	target := &models.SyncTarget{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             name,
		Enabled:          enabled,
		WebhookURL:       url,
		WebhookAPIKey:    "creaves_testkey",
		WebhookBatchSize: 10,
		WebhookMaxPerMin: 100,
	}
	require.NoError(t, pusherTestDB.Create(target))
	SetSyncTargetsKnown(true)
	// The known-targets flag is process-global: do not leak it into
	// non-pusher tests, where a wake would make the worker query models.DB.
	t.Cleanup(func() { SetSyncTargetsKnown(false) })
	return target
}

// deliverBatch is a test shim: deliver one batch to the single seeded
// target (kept so the historical single-target tests read unchanged).
func deliverBatch(t *testing.T) (int, error) {
	t.Helper()
	targets := models.SyncTargets{}
	require.NoError(t, pusherTestDB.Order("created_at").All(&targets))
	require.NotEmpty(t, targets, "deliverBatch shim requires a seeded sync target")
	return deliverTargetBatch(&targets[0])
}

// breakerFailures returns the failure count of the circuit breaker attached
// to the first seeded target (single-target test shorthand).
func breakerFailures(t *testing.T) int {
	t.Helper()
	targets := models.SyncTargets{}
	require.NoError(t, pusherTestDB.Order("created_at").All(&targets))
	require.NotEmpty(t, targets)
	st := webhookPusher.targetState(targets[0].ID)
	st.breaker.mu.Lock()
	defer st.breaker.mu.Unlock()
	return st.breaker.failures
}

// seedUndeliveredEvent inserts an event with delivered_at IS NULL.
func seedUndeliveredEvent(t *testing.T, animalID int) *models.EventStream {
	t.Helper()
	ev := &models.EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		AnimalID:   animalID,
		EventType:  string(models.EventTypeAnimalDiscovered),
		Payload:    []byte(`{"animal":{"species":"Fox"},"initial_status":"in_care"}`),
		CreatedAt:  time.Now(),
	}
	require.NoError(t, pusherTestDB.Create(ev))
	return ev
}

// ---------------------------------------------------------------------------
// CircuitBreaker unit tests (no DB required)
// ---------------------------------------------------------------------------

func TestCircuitBreaker_StartsClosed(t *testing.T) {
	cb := NewCircuitBreaker()
	assert.False(t, cb.IsOpen())
	assert.Equal(t, "closed", cb.state)
	assert.Equal(t, 5, cb.failureThreshold)
	assert.Equal(t, 60*time.Second, cb.resetTimeout)
}

func TestCircuitBreaker_OpensAfterThreshold(t *testing.T) {
	cb := NewCircuitBreaker()

	// Below threshold: stays closed.
	for i := 0; i < cb.failureThreshold-1; i++ {
		cb.RecordFailure()
		assert.False(t, cb.IsOpen(), "should remain closed after %d failures", i+1)
	}

	// At threshold: opens.
	cb.RecordFailure()
	assert.True(t, cb.IsOpen())
	assert.Equal(t, "open", cb.state)
}

func TestCircuitBreaker_RecordSuccessResets(t *testing.T) {
	cb := NewCircuitBreaker()
	cb.RecordFailure()
	cb.RecordFailure()
	cb.RecordSuccess()

	assert.Equal(t, 0, cb.failures)
	assert.Equal(t, "closed", cb.state)
	assert.False(t, cb.IsOpen())
}

func TestCircuitBreaker_HalfOpenAfterResetTimeout(t *testing.T) {
	cb := NewCircuitBreaker()
	// Force the breaker open.
	for i := 0; i < cb.failureThreshold; i++ {
		cb.RecordFailure()
	}
	require.True(t, cb.IsOpen())

	// Simulate the reset timeout elapsing by backdating lastFailure.
	cb.mu.Lock()
	cb.lastFailure = time.Now().Add(-(cb.resetTimeout + time.Second))
	cb.mu.Unlock()

	// First probe after timeout trips half-open and returns false (allow probe).
	assert.False(t, cb.IsOpen())
	cb.mu.Lock()
	assert.Equal(t, "half-open", cb.state)
	cb.mu.Unlock()

	// A success while half-open closes the circuit.
	cb.RecordSuccess()
	assert.Equal(t, "closed", cb.state)
	assert.False(t, cb.IsOpen())
}

// ---------------------------------------------------------------------------
// Rate limiter (allowDelivery) unit tests
// ---------------------------------------------------------------------------

func TestWebhookPusher_AllowDeliveryRateLimits(t *testing.T) {
	resetPusherState()
	target := seedPusherConfig(t, "http://unused.example")

	state := webhookPusher.targetState(target.ID)
	maxPerMin := target.EffectiveMaxPerMin()
	batchSize := target.EffectiveBatchSize()

	// Consume the budget.
	allowed := 0
	for i := 0; i < maxPerMin; i++ {
		if !state.allowDelivery(maxPerMin, batchSize) {
			break
		}
		allowed++
	}
	assert.Greater(t, allowed, 0)
	// Budget exhausted -> further delivery denied.
	assert.False(t, state.allowDelivery(maxPerMin, batchSize))
}

// TestWebhookPusher_RateLimitIsPerTarget proves one target's exhausted
// budget does not throttle another target.
func TestWebhookPusher_RateLimitIsPerTarget(t *testing.T) {
	resetPusherState()
	targetA := seedSyncTarget(t, "target-a", "http://a.example", true)
	targetB := seedSyncTarget(t, "target-b", "http://b.example", true)

	stateA := webhookPusher.targetState(targetA.ID)
	stateB := webhookPusher.targetState(targetB.ID)

	// Exhaust A's budget.
	for stateA.allowDelivery(10, 10) {
	}
	assert.False(t, stateA.allowDelivery(10, 10), "A budget exhausted")
	assert.True(t, stateB.allowDelivery(10, 10), "B budget unaffected")
}

// ---------------------------------------------------------------------------
// deliverBatch integration tests (httptest webhook receiver)
// ---------------------------------------------------------------------------

// recordingReceiver is a minimal webhook receiver that records received
// requests so the pusher tests can assert on the wire payload.
type recordingReceiver struct {
	mu       sync.Mutex
	requests []receivedRequest
	status   int
}

type receivedRequest struct {
	auth  string
	body  []byte
	count int
}

func newRecordingReceiver(status int) *recordingReceiver {
	return &recordingReceiver{status: status}
}

func (rr *recordingReceiver) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	rr.mu.Lock()
	var payload struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	_ = json.Unmarshal(body, &payload)
	rr.requests = append(rr.requests, receivedRequest{
		auth:  r.Header.Get("Authorization"),
		body:  body,
		count: len(payload.Events),
	})
	rr.mu.Unlock()

	if rr.status == 0 {
		w.WriteHeader(http.StatusOK)
	} else {
		w.WriteHeader(rr.status)
	}
}

func (rr *recordingReceiver) totalEvents() int {
	rr.mu.Lock()
	defer rr.mu.Unlock()
	n := 0
	for _, req := range rr.requests {
		n += req.count
	}
	return n
}

func TestDeliverBatch_Success(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	ev := seedUndeliveredEvent(t, 1)
	ev2 := seedUndeliveredEvent(t, 2)

	_, err := deliverBatch(t)
	require.NoError(t, err)

	// Both events delivered to the receiver.
	assert.Equal(t, 2, rr.totalEvents())

	// Authorization header carries the configured bearer key.
	require.NotEmpty(t, rr.requests)
	assert.Equal(t, "Bearer creaves_testkey", rr.requests[0].auth)

	// Events marked delivered in the DB.
	for _, id := range []uuid.UUID{ev.ID, ev2.ID} {
		var got models.EventStream
		require.NoError(t, pusherTestDB.Find(&got, id))
		assert.NotNil(t, got.DeliveredAt, "event %s should be marked delivered", id)
	}
}

// ---------------------------------------------------------------------------
// Multi-hub fan-out: one event must reach EVERY enabled target, with one
// delivery row per (event, target); the legacy event_streams.delivered_at
// rollup is set only once all enabled targets accepted the event.
// ---------------------------------------------------------------------------

// TestFanOut_DeliversToEveryEnabledTarget seeds two receiving targets and
// proves one event is POSTed to both, with per-target delivery rows and the
// rollup set after the second target accepts.
func TestFanOut_DeliversToEveryEnabledTarget(t *testing.T) {
	resetPusherState()

	rrA := newRecordingReceiver(http.StatusOK)
	srvA := httptest.NewServer(http.HandlerFunc(rrA.handler))
	defer srvA.Close()
	rrB := newRecordingReceiver(http.StatusOK)
	srvB := httptest.NewServer(http.HandlerFunc(rrB.handler))
	defer srvB.Close()

	seedPusherConfig(t, srvA.URL)
	targetB := seedSyncTarget(t, "target-b", srvB.URL, true)
	// One disabled target must be skipped entirely.
	seedSyncTarget(t, "target-disabled", "http://127.0.0.1:1/disabled", false)

	ev := seedUndeliveredEvent(t, 1)

	// Fan out one round to every deliverable target (the worker path).
	// (Return value only signals "a full batch was sent, more may pend" —
	// with a single event it is false even on success.)
	deliverPendingBatch()

	// Both enabled receivers got the event; the disabled one got nothing.
	assert.Equal(t, 1, rrA.totalEvents(), "target A received the event")
	assert.Equal(t, 1, rrB.totalEvents(), "target B received the event")

	// One delivery row per enabled target, both delivered.
	targetA := reloadPusherTarget(t, "test-target")
	rowA := deliveryRow(t, ev.ID, targetA.ID)
	assert.NotNil(t, rowA.DeliveredAt)
	rowB := deliveryRow(t, ev.ID, targetB.ID)
	assert.NotNil(t, rowB.DeliveredAt)

	// Rollup: all enabled targets delivered -> event_streams.delivered_at set.
	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.NotNil(t, got.DeliveredAt, "rollup set once every enabled target delivered")
}

// TestFanOut_RollupWaitsForSlowTarget proves the legacy rollup stays NULL
// while one enabled target has not accepted the event yet, and is set once
// the straggler catches up.
func TestFanOut_RollupWaitsForSlowTarget(t *testing.T) {
	resetPusherState()

	rrA := newRecordingReceiver(http.StatusOK)
	srvA := httptest.NewServer(http.HandlerFunc(rrA.handler))
	defer srvA.Close()
	// Target B rejects everything (500) until flipped.
	flaky := &flakyReceiver{status: http.StatusInternalServerError}
	srvB := httptest.NewServer(http.HandlerFunc(flaky.handler))
	defer srvB.Close()

	seedPusherConfig(t, srvA.URL)
	targetB := seedSyncTarget(t, "target-b", srvB.URL, true)

	ev := seedUndeliveredEvent(t, 1)

	deliverPendingBatch()

	// A delivered; B failed -> no rollup yet.
	targetA := reloadPusherTarget(t, "test-target")
	assert.NotNil(t, deliveryRow(t, ev.ID, targetA.ID).DeliveredAt)
	rowB := deliveryRow(t, ev.ID, targetB.ID)
	assert.Nil(t, rowB.DeliveredAt)
	assert.Equal(t, 1, rowB.Attempts, "failed attempt recorded for B")

	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.Nil(t, got.DeliveredAt, "rollup waits for every enabled target")

	// B recovers: next fan-out round delivers the straggler and the rollup
	// appears.
	flaky.status = http.StatusOK
	deliverPendingBatch()

	assert.NotNil(t, deliveryRow(t, ev.ID, targetB.ID).DeliveredAt)
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.NotNil(t, got.DeliveredAt, "rollup set once B caught up")
}

// flakyReceiver answers every POST with a mutable status code and an empty
// body (full accept on 200).
type flakyReceiver struct {
	mu     sync.Mutex
	status int
}

func (fr *flakyReceiver) handler(w http.ResponseWriter, r *http.Request) {
	fr.mu.Lock()
	status := fr.status
	fr.mu.Unlock()
	w.WriteHeader(status)
}

// reloadPusherTarget fetches one sync target by name from the pusher DB.
func reloadPusherTarget(t *testing.T, name string) *models.SyncTarget {
	t.Helper()
	target := &models.SyncTarget{}
	require.NoError(t, pusherTestDB.Where("name = ?", name).First(target))
	return target
}

func TestDeliverBatch_FullCountResponseMarksEventsDelivered(t *testing.T) {
	resetPusherState()
	target := seedPusherConfig(t, "http://unused")
	ev := seedUndeliveredEvent(t, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"processed":1,"total":1}`))
	}))
	defer srv.Close()
	target.WebhookURL = srv.URL
	require.NoError(t, pusherTestDB.Update(target))
	_, err := deliverBatch(t)
	require.NoError(t, err)
	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.NotNil(t, got.DeliveredAt)
}

func TestDeliverBatch_ExplicitPartialResponseDoesNotDeliver(t *testing.T) {
	resetPusherState()
	target := seedPusherConfig(t, "http://unused")
	ev := seedUndeliveredEvent(t, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"processed":0,"total":1,"processed_ids":[],"errors":["failed"]}`))
	}))
	defer srv.Close()
	target.WebhookURL = srv.URL
	require.NoError(t, pusherTestDB.Update(target))
	_, err := deliverBatch(t)
	require.Error(t, err)
	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.Nil(t, got.DeliveredAt)
}

func TestDeliverBatch_NoEventsNoop(t *testing.T) {
	resetPusherState()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)

	_, err := deliverBatch(t)
	require.NoError(t, err)
	assert.False(t, called, "receiver must not be called when there are no events")
}

func TestDeliverBatch_NonOKStatusRecordsFailure(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusInternalServerError)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 1)

	before := breakerFailures(t)
	_, err := deliverBatch(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")

	// Circuit breaker incremented.
	assert.Equal(t, before+1, breakerFailures(t))

	// Event NOT marked delivered.
	var got models.EventStream
	require.NoError(t, pusherTestDB.Where("delivered_at IS NULL").First(&got))
}

func TestDeliverBatch_ConnectionErrorRecordsFailure(t *testing.T) {
	resetPusherState()

	// Point at a closed port to force a connection error.
	seedPusherConfig(t, "http://127.0.0.1:1/webhook")
	seedUndeliveredEvent(t, 1)

	before := breakerFailures(t)
	_, err := deliverBatch(t)
	require.Error(t, err)

	assert.Equal(t, before+1, breakerFailures(t))
}

func TestDeliverBatch_EnvelopeHasInstanceAndVersion(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	cfg := CurrentConfigGet()
	cfg.Description = "Wildlife care centre"
	seedUndeliveredEvent(t, 9)

	_, err := deliverBatch(t)
	require.NoError(t, err)
	require.Len(t, rr.requests, 1)

	var wire struct {
		ContractVersion int `json:"contract_version"`
		Instance        struct {
			ID          string `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"instance"`
	}
	require.NoError(t, json.Unmarshal(rr.requests[0].body, &wire))
	assert.Equal(t, 2, wire.ContractVersion)
	assert.Equal(t, "test-instance", wire.Instance.ID)
	assert.Equal(t, "Test", wire.Instance.Name)
	assert.Equal(t, "Wildlife care centre", wire.Instance.Description)
}

func TestDeliverBatch_PayloadShape(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 9)

	_, err := deliverBatch(t)
	require.NoError(t, err)

	require.Len(t, rr.requests, 1)
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.requests[0].body, &wire))
	events, ok := wire["events"].([]interface{})
	require.True(t, ok)
	require.Len(t, events, 1)
	first := events[0].(map[string]interface{})
	// Contract fields present and correctly typed.
	for _, k := range []string{"id", "instance_id", "animal_id", "event_type", "payload", "created_at"} {
		_, present := first[k]
		assert.True(t, present, "payload missing field %q", k)
	}
	assert.EqualValues(t, 9, first["animal_id"])
	assert.Equal(t, "test-instance", first["instance_id"])
	assert.Equal(t, "animal_discovered", first["event_type"])
}

// TestDeliverBatch_ResyncRunIDOnWire proves the contract v2 addition: an
// event linked to a resync run carries resync_run_id on the wire so the
// console can attribute it (bugs.md #9), and a live event omits the field.
func TestDeliverBatch_ResyncRunIDOnWire(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	event := seedUndeliveredEvent(t, 11)
	runID := uuid.Must(uuid.NewV4())
	event.ResyncRunID = &runID
	require.NoError(t, pusherTestDB.Update(event))

	_, err := deliverBatch(t)
	require.NoError(t, err)

	require.Len(t, rr.requests, 1)
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.requests[0].body, &wire))
	events, ok := wire["events"].([]interface{})
	require.True(t, ok)
	require.Len(t, events, 1)
	first := events[0].(map[string]interface{})
	assert.Equal(t, runID.String(), first["resync_run_id"],
		"resync-delivered event must carry resync_run_id on the wire")
}

// TestDeliverBatch_LiveEventOmitsResyncRunID proves live events do not carry
// a resync_run_id field.
func TestDeliverBatch_LiveEventOmitsResyncRunID(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 12)

	_, err := deliverBatch(t)
	require.NoError(t, err)

	require.Len(t, rr.requests, 1)
	var wire map[string]interface{}
	require.NoError(t, json.Unmarshal(rr.requests[0].body, &wire))
	events, ok := wire["events"].([]interface{})
	require.True(t, ok)
	require.Len(t, events, 1)
	first := events[0].(map[string]interface{})
	_, present := first["resync_run_id"]
	assert.False(t, present, "live event must omit resync_run_id")
}

// selectiveReceiver is a webhook receiver that acknowledges only a subset of
// the delivered events (by returning processed_ids for them). This mimics the
// real console receiver on partial failure, where some events are accepted and
// others are rejected.
type selectiveReceiver struct {
	acceptIDs map[string]bool
}

func (s *selectiveReceiver) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var payload struct {
		Events []struct {
			ID string `json:"id"`
		} `json:"events"`
	}
	_ = json.Unmarshal(body, &payload)

	processed := make([]string, 0, len(payload.Events))
	for _, e := range payload.Events {
		if s.acceptIDs[e.ID] {
			processed = append(processed, e.ID)
		}
	}
	resp := map[string]interface{}{
		"processed":     len(processed),
		"total":         len(payload.Events),
		"processed_ids": processed,
	}
	out, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(out)
}

// TestDeliverBatch_PartialFailureMarksOnlyAccepted proves the partial-failure
// contract: when the receiver reports processed_ids for a subset of the batch,
// the pusher marks ONLY those events as delivered. The un-acknowledged events
// remain undelivered (delivered_at IS NULL) so they are retried on the next
// tick instead of being silently dropped.
func TestDeliverBatch_PartialFailureMarksOnlyAccepted(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc((&selectiveReceiver{
		acceptIDs: map[string]bool{},
	}).handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)

	// Three events; only ev2 and ev3 are accepted by the receiver.
	ev1 := seedUndeliveredEvent(t, 1)
	ev2 := seedUndeliveredEvent(t, 2)
	ev3 := seedUndeliveredEvent(t, 3)

	// Reconfigure the receiver to accept only ev2 and ev3.
	acc := &selectiveReceiver{acceptIDs: map[string]bool{
		ev2.ID.String(): true,
		ev3.ID.String(): true,
	}}
	srv.Config.Handler = http.HandlerFunc(acc.handler)

	_, err := deliverBatch(t)
	// Partial acceptance is reported as an error (circuit breaker records a
	// failure) since commit be7bec9 — but the accepted subset must still be
	// marked delivered and the rejected event left pending for retry.
	require.Error(t, err)

	// ev1 (rejected) stays undelivered for retry.
	var got1 models.EventStream
	require.NoError(t, pusherTestDB.Find(&got1, ev1.ID))
	assert.Nil(t, got1.DeliveredAt, "rejected event must remain undelivered")

	// ev2 and ev3 (accepted) are marked delivered.
	for _, id := range []uuid.UUID{ev2.ID, ev3.ID} {
		var got models.EventStream
		require.NoError(t, pusherTestDB.Find(&got, id))
		assert.NotNil(t, got.DeliveredAt, "accepted event %s must be marked delivered", id)
	}
}

// TestDeliverBatch_PartialAcceptRecordsBreakerFailure pins the deliberate
// circuit-breaker contract for partial acceptance: a response that accepts
// only a subset of the batch counts as a delivery failure for the circuit
// breaker, even though the accepted subset is persisted as delivered. A
// receiver that degrades to accepting nothing still opens the circuit after
// failureThreshold consecutive partial responses instead of being hammered
// every wake.
func TestDeliverBatch_PartialAcceptRecordsBreakerFailure(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc((&selectiveReceiver{
		acceptIDs: map[string]bool{},
	}).handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 1)

	before := breakerFailures(t)
	_, err := deliverBatch(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "accepted 0/1 events")
	assert.Equal(t, before+1, breakerFailures(t),
		"partial acceptance must record a circuit-breaker failure")

	// A subsequent full success resets the breaker (RecordSuccess path is
	// only reached on non-partial responses).
	srv.Config.Handler = http.HandlerFunc(newRecordingReceiver(http.StatusOK).handler)
	seedUndeliveredEvent(t, 2)
	_, err = deliverBatch(t)
	require.NoError(t, err)
	assert.Equal(t, 0, breakerFailures(t),
		"full success must reset the circuit breaker")
}

// TestEnsureWebhookWorkerRunning_StartsWhenEnabled proves the boot path: with
// a config that has webhook forwarding enabled, EnsureWebhookWorkerRunning
// starts the background worker. This is the fix for "webhook worker not
// started at boot".
func TestEnsureWebhookWorkerRunning_StartsWhenEnabled(t *testing.T) {
	resetPusherState()
	// Make sure no worker is left running from a previous test.
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// Seed an enabled webhook config and load it into CurrentConfig.
	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	EnsureWebhookWorkerRunning()
	assert.True(t, IsWebhookWorkerRunning(), "worker should start when webhook is enabled")

	// Calling again must be idempotent (no second worker / no panic).
	EnsureWebhookWorkerRunning()
	assert.True(t, IsWebhookWorkerRunning())

	// Clean up: stop the worker so it does not leak into other tests.
	StopWebhookWorker()
	assert.False(t, IsWebhookWorkerRunning())
}

// TestEnsureWebhookWorkerRunning_StartsWhenDisabled verifies the worker IS
// started even when webhook forwarding is disabled: the worker also performs
// the hourly event purge, so it must run regardless. Delivery itself remains
// gated per-tick by IsWebhookEnabled().
func TestEnsureWebhookWorkerRunning_StartsWhenDisabled(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// Config with event stream on but no enabled sync targets.
	settings := models.ConfigSettings{EnableEventStream: true}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	CurrentConfigSet(cfg)

	EnsureWebhookWorkerRunning()
	assert.True(t, IsWebhookWorkerRunning(), "worker must start even when webhook is disabled (purge duty)")

	StopWebhookWorker()
}

// newEmptyDB creates a fresh in-memory SQLite connection with NO tables, so
// any query against it (e.g. LoadConfig, tx.Create) fails. The caller must
// close the returned connection.
func newEmptyDB(t *testing.T) *pop.Connection {
	t.Helper()
	conn, err := pop.NewConnection(&pop.ConnectionDetails{
		Dialect:  "sqlite",
		Database: ":memory:",
	})
	require.NoError(t, err)
	require.NoError(t, conn.Open())
	return conn
}

// TestStartWebhookWorker_IdempotentGuard verifies the early-return guard: a
// second call to StartWebhookWorker while the worker is already running must
// be a no-op (not a panic, not a second worker).
func TestStartWebhookWorker_IdempotentGuard(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// First call starts the worker.
	StartWebhookWorker()
	assert.True(t, IsWebhookWorkerRunning())

	// Second call hits the "webhookTicker != nil" guard and returns early.
	StartWebhookWorker()
	assert.True(t, IsWebhookWorkerRunning())

	StopWebhookWorker()
	assert.False(t, IsWebhookWorkerRunning())
}

// TestRegisterWebhookShutdown_StopsOnAppStop verifies the process-level
// shutdown listener: when a buffalo.EvtAppStop event is emitted, the listener
// registered by RegisterWebhookShutdown stops the background worker.
func TestRegisterWebhookShutdown_StopsOnAppStop(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// Start the worker so the shutdown listener has something to stop.
	StartWebhookWorker()
	require.True(t, IsWebhookWorkerRunning())

	// Register the shutdown listener (the *buffalo.App param is unused by
	// RegisterWebhookShutdown, so nil is safe).
	RegisterWebhookShutdown(nil)

	// Emit an app-stop event. The events manager dispatches asynchronously
	// (each listener runs in its own goroutine), so we poll briefly.
	require.NoError(t, events.Emit(events.Event{
		Kind: buffalo.EvtAppStop,
	}))

	// The listener should stop the worker within a short window.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !IsWebhookWorkerRunning() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	assert.False(t, IsWebhookWorkerRunning(), "worker should stop on EvtAppStop")
}

// TestStartWebhookWorker_WakeDeliversEvents verifies the background worker
// goroutine inside StartWebhookWorker: when the worker is running with an
// enabled webhook config, a real test receiver, and pending undelivered
// events, a wake signal delivers the batch immediately (the fallback ticker
// now fires only every 60s). This covers the worker loop path
// (IsWebhookEnabled, circuit-breaker, allowDelivery, deliverBatch) which the
// fast start/stop tests cannot reach.
func TestStartWebhookWorker_WakeDeliversEvents(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 42)

	StartWebhookWorker()
	defer StopWebhookWorker()
	require.True(t, IsWebhookWorkerRunning())

	signalWebhookWake()

	// Delivery must happen near-immediately, far below the 60s tick.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if rr.totalEvents() > 0 {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	assert.Greater(t, rr.totalEvents(), 0, "wake signal should have delivered the pending event")
}

// ---------------------------------------------------------------------------
// purgeOldEvents tests (event retention: 1 day delivered / 7 days undelivered)
// ---------------------------------------------------------------------------

// seedEventAt inserts an event with explicit created_at / delivered_at
// timestamps so retention boundaries can be tested deterministically.
func seedEventAt(t *testing.T, animalID int, createdAt time.Time, deliveredAt *time.Time) uuid.UUID {
	t.Helper()
	id := uuid.Must(uuid.NewV4())
	var delivered interface{}
	if deliveredAt != nil {
		delivered = *deliveredAt
	}
	require.NoError(t, pusherTestDB.RawQuery(
		`INSERT INTO event_streams (id, instance_id, animal_id, event_type, payload, delivered_at, created_at)
		 VALUES (?, 'test-instance', ?, 'animal_discovered', '{}', ?, ?)`,
		id.String(), animalID, delivered, createdAt,
	).Exec())
	return id
}

func eventExists(t *testing.T, id uuid.UUID) bool {
	t.Helper()
	var count int
	require.NoError(t, pusherTestDB.RawQuery(
		"SELECT COUNT(*) FROM event_streams WHERE id = ?", id.String(),
	).First(&count))
	return count == 1
}

// TestPurgeOldEvents verifies the retention policy: delivered events older
// than 1 day and undelivered events older than 7 days are deleted, while
// everything newer is kept.
func TestPurgeOldEvents(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	now := time.Now()
	recentDelivered := now.Add(-2 * time.Hour)
	oldDelivered := now.Add(-25 * time.Hour)
	recentUndelivered := now.Add(-3 * 24 * time.Hour) // 3 days: kept
	oldUndelivered := now.Add(-8 * 24 * time.Hour)    // 8 days: purged

	keepDelivered := seedEventAt(t, 1, now.Add(-26*time.Hour), &recentDelivered)
	purgeDelivered := seedEventAt(t, 2, now.Add(-26*time.Hour), &oldDelivered)
	keepUndelivered := seedEventAt(t, 3, recentUndelivered, nil)
	purgeUndelivered := seedEventAt(t, 4, oldUndelivered, nil)

	require.NoError(t, purgeOldEvents())

	assert.True(t, eventExists(t, keepDelivered), "recently delivered event must be kept")
	assert.False(t, eventExists(t, purgeDelivered), "event delivered >1 day ago must be purged")
	assert.True(t, eventExists(t, keepUndelivered), "undelivered event <7 days old must be kept")
	assert.False(t, eventExists(t, purgeUndelivered), "undelivered event >7 days old must be purged")
}

// TestPurgeOldEvents_EmptyTable verifies purge is a no-op on an empty table.
func TestPurgeOldEvents_EmptyTable(t *testing.T) {
	resetPusherState()
	require.NoError(t, purgeOldEvents())
}

// ---------------------------------------------------------------------------
// Wake-signal (event-driven delivery) tests
// ---------------------------------------------------------------------------

// TestSignalWebhookWake_NoWorkerIsNoop proves the signal helper is safe to
// call when no worker is running (nil channel) and does not panic or block.
func TestSignalWebhookWake_NoWorkerIsNoop(t *testing.T) {
	resetPusherState()
	StopWebhookWorker() // ensure stopped
	signalWebhookWake()
	signalWebhookWake()
}

// TestSignalWebhookWake_Debounce proves the cap-1 wake channel collapses a
// burst of signals into at most one pending wake.
func TestSignalWebhookWake_Debounce(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.NoError(t, func() error { StartWebhookWorker(); return nil }())
	defer StopWebhookWorker()

	// Signal a burst; channel capacity is 1 so exactly one wake is pending.
	for i := 0; i < 5; i++ {
		signalWebhookWake()
	}

	pusherMu.Lock()
	wake := webhookWakeCh
	pusherMu.Unlock()
	require.NotNil(t, wake)

	assert.Equal(t, 1, len(wake), "cap-1 channel must hold exactly one pending wake")

	// Drain it for the stopped-receiver path below.
	select {
	case <-wake:
	default:
	}
}

// TestWorkerLoop_WakeDeliversImmediately proves a published event is picked
// up by the worker via the wake signal, without waiting for the 60s fallback
// tick.
func TestWorkerLoop_WakeDeliversImmediately(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	ev := seedUndeliveredEvent(t, 1)

	StartWebhookWorker()
	defer StopWebhookWorker()

	signalWebhookWake()

	// Poll briefly: delivery must happen far below the 60s fallback interval.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		var got models.EventStream
		require.NoError(t, pusherTestDB.Find(&got, ev.ID))
		if got.DeliveredAt != nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.NotNil(t, got.DeliveredAt, "wake signal must trigger near-immediate delivery")
	assert.Equal(t, 1, rr.totalEvents())
}

// TestWorkerLoop_WakeDrainsBurst proves one wake drains a multi-batch burst:
// 25 events with batch size 10 must be delivered in a single wake (3 batches).
func TestWorkerLoop_WakeDrainsBurst(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL) // batch size 10, max/min 100

	const total = 25
	for i := 0; i < total; i++ {
		seedUndeliveredEvent(t, 100+i)
	}

	StartWebhookWorker()
	defer StopWebhookWorker()

	signalWebhookWake()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rr.totalEvents() >= total {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	assert.Equal(t, total, rr.totalEvents(), "one wake must drain all %d events across multiple batches", total)

	var pending int
	require.NoError(t, pusherTestDB.RawQuery("SELECT COUNT(*) FROM event_streams WHERE delivered_at IS NULL").First(&pending))
	assert.Equal(t, 0, pending)
}

// TestDeliverPendingBatch_ReturnsTrueOnlyWhenFullBatch checks the drain-loop
// continuation contract: true on a full batch, false on empty/partial.
func TestDeliverPendingBatch_ReturnsTrueOnlyWhenFullBatch(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL) // batch size 10

	// Empty: false.
	assert.False(t, deliverPendingBatch())

	// Partial (3 < 10): false after delivering.
	for i := 0; i < 3; i++ {
		seedUndeliveredEvent(t, 200+i)
	}
	assert.False(t, deliverPendingBatch())
	assert.Equal(t, 3, rr.totalEvents())

	// Full (12 > 10): first call true (delivers 10), second false (delivers 2).
	for i := 0; i < 12; i++ {
		seedUndeliveredEvent(t, 300+i)
	}
	assert.True(t, deliverPendingBatch(), "full batch must signal more pending")
	assert.False(t, deliverPendingBatch())
	assert.Equal(t, 3+12, rr.totalEvents())
}

// ---------------------------------------------------------------------------
// Console acknowledgements + producer sync announcement (checksum fix)
// ---------------------------------------------------------------------------

// seedAckableStateEvent seeds an undelivered animal_state event with the
// given content hash (the hash the console must echo to confirm it).
func seedAckableStateEvent(t *testing.T, animalID int, hash string, resyncRunID *uuid.UUID) *models.EventStream {
	t.Helper()
	ev := &models.EventStream{
		ID:          uuid.Must(uuid.NewV4()),
		InstanceID:  "test-instance",
		AnimalID:    animalID,
		EventType:   string(models.EventTypeAnimalState),
		Payload:     []byte(`{"animal":{"species":"Fox"},"state_hash":"` + hash + `"}`),
		ContentHash: &hash,
		ResyncRunID: resyncRunID,
		CreatedAt:   time.Now(),
	}
	require.NoError(t, pusherTestDB.Create(ev))
	return ev
}

// TestDeliverBatch_ConfirmedAcksMarkAcknowledged proves the ack round-trip:
// a console response echoing processed_ids plus confirmed (id + stored state
// hash) sets acknowledged_at on the matching events. The acknowledged count
// feeds "Delivered & current" on /webhook_resync.
func TestDeliverBatch_ConfirmedAcksMarkAcknowledged(t *testing.T) {
	resetPusherState()

	matching := seedAckableStateEvent(t, 71, "hash-current", nil)
	stale := seedAckableStateEvent(t, 72, "hash-stale", nil)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		type confirmation struct {
			ID        string `json:"id"`
			StateHash string `json:"state_hash"`
		}
		confirmed := []confirmation{}
		for _, e := range payload.Events {
			// The console echoes the hash it stored: correct for the
			// current-hash event, a stale value for the other one.
			hash := "hash-current"
			if e.ID == stale.ID.String() {
				hash = "hash-outdated"
			}
			confirmed = append(confirmed, confirmation{ID: e.ID, StateHash: hash})
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"processed":     len(payload.Events),
			"total":         len(payload.Events),
			"processed_ids": []string{},
			"confirmed":     confirmed,
		})
	}))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	_, err := deliverBatch(t)
	require.NoError(t, err)

	var gotMatching, gotStale models.EventStream
	require.NoError(t, pusherTestDB.Find(&gotMatching, matching.ID))
	require.NoError(t, pusherTestDB.Find(&gotStale, stale.ID))
	assert.NotNil(t, gotMatching.DeliveredAt)
	assert.NotNil(t, gotMatching.AcknowledgedAt, "echoed matching hash must acknowledge the event")
	assert.NotNil(t, gotStale.DeliveredAt)
	assert.Nil(t, gotStale.AcknowledgedAt, "stale echoed hash must NOT acknowledge the event")
}

// TestDeliverBatch_AttachesResyncAnnouncement proves the producer announces
// its expected sync state (total + checksum) in envelopes that belong to a
// resync run — the console stores this and displays stored/announced(expected).
func TestDeliverBatch_AttachesResyncAnnouncement(t *testing.T) {
	resetPusherState()

	runID := uuid.Must(uuid.NewV4())
	announcedAt := time.Now().UTC().Truncate(time.Second)
	require.NoError(t, pusherTestDB.RawQuery(
		"INSERT INTO resync_runs (id, instance_id, status, started_at, created_at, updated_at, announced_expected_total, announced_expected_checksum, announced_at, errors) VALUES (?, 'test-instance', 'running', ?, ?, ?, 42, 'sha256:announce42', ?, '')",
		runID, time.Now(), time.Now(), time.Now(), announcedAt,
	).Exec())

	ev := seedAckableStateEvent(t, 5, "hash-r", &runID)

	var mu sync.Mutex
	var envelope map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &envelope)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"processed":1,"total":1,"processed_ids":["` + ev.ID.String() + `"]}`))
	}))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	_, err := deliverBatch(t)
	require.NoError(t, err)
	require.NotNil(t, envelope, "receiver must have been called")

	var wire struct {
		ContractVersion int `json:"contract_version"`
		Sync            *struct {
			ExpectedTotal    int        `json:"expected_total"`
			ExpectedChecksum string     `json:"expected_checksum"`
			AnnouncedAt      *time.Time `json:"announced_at"`
		} `json:"sync"`
	}
	require.NoError(t, json.Unmarshal(mustLock(&mu, envelope), &wire))
	assert.Equal(t, 2, wire.ContractVersion)
	require.NotNil(t, wire.Sync, "resync batch must carry the sync announcement block")
	assert.Equal(t, 42, wire.Sync.ExpectedTotal)
	assert.Equal(t, "sha256:announce42", wire.Sync.ExpectedChecksum)
	require.NotNil(t, wire.Sync.AnnouncedAt)
	assert.True(t, wire.Sync.AnnouncedAt.Equal(announcedAt), "announced_at: %v vs %v", wire.Sync.AnnouncedAt, announcedAt)
}

// TestDeliverBatch_NoAnnouncementForRegularEvents proves regular (non-resync)
// deliveries do not carry a sync block: the announcement is the resync run's
// job, so the console's stored announcement only refreshes with resyncs.
func TestDeliverBatch_NoAnnouncementForRegularEvents(t *testing.T) {
	resetPusherState()

	ev := seedAckableStateEvent(t, 6, "hash-plain", nil)

	var mu sync.Mutex
	var envelope map[string]json.RawMessage
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		_ = json.Unmarshal(body, &envelope)
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"processed":1,"total":1,"processed_ids":["` + ev.ID.String() + `"]}`))
	}))
	defer srv.Close()
	seedPusherConfig(t, srv.URL)

	_, err := deliverBatch(t)
	require.NoError(t, err)

	require.NotNil(t, envelope)
	_, hasSync := envelope["sync"]
	assert.False(t, hasSync, "non-resync batches must not carry a sync block")
}

// mustLock serializes map access for the receiver goroutine in the tests
// above (deliverBatch runs on the test goroutine, but keep it honest).
func mustLock(mu *sync.Mutex, m map[string]json.RawMessage) []byte {
	mu.Lock()
	defer mu.Unlock()
	data, err := json.Marshal(m)
	if err != nil {
		panic(err)
	}
	return data
}

// ---------------------------------------------------------------------------
// Actionable delivery error messages (bug 5)
// ---------------------------------------------------------------------------

func TestDeliverBatch_NonOKStatusIncludesBodyExcerpt(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"detail":"boom: invalid api key"}`))
	}))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
	assert.Contains(t, err.Error(), "boom: invalid api key")
	assert.Contains(t, err.Error(), srv.URL, "error must name the failing URL")
}

func TestDeliverBatch_PartialFailureIncludesReceiverErrors(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"processed":0,"total":1,"processed_ids":[],"errors":["event abc: instance block mismatch"]}`))
	}))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "0/1")
	assert.Contains(t, err.Error(), "instance block mismatch")
	assert.Contains(t, err.Error(), srv.URL, "error must name the failing URL")
}

func TestDeliverBatch_TransportErrorIncludesURL(t *testing.T) {
	resetPusherState()

	// Closed port forces a connection-refused transport error.
	seedPusherConfig(t, "http://127.0.0.1:1/webhook")
	seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "http://127.0.0.1:1/webhook")
	assert.Contains(t, err.Error(), "connection refused")
}

// ---------------------------------------------------------------------------
// Poison-queue protection (bug 6): delivery attempts tracking + capped skip
// ---------------------------------------------------------------------------

// deliveryRow fetches the (event, target) delivery row; fails the test when
// absent.
func deliveryRow(t *testing.T, eventID, targetID uuid.UUID) *models.EventDelivery {
	t.Helper()
	row := &models.EventDelivery{}
	require.NoError(t, pusherTestDB.Where(
		"event_id = ? AND target_id = ?", eventID.String(), targetID.String()).First(row))
	return row
}

func TestDeliverBatch_FailureIncrementsAttemptsAndStoresError(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"processed":0,"total":1,"processed_ids":[],"errors":["event x: instance block mismatch"]}`))
	}))
	defer srv.Close()

	target := seedPusherConfig(t, srv.URL)
	ev := seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)

	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.Nil(t, got.DeliveredAt)
	row := deliveryRow(t, ev.ID, target.ID)
	assert.Equal(t, 1, row.Attempts, "failed delivery must increment attempts")
	require.NotNil(t, row.LastError)
	assert.Contains(t, *row.LastError, "instance block mismatch")

	// Second failure increments again.
	_, err = deliverBatch(t)
	require.Error(t, err)
	row = deliveryRow(t, ev.ID, target.ID)
	assert.Equal(t, 2, row.Attempts)
}

func TestDeliverBatch_TransportFailureIncrementsAttempts(t *testing.T) {
	resetPusherState()

	target := seedPusherConfig(t, "http://127.0.0.1:1/webhook")
	ev := seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)

	row := deliveryRow(t, ev.ID, target.ID)
	assert.Equal(t, 1, row.Attempts)
	require.NotNil(t, row.LastError)
	assert.Contains(t, *row.LastError, "connection refused")
}

// TestDeliverBatch_UnparseableResponseIncrementsAttempts reproduces the e2e
// finding: a receiver answering 200 with a contract-violating body (here
// "confirmed": true, a bool where the contract expects an array) made the
// batch retry forever WITHOUT incrementing attempts — the poison-message
// backstop never engaged and the queue spun indefinitely.
func TestDeliverBatch_UnparseableResponseIncrementsAttempts(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"contract_version":2,"processed":1,"total":1,"processed_ids":["x"],"errors":[],"confirmed":true}`))
	}))
	defer srv.Close()

	target := seedPusherConfig(t, srv.URL)
	ev := seedUndeliveredEvent(t, 1)

	_, err := deliverBatch(t)
	require.Error(t, err)
	require.Contains(t, err.Error(), "invalid webhook response")

	row := deliveryRow(t, ev.ID, target.ID)
	assert.Equal(t, 1, row.Attempts, "unparseable 200 must consume attempt budget")
	require.NotNil(t, row.LastError)
	assert.Contains(t, *row.LastError, "invalid webhook response")

	var got models.EventStream
	require.NoError(t, pusherTestDB.Find(&got, ev.ID))
	assert.Nil(t, got.DeliveredAt, "nothing may be marked delivered on an unparseable response")

	// Second round increments again — the event approaches the cap instead
	// of spinning forever.
	_, err = deliverBatch(t)
	require.Error(t, err)
	row = deliveryRow(t, ev.ID, target.ID)
	assert.Equal(t, 2, row.Attempts)
}

// TestDeliverBatch_CappedEventSkippedNoHeadOfLineBlocking reproduces the
// poison-queue bug: an event at the attempt cap must be skipped so newer
// events behind it are delivered.
func TestDeliverBatch_CappedEventSkippedNoHeadOfLineBlocking(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	target := seedPusherConfig(t, srv.URL)

	// Oldest event: poisoned (at the attempt cap for this target).
	poison := seedUndeliveredEvent(t, 1)
	require.NoError(t, pusherTestDB.RawQuery(
		"UPDATE event_streams SET created_at = ? WHERE id = ?",
		time.Now().Add(-time.Hour), poison.ID.String(),
	).Exec())
	require.NoError(t, pusherTestDB.Create(&models.EventDelivery{
		ID:        uuid.Must(uuid.NewV4()),
		EventID:   poison.ID,
		TargetID:  target.ID,
		Attempts:  models.MaxDeliveryAttempts,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	// Newer event: healthy, must be delivered despite the older poison row.
	healthy := seedUndeliveredEvent(t, 2)

	n, err := deliverBatch(t)
	require.NoError(t, err)
	assert.Equal(t, 1, n, "batch must contain only the healthy event")
	assert.Equal(t, 1, rr.totalEvents())

	var gotPoison, gotHealthy models.EventStream
	require.NoError(t, pusherTestDB.Find(&gotPoison, poison.ID))
	assert.Nil(t, gotPoison.DeliveredAt, "capped event stays undelivered")
	require.NoError(t, pusherTestDB.Find(&gotHealthy, healthy.ID))
	assert.NotNil(t, gotHealthy.DeliveredAt, "healthy event delivered — no head-of-line blocking")
}

// A partial failure must not penalize events the receiver DID accept.
func TestDeliverBatch_PartialFailureAttemptsOnlyOnRejected(t *testing.T) {
	resetPusherState()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload struct {
			Events []struct {
				ID string `json:"id"`
			} `json:"events"`
		}
		_ = json.Unmarshal(body, &payload)
		accepted := ""
		rejected := ""
		if len(payload.Events) == 2 {
			accepted = payload.Events[0].ID
			rejected = payload.Events[1].ID
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"processed":1,"total":2,"processed_ids":["` + accepted + `"],"errors":["event ` + rejected + `: broken"]}`))
	}))
	defer srv.Close()

	target := seedPusherConfig(t, srv.URL)
	good := seedUndeliveredEvent(t, 1)
	bad := seedUndeliveredEvent(t, 2)

	_, err := deliverBatch(t)
	require.Error(t, err)

	var gotGood, gotBad models.EventStream
	require.NoError(t, pusherTestDB.Find(&gotGood, good.ID))
	assert.NotNil(t, gotGood.DeliveredAt)
	goodRow := deliveryRow(t, good.ID, target.ID)
	assert.Equal(t, 0, goodRow.Attempts, "accepted event must not be penalized")
	assert.NotNil(t, goodRow.DeliveredAt)
	require.NoError(t, pusherTestDB.Find(&gotBad, bad.ID))
	assert.Nil(t, gotBad.DeliveredAt)
	badRow := deliveryRow(t, bad.ID, target.ID)
	assert.Equal(t, 1, badRow.Attempts, "rejected event gets the attempt")
}
