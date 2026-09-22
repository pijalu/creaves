package actions

import (
	"bytes"
	"creaves/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/events"
	"github.com/gofrs/uuid"
)

// maxErrorExcerpt caps how much of a failing webhook response body is copied
// into error messages / logs (receivers can return arbitrarily large bodies).
const maxErrorExcerpt = 500

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.Mutex
	failures         int
	lastFailure      time.Time
	state            string // "closed", "open", "half-open"
	failureThreshold int
	resetTimeout     time.Duration
}

// NewCircuitBreaker creates a new circuit breaker
func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{
		state:            "closed",
		failureThreshold: 5,
		resetTimeout:     60 * time.Second,
	}
}

// IsOpen reports whether the circuit is open. An open circuit whose reset
// timeout has elapsed transitions to half-open (delivery allowed again) under
// the same lock, so the state machine has no lock-upgrade race.
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.state == "open" && time.Since(cb.lastFailure) > cb.resetTimeout {
		cb.state = "half-open"
	}
	return cb.state == "open"
}

// RecordSuccess resets the circuit breaker
func (cb *CircuitBreaker) RecordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures = 0
	cb.state = "closed"
}

// RecordFailure increments failure count and opens circuit if threshold reached
func (cb *CircuitBreaker) RecordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.failures++
	cb.lastFailure = time.Now()
	if cb.failures >= cb.failureThreshold {
		cb.state = "open"
	}
}

// targetDeliveryState holds the per-target delivery runtime state: one
// circuit breaker and one rate limiter per sync target, so a failing or
// slow hub never throttles delivery to the others.
type targetDeliveryState struct {
	breaker       *CircuitBreaker
	lastDelivery  time.Time
	eventsThisMin int
}

// WebhookPusher handles sending events to the configured sync targets
// (multi-hub fan-out). Shared HTTP client; per-target state keyed by
// sync target ID.
type WebhookPusher struct {
	client  *http.Client
	mu      sync.Mutex
	targets map[uuid.UUID]*targetDeliveryState
}

// Global webhook pusher instance
var (
	webhookPusher *WebhookPusher
	webhookTicker *time.Ticker
	webhookWakeCh chan struct{}
	stopChan      chan bool
	// webhookWorkerDone is closed by the worker loop goroutine on exit so
	// StopWebhookWorker can join it (making stops fully synchronous).
	webhookWorkerDone chan struct{}
	pusherMu          sync.Mutex
)

// retryPollInterval is the fallback polling cadence used to retry failed
// deliveries and recover from circuit-breaker pauses. New events do NOT wait
// for this tick: PublishEvent signals the worker through the wake channel for
// near-immediate delivery.
const retryPollInterval = 60 * time.Second

// Event retention policy: delivered events are kept for 1 day,
// undelivered events for 7 days. Older rows are purged automatically
// by the webhook worker so the event_streams table does not grow
// unbounded.
const (
	deliveredEventRetention   = 24 * time.Hour
	undeliveredEventRetention = 7 * 24 * time.Hour
	eventPurgeInterval        = time.Hour
)

// init initializes the webhook pusher
func init() {
	webhookPusher = &WebhookPusher{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		targets: make(map[uuid.UUID]*targetDeliveryState),
	}
}

// targetState returns (creating on first use) the delivery runtime state for
// one sync target. Caller must not hold wp.mu.
func (wp *WebhookPusher) targetState(targetID uuid.UUID) *targetDeliveryState {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	st, ok := wp.targets[targetID]
	if !ok {
		st = &targetDeliveryState{breaker: NewCircuitBreaker()}
		wp.targets[targetID] = st
	}
	return st
}

// forgetTarget drops the runtime state of a deleted sync target.
func (wp *WebhookPusher) forgetTarget(targetID uuid.UUID) {
	wp.mu.Lock()
	defer wp.mu.Unlock()
	delete(wp.targets, targetID)
}

// allowDelivery checks and consumes the per-target rate limit. The batch
// size is charged up front so a burst of full batches cannot exceed the
// configured per-minute cap.
func (st *targetDeliveryState) allowDelivery(maxPerMin, batchSize int) bool {
	wp := webhookPusher
	wp.mu.Lock()
	defer wp.mu.Unlock()

	now := time.Now()
	if now.Sub(st.lastDelivery) >= time.Minute {
		st.eventsThisMin = 0
	}
	if st.eventsThisMin >= maxPerMin {
		return false
	}
	st.lastDelivery = now
	st.eventsThisMin += batchSize
	if st.eventsThisMin > maxPerMin {
		st.eventsThisMin = maxPerMin
	}
	return true
}

