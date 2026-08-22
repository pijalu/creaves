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
			content_hash TEXT,
			resync_run_id TEXT,
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
}

// resetPusherState clears DB tables and resets the package-global pusher
// (rate limiter counters + circuit breaker) between tests.
func resetPusherState() {
	pusherTestDB.RawQuery("DELETE FROM event_streams").Exec()
	pusherTestDB.RawQuery("DELETE FROM config").Exec()

	webhookPusher.mu.Lock()
	webhookPusher.eventsThisMin = 0
	// Seed lastDelivery to "now" so the per-minute window does not reset
	// on every call (a zero time would look like >1 minute has elapsed).
	webhookPusher.lastDelivery = time.Now()
	webhookPusher.mu.Unlock()

	webhookPusher.circuitBreaker = NewCircuitBreaker()
}

// seedPusherConfig installs CurrentConfig pointing webhook delivery at url.
func seedPusherConfig(t *testing.T, url string) {
	t.Helper()
	settings := models.ConfigSettings{
		EnableEventStream: true,
		WebhookEnabled:    true,
		WebhookURL:        url,
		WebhookAPIKey:     "creaves_testkey",
		WebhookBatchSize:  10,
		WebhookMaxPerMin:  100,
	}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	require.NoError(t, pusherTestDB.Create(cfg))
	CurrentConfig = cfg
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
	cb.mu.RLock()
	assert.Equal(t, "half-open", cb.state)
	cb.mu.RUnlock()

	// A success while half-open closes the circuit.
	cb.RecordSuccess()
	assert.Equal(t, "closed", cb.state)
	assert.False(t, cb.IsOpen())
}

// ---------------------------------------------------------------------------
// Rate limiter (allowDelivery) unit tests
// ---------------------------------------------------------------------------

func TestWebhookPusher_AllowDeliveryRequiresConfig(t *testing.T) {
	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

	wp := &WebhookPusher{circuitBreaker: NewCircuitBreaker()}
	assert.False(t, wp.allowDelivery())
}

func TestWebhookPusher_AllowDeliveryRateLimits(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	settings, _ := CurrentConfig.GetSettings()
	maxPerMin := settings.WebhookMaxPerMin

	// Consume the budget.
	allowed := 0
	for i := 0; i < maxPerMin; i++ {
		if !webhookPusher.allowDelivery() {
			break
		}
		allowed++
	}
	assert.Greater(t, allowed, 0)
	// Budget exhausted -> further delivery denied.
	assert.False(t, webhookPusher.allowDelivery())
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

	require.NoError(t, deliverBatch())

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

func TestDeliverBatch_NoEventsNoop(t *testing.T) {
	resetPusherState()

	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)

	require.NoError(t, deliverBatch())
	assert.False(t, called, "receiver must not be called when there are no events")
}

func TestDeliverBatch_NonOKStatusRecordsFailure(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusInternalServerError)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	seedUndeliveredEvent(t, 1)

	before := webhookPusher.circuitBreaker.failures
	err := deliverBatch()
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")

	// Circuit breaker incremented.
	assert.Equal(t, before+1, webhookPusher.circuitBreaker.failures)

	// Event NOT marked delivered.
	var got models.EventStream
	require.NoError(t, pusherTestDB.Where("delivered_at IS NULL").First(&got))
}

func TestDeliverBatch_ConnectionErrorRecordsFailure(t *testing.T) {
	resetPusherState()

	// Point at a closed port to force a connection error.
	seedPusherConfig(t, "http://127.0.0.1:1/webhook")
	seedUndeliveredEvent(t, 1)

	before := webhookPusher.circuitBreaker.failures
	err := deliverBatch()
	require.Error(t, err)

	assert.Equal(t, before+1, webhookPusher.circuitBreaker.failures)
}

func TestDeliverBatch_EnvelopeHasInstanceAndVersion(t *testing.T) {
	resetPusherState()

	rr := newRecordingReceiver(http.StatusOK)
	srv := httptest.NewServer(http.HandlerFunc(rr.handler))
	defer srv.Close()

	seedPusherConfig(t, srv.URL)
	CurrentConfig.Description = "Wildlife care centre"
	seedUndeliveredEvent(t, 9)

	require.NoError(t, deliverBatch())
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

	require.NoError(t, deliverBatch())

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

	require.NoError(t, deliverBatch())

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

// TestEnsureWebhookWorkerRunning_NoopWhenDisabled verifies the worker is NOT
// started when webhook forwarding is disabled.
func TestEnsureWebhookWorkerRunning_NoopWhenDisabled(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// Config with webhook disabled.
	settings := models.ConfigSettings{
		EnableEventStream: true,
		WebhookEnabled:    false,
		WebhookURL:        "",
	}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	CurrentConfig = cfg

	EnsureWebhookWorkerRunning()
	assert.False(t, IsWebhookWorkerRunning(), "worker must not start when webhook is disabled")
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

// TestStartWebhookWorker_TickerDeliversEvents verifies the background ticker
// goroutine inside StartWebhookWorker: when the worker is running with an
// enabled webhook config, a real test receiver, and pending undelivered
// events, the ticker fires and delivers the batch. This covers the
// case <-ticker.C path (IsWebhookEnabled, circuit-breaker, allowDelivery,
// deliverBatch) which the fast start/stop tests cannot reach.
func TestStartWebhookWorker_TickerDeliversEvents(t *testing.T) {
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

	// The ticker fires every 5s; wait long enough for at least one tick.
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		if rr.totalEvents() > 0 {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	assert.Greater(t, rr.totalEvents(), 0, "ticker should have delivered the pending event")
}
