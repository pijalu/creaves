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
	stopChan      chan bool
	pusherMu      sync.Mutex
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

	ticker := time.NewTicker(5 * time.Second)
	stop := make(chan bool)
	// Publish for StopWebhookWorker / IsWebhookWorkerRunning coordination.
	webhookTicker = ticker
	stopChan = stop

	go func() {
		// Reference the LOCAL ticker and stop channel so that
		// StopWebhookWorker nil-ing the package globals cannot race with a
		// delivery already in flight (previously this read the global
		// webhookTicker directly and could nil-deref on shutdown).
		for {
			select {
			case <-ticker.C:
				if !IsWebhookEnabled() {
					continue
				}
				if webhookPusher.circuitBreaker.IsOpen() {
					continue
				}
				if !webhookPusher.allowDelivery() {
					continue
				}
				if err := deliverBatch(); err != nil {
					fmt.Printf("Webhook delivery failed: %v\n", err)
				}
			case <-stop:
				return
			}
		}
	}()

	fmt.Println("Webhook worker started")
}

// StopWebhookWorker stops the background webhook delivery worker
func StopWebhookWorker() {
	pusherMu.Lock()
	defer pusherMu.Unlock()

	if webhookTicker != nil {
		webhookTicker.Stop()
		close(stopChan)
		webhookTicker = nil
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

// deliverBatch queries undelivered events and sends them to the webhook
func deliverBatch() error {
	config := CurrentConfig
	if config == nil {
		return fmt.Errorf("no config loaded")
	}

	settings, err := config.GetSettings()
	if err != nil {
		return fmt.Errorf("failed to get settings: %w", err)
	}

	// Query undelivered events
	events := &models.EventStreams{}
	err = models.DB.Where("delivered_at IS NULL").
		Order("created_at").
		Limit(settings.WebhookBatchSize).
		All(events)
	if err != nil {
		return fmt.Errorf("failed to query events: %w", err)
	}

	if len(*events) == 0 {
		return nil
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
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	// Send HTTP POST
	req, err := http.NewRequest("POST", settings.WebhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+settings.WebhookAPIKey)

	resp, err := webhookPusher.client.Do(req)
	if err != nil {
		webhookPusher.circuitBreaker.RecordFailure()
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		webhookPusher.circuitBreaker.RecordFailure()
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}

	// Determine which events the receiver actually accepted. On partial
	// failure the receiver returns the IDs it processed (processed_ids);
	// events absent from that list are left undelivered (delivered_at IS
	// NULL) so they are retried on the next tick instead of being silently
	// dropped.
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read webhook response: %w", err)
	}

	var result struct {
		ProcessedIDs []string `json:"processed_ids"`
	}
	_ = json.Unmarshal(bodyBytes, &result)

	accepted := make(map[string]bool, len(*events))
	if len(result.ProcessedIDs) > 0 {
		for _, id := range result.ProcessedIDs {
			accepted[id] = true
		}
	} else {
		// Backward compatibility: a receiver that does not report
		// processed_ids is assumed to have accepted every event on 200 OK.
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

	webhookPusher.circuitBreaker.RecordSuccess()
	if delivered < len(*events) {
		fmt.Printf("Delivered %d/%d events to webhook; %d will be retried\n", delivered, len(*events), len(*events)-delivered)
	} else {
		fmt.Printf("Delivered %d events to webhook\n", delivered)
	}

	return nil
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
	if err := deliverBatch(); err != nil {
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

// EnsureWebhookWorkerRunning starts the webhook worker if the webhook is
// enabled and it is not already running. It is safe to call repeatedly.
func EnsureWebhookWorkerRunning() {
	if IsWebhookEnabled() && !IsWebhookWorkerRunning() {
		StartWebhookWorker()
	}
}

// InitWebhookAtBoot loads the configuration from the database and starts the
// webhook delivery worker when forwarding is enabled. It must be called at
// application startup, once the database connection (models.DB) is available,
// so that events queued before a restart are still delivered. A failure to
// load the configuration is logged but never fatal: the worker remains stopped
// and may be started lazily later (e.g. when the next event is published).
func InitWebhookAtBoot() {
	if _, err := LoadConfig(models.DB); err != nil {
		fmt.Printf("Webhook: failed to load config at boot: %v\n", err)
		return
	}
	if err := RecoverInterruptedRuns(models.DB); err != nil {
		fmt.Printf("Webhook: failed to recover resync runs: %v\n", err)
	}
	EnsureWebhookWorkerRunning()
}