// StartWebhookWorker starts the background webhook delivery worker
func StartWebhookWorker() {
	pusherMu.Lock()
	defer pusherMu.Unlock()

	if webhookTicker != nil {
		return // Already running
	}

	ticker := time.NewTicker(retryPollInterval)
	stop := make(chan bool)
	// Buffered cap-1 wake channel: signals from PublishEvent collapse into at
	// most one pending wake (natural debounce during bursts).
	wake := make(chan struct{}, 1)
	// Publish for StopWebhookWorker / IsWebhookWorkerRunning / signalWebhookWake
	// coordination.
	webhookTicker = ticker
	webhookWakeCh = wake
	stopChan = stop
	done := make(chan struct{})
	webhookWorkerDone = done

	go webhookWorkerLoop(ticker, wake, stop, done)

	fmt.Println("Webhook worker started")
}

// webhookWorkerLoop is event-driven: it sleeps until PublishEvent signals a
// wake, then drains all pending events in a batch loop. The ticker is only a
// fallback for retrying failed deliveries and running the hourly purge. It
// references the LOCAL ticker/wake/stop channels so that StopWebhookWorker
// nil-ing the package globals cannot race with a delivery already in flight
// (previously this read the global webhookTicker directly and could nil-deref
// on shutdown).
func webhookWorkerLoop(ticker *time.Ticker, wake chan struct{}, stop chan bool, done chan struct{}) {
	defer close(done)
	lastPurge := time.Time{} // zero value: purge runs on first tick
	for {
		select {
		case <-wake:
			drainPendingBatches()
		case <-ticker.C:
			lastPurge = purgeExpiredEvents(lastPurge)
			deliverPendingBatch()
		case <-stop:
			return
		}
	}
}

// signalWebhookWake nudges the worker to deliver now instead of waiting for
// the fallback tick. Safe to call when the worker is stopped (no-op).
func signalWebhookWake() {
	pusherMu.Lock()
	wake := webhookWakeCh
	pusherMu.Unlock()
	if wake == nil {
		return
	}
	select {
	case wake <- struct{}{}:
	default: // a wake is already pending
	}
}

// drainPendingBatches delivers batches back-to-back until no undelivered
// events remain (or delivery becomes gated). Used on wake so a burst of
// events drains immediately instead of one batch per tick.
func drainPendingBatches() {
	for deliverPendingBatch() {
	}
}

// purgeExpiredEvents purges expired events once per interval, even when the
// webhook is disabled, so the event_streams table cannot grow unbounded. It
// returns the timestamp of the last successful purge.
func purgeExpiredEvents(lastPurge time.Time) time.Time {
	if time.Since(lastPurge) < eventPurgeInterval {
		return lastPurge
	}
	if err := purgeOldEvents(); err != nil {
		fmt.Printf("Event purge failed: %v\n", err)
		return lastPurge
	}
	return time.Now()
}

// syncTargetsKnown caches "at least one enabled sync target exists" so the
// background worker can short-circuit before touching the database. This
// matters beyond the cheap win: the worker goroutine shares models.DB with
// request handlers, and a pop connection is not safe for concurrent use —
// with no target configured (the default, and the state of most test flows)
// the worker must not query the database at all (mirrors the legacy
// behaviour, which returned before any query when no webhook URL was set).
// The flag is refreshed at boot and on every sync-target CRUD operation via
// SetSyncTargetsKnown.
var syncTargetsKnown atomic.Bool

// SetSyncTargetsKnown records whether at least one enabled sync target
// exists. Called by the sync-target CRUD handlers, by InitWebhookAtBoot, and
// by tests seeding targets directly.
func SetSyncTargetsKnown(known bool) {
	syncTargetsKnown.Store(known)
}

// deliverPendingBatch fans one delivery round out to every deliverable sync
// target. It returns true when at least one target delivered a full batch,
// meaning more events are likely pending and the caller should loop (see
// drainPendingBatches).
func deliverPendingBatch() bool {
	if !syncTargetsKnown.Load() {
		return false
	}
	targets, err := models.EnabledSyncTargets(models.DB)
	if err != nil {
		fmt.Printf("Webhook: failed to list sync targets: %v\n", err)
		return false
	}
	more := false
	for i := range targets {
		target := &targets[i]
		if !target.Deliverable() {
			continue
		}
		state := webhookPusher.targetState(target.ID)
		if state.breaker.IsOpen() {
			continue
		}
		batchSize := target.EffectiveBatchSize()
		if !state.allowDelivery(target.EffectiveMaxPerMin(), batchSize) {
			continue
		}
		n, err := deliverTargetBatch(target)
		if err != nil {
			fmt.Printf("Webhook delivery to %q failed: %v\n", target.Name, err)
			continue
		}
		if n >= batchSize {
			more = true
		}
	}
	return more
}

