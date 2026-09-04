package actions

import (
	"bytes"
	"creaves/models"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/events"
)

// CircuitBreaker implements a simple circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.RWMutex
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

// IsOpen returns true if the circuit is open
func (cb *CircuitBreaker) IsOpen() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if cb.state == "open" {
		if time.Since(cb.lastFailure) > cb.resetTimeout {
			cb.mu.RUnlock()
			cb.mu.Lock()
			cb.state = "half-open"
			cb.mu.Unlock()
			cb.mu.RLock()
			return false
		}
		return true
	}
	return false
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

// WebhookPusher handles sending events to the webhook endpoint
type WebhookPusher struct {
	client         *http.Client
	circuitBreaker *CircuitBreaker
	lastDelivery   time.Time
	eventsThisMin  int
	mu             sync.RWMutex
}

// Global webhook pusher instance
var (
	webhookPusher *WebhookPusher
	webhookTicker *time.Ticker
	webhookWakeCh chan struct{}
	stopChan      chan bool
	pusherMu      sync.Mutex
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
		circuitBreaker: NewCircuitBreaker(),
	}
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

	go webhookWorkerLoop(ticker, wake, stop)

	fmt.Println("Webhook worker started")
}

// webhookWorkerLoop is event-driven: it sleeps until PublishEvent signals a
// wake, then drains all pending events in a batch loop. The ticker is only a
// fallback for retrying failed deliveries and running the hourly purge. It
// references the LOCAL ticker/wake/stop channels so that StopWebhookWorker
// nil-ing the package globals cannot race with a delivery already in flight
// (previously this read the global webhookTicker directly and could nil-deref
// on shutdown).
func webhookWorkerLoop(ticker *time.Ticker, wake chan struct{}, stop chan bool) {
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

// deliverPendingBatch delivers one batch of pending events if delivery is
// currently allowed (webhook enabled, circuit closed, rate limit not hit).
// It returns true only when a full batch was delivered, meaning more events
// are likely pending and the caller should loop (see drainPendingBatches).
func deliverPendingBatch() bool {
	if !IsWebhookEnabled() {
		return false
	}
	if webhookPusher.circuitBreaker.IsOpen() {
		return false
	}
	if !webhookPusher.allowDelivery() {
		return false
	}
	n, err := deliverBatch()
	if err != nil {
		fmt.Printf("Webhook delivery failed: %v\n", err)
		return false
	}
	settings, err := CurrentConfig.GetSettings()
	if err != nil {
		return false
	}
	return n >= settings.WebhookBatchSize
}

// StopWebhookWorker stops the background webhook delivery worker
func StopWebhookWorker() {
	pusherMu.Lock()
	defer pusherMu.Unlock()

	if webhookTicker != nil {
		webhookTicker.Stop()
		close(stopChan)
		webhookTicker = nil
		webhookWakeCh = nil
		fmt.Println("Webhook worker stopped")
	}
}

// IsWebhookWorkerRunning returns true if the worker is running
func IsWebhookWorkerRunning() bool {
	pusherMu.Lock()
	defer pusherMu.Unlock()
	return webhookTicker != nil
}

// allowDelivery checks if delivery is allowed based on rate limiting
func (wp *WebhookPusher) allowDelivery() bool {
	wp.mu.Lock()
	defer wp.mu.Unlock()

	now := time.Now()
	if now.Sub(wp.lastDelivery) >= time.Minute {
		wp.eventsThisMin = 0
	}

	config := CurrentConfig
	if config == nil {
		return false
	}

	settings, err := config.GetSettings()
	if err != nil {
		return false
	}

	maxPerMin := settings.WebhookMaxPerMin
	if maxPerMin <= 0 {
		maxPerMin = 60
	}

	if wp.eventsThisMin >= maxPerMin {
		return false
	}

	wp.eventsThisMin += settings.WebhookBatchSize
	if wp.eventsThisMin > maxPerMin {
		wp.eventsThisMin = maxPerMin
	}

	return true
}

// purgeOldEvents deletes events that have outlived the retention
// policy: delivered events older than deliveredEventRetention and
// undelivered events older than undeliveredEventRetention.
func purgeOldEvents() error {
	deliveredCutoff := time.Now().Add(-deliveredEventRetention)
	undeliveredCutoff := time.Now().Add(-undeliveredEventRetention)

	if err := models.DB.RawQuery(
		"DELETE FROM event_streams WHERE delivered_at IS NOT NULL AND delivered_at < ?",
		deliveredCutoff,
	).Exec(); err != nil {
		return fmt.Errorf("failed to purge delivered events: %w", err)
	}

	if err := models.DB.RawQuery(
		"DELETE FROM event_streams WHERE delivered_at IS NULL AND created_at < ?",
		undeliveredCutoff,
	).Exec(); err != nil {
		return fmt.Errorf("failed to purge undelivered events: %w", err)
	}

	return nil
}

// deliverBatch queries undelivered events and sends them to the webhook.
// It returns the number of events in the queried batch (accepted or not), so
// callers can decide whether more events are likely pending.
func deliverBatch() (int, error) {
	config := CurrentConfig
	if config == nil {
		return 0, fmt.Errorf("no config loaded")
	}

	settings, err := config.GetSettings()
	if err != nil {
		return 0, fmt.Errorf("failed to get settings: %w", err)
	}

	// Query undelivered events
	events := &models.EventStreams{}
	err = models.DB.Where("delivered_at IS NULL").
		Order("created_at").
		Limit(settings.WebhookBatchSize).
		All(events)
	if err != nil {
		return 0, fmt.Errorf("failed to query events: %w", err)
	}

	if len(*events) == 0 {
		return 0, nil
	}

	// Build payload
	type webhookEvent struct {
		ID         string          `json:"id"`
		InstanceID string          `json:"instance_id"`
		AnimalID   int             `json:"animal_id"`
		EventType  string          `json:"event_type"`
		Payload    json.RawMessage `json:"payload"`
		CreatedAt  time.Time       `json:"created_at"`
	}

	payloadEvents := make([]webhookEvent, len(*events))
	for i, event := range *events {
		payloadEvents[i] = webhookEvent{
			ID:         event.ID.String(),
			InstanceID: event.InstanceID,
			AnimalID:   event.AnimalID,
			EventType:  event.EventType,
			Payload:    event.Payload,
			CreatedAt:  event.CreatedAt,
		}
	}

	payload := map[string]interface{}{
		"contract_version": 2,
		"instance":         map[string]string{"id": config.InstanceID, "name": config.Name, "description": config.Description},
		"events":           payloadEvents,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return len(*events), fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send HTTP POST
	req, err := http.NewRequest("POST", settings.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return len(*events), fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.WebhookAPIKey)

	resp, err := webhookPusher.client.Do(req)
	if err != nil {
		webhookPusher.circuitBreaker.RecordFailure()
		return len(*events), fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		webhookPusher.circuitBreaker.RecordFailure()
		return len(*events), fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// Determine which events the receiver actually accepted. On partial
	// failure the receiver returns the IDs it processed (processed_ids);
	// events absent from that list are left undelivered (delivered_at IS
	// NULL) so they are retried on the next tick instead of being silently
	// dropped.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return len(*events), fmt.Errorf("failed to read webhook response: %w", err)
	}

	var result struct {
		Processed    *int     `json:"processed"`
		Total        *int     `json:"total"`
		ProcessedIDs []string `json:"processed_ids"`
		Errors       []string `json:"errors"`
	}
	if len(bytes.TrimSpace(bodyBytes)) > 0 {
		if err := json.Unmarshal(bodyBytes, &result); err != nil {
			return len(*events), fmt.Errorf("invalid webhook response: %w", err)
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
		webhookPusher.circuitBreaker.RecordFailure()
	}
	if len(result.ProcessedIDs) == 0 && result.Processed == nil && result.Total == nil && len(result.Errors) == 0 {
		for _, event := range *events {
			accepted[event.ID.String()] = true
		}
	}

	// Mark accepted events as delivered; leave the rest for retry.
	now := time.Now()
	delivered := 0
	for i := range *events {
		event := &(*events)[i]
		if !accepted[event.ID.String()] {
			continue
		}
		event.DeliveredAt = &now
		if err := models.DB.Update(event); err != nil {
			fmt.Printf("Failed to mark event %s as delivered: %v\n", event.ID, err)
			continue
		}
		delivered++
	}

	if responseErr {
		return len(*events), fmt.Errorf("webhook accepted %d/%d events", delivered, len(*events))
	}
	webhookPusher.circuitBreaker.RecordSuccess()
	if delivered < len(*events) {
		fmt.Printf("Delivered %d/%d events to webhook; %d will be retried\n", delivered, len(*events), len(*events)-delivered)
	} else {
		fmt.Printf("Delivered %d events to webhook\n", delivered)
	}

	return len(*events), nil
}

// TriggerWebhookDelivery triggers an immediate webhook delivery attempt
func TriggerWebhookDelivery() {
	if !IsWebhookEnabled() {
		return
	}
	if webhookPusher.circuitBreaker.IsOpen() {
		return
	}
	if !webhookPusher.allowDelivery() {
		return
	}
	if _, err := deliverBatch(); err != nil {
		fmt.Printf("Webhook delivery failed: %v\n", err)
	}
}

// RegisterWebhookShutdown registers graceful shutdown hook
func RegisterWebhookShutdown(app *buffalo.App) {
	events.Listen(func(e events.Event) {
		if e.Kind == buffalo.EvtAppStop {
			StopWebhookWorker()
		}
	})
}

// EnsureWebhookWorkerRunning starts the webhook worker if it is not
// already running. The worker is started unconditionally (even when webhook
// forwarding is disabled) because it also runs the hourly event purge;
// delivery itself remains gated per-tick by IsWebhookEnabled(). It is safe
// to call repeatedly.
func EnsureWebhookWorkerRunning() {
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
	EnsureWebhookWorkerRunning()
	// Boot sweep: pick up events left undelivered by a previous session
	// without waiting for the first fallback tick.
	signalWebhookWake()
}