// StopWebhookWorker stops the background webhook delivery worker. It is
// synchronous: it waits for the worker goroutine (including any delivery
// already in flight) to exit, so callers that follow it with config changes
// cannot race with a worker still reading the old config.
func StopWebhookWorker() {
	pusherMu.Lock()
	var done chan struct{}
	if webhookTicker != nil {
		webhookTicker.Stop()
		close(stopChan)
		done = webhookWorkerDone
		webhookTicker = nil
		webhookWakeCh = nil
		webhookWorkerDone = nil
		fmt.Println("Webhook worker stopped")
	}
	pusherMu.Unlock()
	if done != nil {
		<-done
	}
}

// IsWebhookWorkerRunning returns true if the worker is running
func IsWebhookWorkerRunning() bool {
	pusherMu.Lock()
	defer pusherMu.Unlock()
	return webhookTicker != nil
}

// purgeOldEvents deletes events that have outlived the retention
// policy: delivered events older than deliveredEventRetention and
// undelivered events older than undeliveredEventRetention.
func purgeOldEvents() error {
	deliveredCutoff := time.Now().Add(-deliveredEventRetention)
	undeliveredCutoff := time.Now().Add(-undeliveredEventRetention)

	// Chunked deletes: an unbounded DELETE holds row locks and builds one
	// huge binlog transaction; batched rounds keep each step small. Each
	// round selects one batch of row ids, then deletes them by primary
	// key — `DELETE ... LIMIT` is not portable (SQLite builds without
	// SQLITE_ENABLE_UPDATE_DELETE_LIMIT reject it), so the id-first form
	// keeps MySQL/MariaDB and the sqlite test suite on the same path.
	const purgeChunk = 1000

	for {
		var ids []uuid.UUID
		if err := models.DB.RawQuery(
			"SELECT id FROM event_streams WHERE delivered_at IS NOT NULL AND delivered_at < ? LIMIT ?",
			deliveredCutoff, purgeChunk,
		).All(&ids); err != nil {
			return fmt.Errorf("failed to select delivered events for purge: %w", err)
		}
		if len(ids) == 0 {
			break
		}
		if err := deleteEventsAndDeliveries(ids); err != nil {
			return fmt.Errorf("failed to purge delivered events: %w", err)
		}
		if len(ids) < purgeChunk {
			break
		}
	}

	for {
		var ids []uuid.UUID
		if err := models.DB.RawQuery(
			"SELECT id FROM event_streams WHERE delivered_at IS NULL AND created_at < ? LIMIT ?",
			undeliveredCutoff, purgeChunk,
		).All(&ids); err != nil {
			return fmt.Errorf("failed to select undelivered events for purge: %w", err)
		}
		if len(ids) == 0 {
			break
		}
		if err := deleteEventsAndDeliveries(ids); err != nil {
			return fmt.Errorf("failed to purge undelivered events: %w", err)
		}
		if len(ids) < purgeChunk {
			break
		}
	}

	return nil
}

// deleteEventsAndDeliveries removes a chunk of events together with their
// per-target delivery rows (event_deliveries references event_streams
// logically; rows are removed first to keep the pair consistent).
func deleteEventsAndDeliveries(ids []uuid.UUID) error {
	if len(ids) == 0 {
		return nil
	}
	if _, err := models.DB.RawQuery(
		"DELETE FROM event_deliveries WHERE event_id IN (?)", ids,
	).ExecWithCount(); err != nil {
		return err
	}
	_, err := models.DB.RawQuery(
		"DELETE FROM event_streams WHERE id IN (?)", ids,
	).ExecWithCount()
	return err
}

// syncAnnouncementWire is the "sync" envelope block: the producer's
// announced expected sync state for a resync run.
type syncAnnouncementWire struct {
	ExpectedTotal    int        `json:"expected_total"`
	ExpectedChecksum string     `json:"expected_checksum"`
	AnnouncedAt      *time.Time `json:"announced_at"`
}

// attachResyncAnnouncement adds the producer announcement (checksum fix):
// batches belonging to a resync run carry that run's announced expected
// sync state. The console stores it on the instance row and displays
// stored/announced(expected) with a checksum comparison — it can no longer
// mistake "the events I received" for "everything the producer has".
func attachResyncAnnouncement(events *models.EventStreams, payload map[string]interface{}) {
	for i := range *events {
		if (*events)[i].ResyncRunID == nil {
			continue
		}
		run := &models.ResyncRun{}
		if err := models.DB.Find(run, *(*events)[i].ResyncRunID); err != nil ||
			run.AnnouncedExpectedChecksum == nil || *run.AnnouncedExpectedChecksum == "" {
			continue
		}
		payload["sync"] = syncAnnouncementWire{
			ExpectedTotal:    run.AnnouncedExpectedTotal,
			ExpectedChecksum: *run.AnnouncedExpectedChecksum,
			AnnouncedAt:      run.AnnouncedAt,
		}
		return
	}
}

// stateConfirmation is one console acknowledgement: the echoed state hash
// the console stored for a processed animal_state event.
type stateConfirmation struct {
	ID        string `json:"id"`
	StateHash string `json:"state_hash"`
}

// webhookEvent is the wire representation of one lifecycle event in the
// delivery payload (contract v2).
type webhookEvent struct {
	ID          string          `json:"id"`
	InstanceID  string          `json:"instance_id"`
	AnimalID    int             `json:"animal_id"`
	EventType   string          `json:"event_type"`
	Payload     json.RawMessage `json:"payload"`
	ResyncRunID string          `json:"resync_run_id,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
}

// newWireEvent maps a stored event onto the wire. Contract v2 addition
// (bugs.md #9): resync-delivered events carry their run id so the console
// can attribute them in its event history. Live events omit the field.
func newWireEvent(event models.EventStream) webhookEvent {
	wire := webhookEvent{
		ID:         event.ID.String(),
		InstanceID: event.InstanceID,
		AnimalID:   event.AnimalID,
		EventType:  string(event.EventType),
		Payload:    event.Payload,
		CreatedAt:  event.CreatedAt,
	}
	if event.ResyncRunID != nil {
		wire.ResyncRunID = event.ResyncRunID.String()
	}
	return wire
}

// pendingEventsForTarget selects the oldest events that still need delivery
// to this target: no delivery row yet, or a delivery row that is neither
// delivered nor undeliverable (attempts below the cap).
func pendingEventsForTarget(target *models.SyncTarget, limit int) (*models.EventStreams, error) {
	events := &models.EventStreams{}
	err := models.DB.RawQuery(`SELECT e.* FROM event_streams e
		LEFT JOIN event_deliveries d ON d.event_id = e.id AND d.target_id = ?
		WHERE (d.id IS NULL) OR (d.delivered_at IS NULL AND d.attempts < ?)
		ORDER BY e.created_at LIMIT ?`,
		target.ID.String(), models.MaxDeliveryAttempts, limit).All(events)
	return events, err
}

// deliverTargetBatch queries pending events for one target and posts them to
// that target's webhook endpoint. It returns the number of events in the
// queried batch (accepted or not), so callers can decide whether more events
// are likely pending.
func deliverTargetBatch(target *models.SyncTarget) (int, error) {
	config := CurrentConfigGet()
	if config == nil {
		return 0, fmt.Errorf("no config loaded")
	}

	batchSize := target.EffectiveBatchSize()
	events, err := pendingEventsForTarget(target, batchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to query events: %w", err)
	}
	if len(*events) == 0 {
		return 0, nil
	}

	// Build payload
	payloadEvents := make([]webhookEvent, len(*events))
	for i, event := range *events {
		payloadEvents[i] = newWireEvent(event)
	}

	payload := map[string]interface{}{
		"contract_version": 2,
		"instance":         map[string]string{"id": config.InstanceID, "name": config.Name, "description": config.Description},
		"events":           payloadEvents,
	}
	attachResyncAnnouncement(events, payload)

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return len(*events), fmt.Errorf("failed to marshal payload: %w", err)
	}

	state := webhookPusher.targetState(target.ID)

	// Send HTTP POST
	req, err := http.NewRequest("POST", target.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return len(*events), fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+target.WebhookAPIKey)

	resp, err := webhookPusher.client.Do(req)
	if err != nil {
		state.breaker.RecordFailure()
		// Transport failure: surface the URL so operators can tell DNS /
		// refused-connection / TLS problems apart without guessing.
		err = fmt.Errorf("webhook POST %s (%d events) failed: %w", target.WebhookURL, len(*events), err)
		recordDeliveryFailures(events, target, err)
		return len(*events), err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		state.breaker.RecordFailure()
		// Non-200: include a body excerpt — receivers usually explain the
		// rejection there (bad auth, malformed envelope, ...).
		excerpt, _ := io.ReadAll(io.LimitReader(resp.Body, maxErrorExcerpt))
		err = fmt.Errorf("webhook POST %s (%d events) returned status %d: %s", target.WebhookURL, len(*events), resp.StatusCode, strings.TrimSpace(string(excerpt)))
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			// Invalid API key: retrying cannot succeed until the operator
			// fixes the target credentials. Mark the batch undeliverable at
			// once instead of burning the attempt budget while the queue
			// stalls; the "retry undeliverable" action re-queues the events
			// after the key is fixed.
			markBatchUndeliverable(events, target, err)
		} else {
			recordDeliveryFailures(events, target, err)
		}
		return len(*events), err
	}

	// Determine which events the receiver actually accepted. On partial
	// failure the receiver returns the IDs it processed (processed_ids);
	// events absent from that list are left undelivered (delivery row stays
	// pending) so they are retried on the next tick instead of being
	// silently dropped.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		err = fmt.Errorf("failed to read webhook response: %w", err)
		recordDeliveryFailures(events, target, err)
		return len(*events), err
	}

	var result struct {
		Processed    *int     `json:"processed"`
		Total        *int     `json:"total"`
		ProcessedIDs []string `json:"processed_ids"`
		Errors       []string `json:"errors"`
		// Confirmations (checksum fix): the console echoes, per processed
		// animal_state event, the state hash it actually stored. Only
		// echoed events whose hash matches the producer content hash get
		// acknowledged_at set — the "Delivered & current" confirmation on
		// /webhook_resync is fed by these, not by bare HTTP acceptance.
		Confirmed []stateConfirmation `json:"confirmed"`
	}
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			// 200 OK with a body we cannot parse (e.g. contract violation
			// like "confirmed": true instead of an array). Without
			// recording attempts the batch would retry forever, bypassing
			// the poison-message backstop (found in e2e: a mock receiver
			// with a malformed response shape kept the queue spinning).
			state.breaker.RecordFailure()
			err = fmt.Errorf("invalid webhook response: %w", err)
			recordDeliveryFailures(events, target, err)
			return len(*events), err
		}
	}

	accepted := make(map[string]bool, len(result.ProcessedIDs))
	for _, id := range result.ProcessedIDs {
		accepted[id] = true
	}
	// Legacy receivers may return an empty body. Only that unstructured 200
	// response gets all-events compatibility; explicit errors or partial
	// counts must never silently mark absent events as delivered.
	partial := result.Processed != nil && result.Total != nil && *result.Processed < *result.Total
	responseErr := len(result.Errors) > 0 || partial
	if responseErr {
		// Continue below so explicitly listed processed_ids are persisted;
		// unlisted events remain pending for retry.
		state.breaker.RecordFailure()
	}
	// The documented Console response reports only processed/total counts on
	// full success. Treat that complete response as acceptance of every event;
	// partial responses still require explicit processed_ids.
	fullCount := result.Processed != nil && result.Total != nil && *result.Processed == len(*events) && *result.Total == len(*events) && len(result.Errors) == 0
	legacyEmpty := result.Processed == nil && result.Total == nil && len(result.Errors) == 0
	if (fullCount || legacyEmpty) && len(result.ProcessedIDs) == 0 {
		for _, event := range *events {
			accepted[event.ID.String()] = true
		}
	}

	// Mark accepted events as delivered for this target; leave the rest
	// pending for retry.
	now := time.Now()
	deliveredIDs := make([]uuid.UUID, 0, len(*events))
	for i := range *events {
		event := &(*events)[i]
		if accepted[event.ID.String()] {
			deliveredIDs = append(deliveredIDs, event.ID)
		}
	}
	delivered := 0
	if err := upsertDeliveries(deliveredIDs, target.ID, now); err != nil {
		fmt.Printf("Failed to mark %d events as delivered to %q: %v\n", len(deliveredIDs), target.Name, err)
	} else {
		delivered = len(deliveredIDs)
		for _, id := range deliveredIDs {
			for i := range *events {
				if (*events)[i].ID == id {
					(*events)[i].DeliveredAt = &now
					break
				}
			}
		}
		// Rollup: event_streams.delivered_at is set once every enabled
		// target has its delivery row delivered.
		if err := rollupDeliveredEvents(deliveredIDs, now); err != nil {
			fmt.Printf("Failed to roll up delivered events: %v\n", err)
		}
	}

	acknowledged := applyConfirmations(events, accepted, result.Confirmed, target, now)

	if responseErr {
		// Partial/rejected delivery: relay the receiver's per-event errors
		// (they name the exact event and cause, e.g. "instance block
		// mismatch") so the log line is actionable on its own.
		err := fmt.Errorf("webhook POST %s accepted %d/%d events (receiver errors: %s)", target.WebhookURL, delivered, len(*events), strings.Join(result.Errors, "; "))
		// Rejected events stay pending and are retried on the next tick —
		// a partial accept is often transient (receiver busy, one event
		// momentarily invalid). The per-event attempt budget (25) is the
		// poison-message backstop; once exhausted the event is undeliverable
		// and the "retry undeliverable" admin action re-queues it.
		recordDeliveryFailures(events, target, err)
		return len(*events), err
	}
	state.breaker.RecordSuccess()
	if delivered < len(*events) {
		fmt.Printf("Delivered %d/%d events to %q (%d acknowledged); %d will be retried\n", delivered, len(*events), target.Name, acknowledged, len(*events)-delivered)
	} else {
		fmt.Printf("Delivered %d events to %q (%d acknowledged)\n", delivered, target.Name, acknowledged)
	}

	return len(*events), nil
}

// upsertDeliveries marks each listed event delivered for one target,
// creating the delivery row on first success. Portable upsert (SELECT then
// INSERT or UPDATE — pop has no portable ON DUPLICATE KEY support and the
// sqlite test suite runs the same code path).
func upsertDeliveries(eventIDs []uuid.UUID, targetID uuid.UUID, now time.Time) error {
	for _, id := range eventIDs {
		if err := markDeliveryDelivered(id, targetID, now); err != nil {
			return err
		}
	}
	return nil
}

// markDeliveryDelivered sets delivered_at on the (event, target) delivery
// row, creating it when missing.
func markDeliveryDelivered(eventID, targetID uuid.UUID, now time.Time) error {
	var existing models.EventDelivery
	err := models.DB.Where("event_id = ? AND target_id = ?", eventID.String(), targetID.String()).First(&existing)
	if err == nil {
		return models.DB.RawQuery(
			"UPDATE event_deliveries SET delivered_at = ?, updated_at = ? WHERE id = ?",
			now, now, existing.ID.String()).Exec()
	}
	rowID, idErr := uuid.NewV4()
	if idErr != nil {
		return idErr
	}
	return models.DB.RawQuery(
		"INSERT INTO event_deliveries (id, event_id, target_id, attempts, delivered_at, created_at, updated_at) VALUES (?, ?, ?, 0, ?, ?, ?)",
		rowID.String(), eventID.String(), targetID.String(), now, now, now).Exec()
}

// rollupDeliveredEvents sets event_streams.delivered_at for events whose
// delivery rows now cover every enabled target. Runs after each successful
// batch; cheap because the candidate set is the just-delivered ids.
func rollupDeliveredEvents(eventIDs []uuid.UUID, now time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	enabled, err := models.EnabledSyncTargets(models.DB)
	if err != nil {
		return err
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(eventIDs)), ",")
	args := make([]interface{}, 0, len(eventIDs)+2)
	args = append(args, now, now)
	for _, id := range eventIDs {
		args = append(args, id)
	}
	if len(enabled) == 0 {
		// No enabled target (anymore): nothing should stay marked delivered
		// from this round — but the rows were delivered to a target that got
		// disabled meanwhile, which is a legitimate delivered state. Mark
		// them delivered on the stream too so they purge normally.
		q := fmt.Sprintf("UPDATE event_streams SET delivered_at = COALESCE(delivered_at, ?), updated_at = ? WHERE id IN (%s)", placeholders)
		return models.DB.RawQuery(q, args...).Exec()
	}
	// delivered when the count of enabled targets equals the count of
	// delivered rows for this event over enabled targets.
	targetIDs := make([]interface{}, 0, len(enabled))
	for _, t := range enabled {
		targetIDs = append(targetIDs, t.ID.String())
	}
	tph := strings.TrimRight(strings.Repeat("?,", len(enabled)), ",")
	q := fmt.Sprintf(`UPDATE event_streams SET delivered_at = ?, updated_at = ?
		WHERE id IN (%s) AND delivered_at IS NULL
		AND (SELECT COUNT(*) FROM event_deliveries d
			WHERE d.event_id = event_streams.id AND d.delivered_at IS NOT NULL AND d.target_id IN (%s)) = ?`, placeholders, tph)
	args = append(args, targetIDs...)
	args = append(args, len(enabled))
	return models.DB.RawQuery(q, args...).Exec()
}

// applyConfirmations applies console confirmations for one target: an
// acknowledgement is only trusted when the echoed state hash equals the
// producer's content hash for that event. Older consoles never send
// "confirmed" — those deliveries keep acknowledged_at NULL and the resync
// page honestly reports them unconfirmed instead of pretending
// delivery == confirmation.
func applyConfirmations(events *models.EventStreams, accepted map[string]bool, confirmed []stateConfirmation, target *models.SyncTarget, now time.Time) int {
	batchByID := make(map[string]*models.EventStream, len(*events))
	for i := range *events {
		batchByID[(*events)[i].ID.String()] = &(*events)[i]
	}
	acknowledgeIDs := make([]uuid.UUID, 0, len(confirmed))
	for _, conf := range confirmed {
		event, ok := batchByID[conf.ID]
		if !ok || !accepted[conf.ID] || event.ContentHash == nil ||
			*event.ContentHash == "" || *event.ContentHash != conf.StateHash {
			continue
		}
		acknowledgeIDs = append(acknowledgeIDs, event.ID)
	}
	if len(acknowledgeIDs) == 0 {
		return 0
	}
	if err := acknowledgeDeliveries(acknowledgeIDs, target.ID, now); err != nil {
		fmt.Printf("Failed to acknowledge %d events for %q: %v\n", len(acknowledgeIDs), target.Name, err)
		return 0
	}
	if err := rollupAcknowledgedEvents(acknowledgeIDs, now); err != nil {
		fmt.Printf("Failed to roll up acknowledged events: %v\n", err)
	}
	for _, id := range acknowledgeIDs {
		if event, ok := batchByID[id.String()]; ok {
			ack := now
			event.AcknowledgedAt = &ack
		}
	}
	return len(acknowledgeIDs)
}

// acknowledgeDeliveries sets acknowledged_at on the delivery rows of one
// target for the validated events.
func acknowledgeDeliveries(eventIDs []uuid.UUID, targetID uuid.UUID, now time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(eventIDs)), ",")
	args := make([]interface{}, 0, len(eventIDs)+3)
	args = append(args, now, now, targetID.String())
	for _, id := range eventIDs {
		args = append(args, id)
	}
	q := fmt.Sprintf("UPDATE event_deliveries SET acknowledged_at = ?, updated_at = ? WHERE target_id = ? AND event_id IN (%s)", placeholders)
	return models.DB.RawQuery(q, args...).Exec()
}

// rollupAcknowledgedEvents sets event_streams.acknowledged_at once every
// enabled target has acknowledged its delivery of the event.
func rollupAcknowledgedEvents(eventIDs []uuid.UUID, now time.Time) error {
	if len(eventIDs) == 0 {
		return nil
	}
	enabled, err := models.EnabledSyncTargets(models.DB)
	if err != nil {
		return err
	}
	if len(enabled) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(eventIDs)), ",")
	args := make([]interface{}, 0, len(eventIDs)+len(enabled)+3)
	args = append(args, now, now)
	for _, id := range eventIDs {
		args = append(args, id)
	}
	targetIDs := make([]interface{}, 0, len(enabled))
	for _, t := range enabled {
		targetIDs = append(targetIDs, t.ID.String())
	}
	tph := strings.TrimRight(strings.Repeat("?,", len(enabled)), ",")
	q := fmt.Sprintf(`UPDATE event_streams SET acknowledged_at = ?, updated_at = ?
		WHERE id IN (%s) AND acknowledged_at IS NULL
		AND (SELECT COUNT(*) FROM event_deliveries d
			WHERE d.event_id = event_streams.id AND d.acknowledged_at IS NOT NULL AND d.target_id IN (%s)) = ?`, placeholders, tph)
	args = append(args, targetIDs...)
	args = append(args, len(enabled))
	return models.DB.RawQuery(q, args...).Exec()
}

// markBatchUndeliverable marks every still-pending event of the batch
// undeliverable for this target (attempts set to the cap) with the given
// reason. Used for permanent rejections — invalid credentials (401/403)
// and explicit receiver rejections — where retrying cannot succeed without
// operator intervention. The "retry undeliverable" admin action resets the
// rows so the events re-enter the delivery queue.
func markBatchUndeliverable(events *models.EventStreams, target *models.SyncTarget, cause error) {
	if events == nil || len(*events) == 0 || cause == nil {
		return
	}
	msg := cause.Error()
	if len(msg) > maxErrorExcerpt {
		msg = msg[:maxErrorExcerpt]
	}
	for i := range *events {
		event := &(*events)[i]
		if event.DeliveredAt != nil {
			continue
		}
		if err := setDeliveryAttempts(event.ID, target.ID, models.MaxDeliveryAttempts, msg); err != nil {
			fmt.Printf("Failed to mark event %s undeliverable: %v\n", event.ID, err)
		}
	}
}

// setDeliveryAttempts upserts the (event, target) delivery row with an
// absolute attempts value and a last-error message (portable SELECT then
// INSERT or UPDATE).
func setDeliveryAttempts(eventID, targetID uuid.UUID, attempts int, msg string) error {
	var existing models.EventDelivery
	err := models.DB.Where("event_id = ? AND target_id = ?", eventID.String(), targetID.String()).First(&existing)
	if err == nil {
		return models.DB.RawQuery(
			"UPDATE event_deliveries SET attempts = ?, last_error = ?, updated_at = ? WHERE id = ?",
			attempts, msg, time.Now(), existing.ID.String()).Exec()
	}
	rowID, idErr := uuid.NewV4()
	if idErr != nil {
		return idErr
	}
	return models.DB.RawQuery(
		"INSERT INTO event_deliveries (id, event_id, target_id, attempts, last_error, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		rowID.String(), eventID.String(), targetID.String(), attempts, msg, time.Now(), time.Now()).Exec()
}

// recordDeliveryFailures increments attempts and stores the last error on
// the delivery rows of every event still pending for this target after a
// failed batch. Rows are created when missing so the attempt budget is
// tracked from the first failure. Failure to persist is logged, never
// fatal: delivery correctness does not depend on the counter, only
// poison-queue protection does.
func recordDeliveryFailures(events *models.EventStreams, target *models.SyncTarget, batchErr error) {
	if events == nil || len(*events) == 0 || batchErr == nil {
		return
	}
	msg := batchErr.Error()
	if len(msg) > maxErrorExcerpt {
		msg = msg[:maxErrorExcerpt]
	}
	for i := range *events {
		event := &(*events)[i]
		if event.DeliveredAt != nil {
			continue
		}
		if err := incrementDeliveryAttempts(event.ID, target.ID, msg); err != nil {
			fmt.Printf("Failed to record delivery failure for event %s: %v\n", event.ID, err)
		}
	}
}

// incrementDeliveryAttempts upserts the (event, target) delivery row,
// incrementing attempts by one and storing the last-error message
// (portable SELECT then INSERT or UPDATE).
func incrementDeliveryAttempts(eventID, targetID uuid.UUID, msg string) error {
	var existing models.EventDelivery
	err := models.DB.Where("event_id = ? AND target_id = ?", eventID.String(), targetID.String()).First(&existing)
	if err == nil {
		return models.DB.RawQuery(
			"UPDATE event_deliveries SET attempts = attempts + 1, last_error = ?, updated_at = ? WHERE id = ?",
			msg, time.Now(), existing.ID.String()).Exec()
	}
	rowID, idErr := uuid.NewV4()
	if idErr != nil {
		return idErr
	}
	return models.DB.RawQuery(
		"INSERT INTO event_deliveries (id, event_id, target_id, attempts, last_error, created_at, updated_at) VALUES (?, ?, ?, 1, ?, ?, ?)",
		rowID.String(), eventID.String(), targetID.String(), msg, time.Now(), time.Now()).Exec()
}

// RegisterWebhookShutdown registers graceful shutdown hook
func RegisterWebhookShutdown(app *buffalo.App) {
	events.Listen(func(e events.Event) {
		if e.Kind == buffalo.EvtAppStop {
			StopWebhookWorker()
		}
	})
}

// webhookWorkerAutoStartDisabled keeps EnsureWebhookWorkerRunning from
// implicitly starting the background worker. It exists for the test suites:
// the worker is a process-global goroutine that queries the shared
// models.DB connection every tick, so a test that wakes it through a
// handler path (event publish, sync-target CRUD, DLQ reset) would leave it
// running for the rest of the package run and make later tests
// order-dependent (the bugs.md #10 flake). Production startup is unaffected:
// the flag defaults to false and only the test bootstrap sets it; suites
// that assert worker behaviour start the worker explicitly.
var webhookWorkerAutoStartDisabled atomic.Bool

// EnsureWebhookWorkerRunning starts the webhook worker if it is not
// already running. The worker is started unconditionally (even when webhook
// forwarding is disabled) because it also runs the hourly event purge;
// delivery itself remains gated per-tick by the presence of deliverable
// sync targets. It is safe to call repeatedly.
func EnsureWebhookWorkerRunning() {
	if webhookWorkerAutoStartDisabled.Load() {
		return
	}
	if !IsWebhookWorkerRunning() {
		StartWebhookWorker()
	}
}

// InitWebhookAtBoot loads the configuration from the database and starts the
// background webhook worker (delivery + hourly event purge). It must be called
// at application startup, once the database connection (models.DB) is
// available, so that events queued before a restart are still delivered and
// expired events are purged. A failure to load the configuration is logged
// but never fatal: the worker remains stopped and may be started lazily
// later (e.g. when the next event is published).
func InitWebhookAtBoot() {
	if _, err := LoadConfig(models.DB); err != nil {
		fmt.Printf("Webhook: failed to load config at boot: %v\n", err)
		return
	}
	if err := RecoverInterruptedRuns(models.DB); err != nil {
		fmt.Printf("Webhook: failed to recover resync runs: %v\n", err)
	}
	known, err := models.HasEnabledSyncTarget(models.DB)
	if err != nil {
		fmt.Printf("Webhook: failed to list sync targets at boot: %v\n", err)
	}
	SetSyncTargetsKnown(known)
	EnsureWebhookWorkerRunning()
	// Boot sweep: pick up events left undelivered by a previous session
	// without waiting for the first fallback tick.
	signalWebhookWake()
}
