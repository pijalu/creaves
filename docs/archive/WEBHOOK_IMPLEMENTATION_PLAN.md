# Webhook Event Architecture Implementation Plan

## Overview
Transition from MySQL polling to webhook-based event push architecture. Creaves instances push events to Creaves Console via authenticated HTTP webhooks with circuit breaker and batching support.

**Status:** Ready for Implementation  
**Last Updated:** 2026-04-24

---

## User Clarifications Applied

1. **Webhook Processing:** Synchronous (process events immediately in HTTP handler)
2. **Database Cleanup:** Drop all MySQL import leftovers (SourceInstance table)
3. **Graceful Shutdown:** Hook into Buffalo shutdown signal to drain pending deliveries
4. **Rate Limiting:** Client-side configuration to limit events per minute

---

## Part A: Creaves Console — Webhook Reception

### A1. Database Changes

#### Drop Legacy SourceInstance Table
**Migration:** `drop_source_instances` (run before creating new tables)
- Drop table `source_instances`
- Drop associated indexes

#### Create Webhook API Keys Table
**Migration:** `create_webhook_api_keys.up.fizz`
```
create_table("webhook_api_keys") {
    t.Column("id", "uuid", {primary: true})
    t.Column("name", "string", {})
    t.Column("key_hash", "string", {})
    t.Column("key_prefix", "string", {})
    t.Column("instance_id", "string", {null: true})
    t.Column("active", "bool", {default: true})
    t.Column("last_used_at", "timestamp", {null: true})
    t.Timestamps()
}
add_index("webhook_api_keys", ["instance_id"], {})
add_index("webhook_api_keys", ["key_hash"], {unique: true})
```

### A2. New Model: `models/webhook_api_key.go`

**Fields:**
- `ID uuid.UUID` - Primary key
- `Name string` - Human-readable label
- `KeyHash string` - bcrypt hash of API key
- `KeyPrefix string` - First 8 characters for display
- `InstanceID string` - Optional restriction to specific instance
- `Active bool` - Enable/disable key
- `LastUsedAt *time.Time` - Last successful authentication
- `CreatedAt/UpdatedAt time.Time`

**Methods:**
- `Validate()` - Name required, KeyHash required
- `Authenticate(rawKey string) bool` - bcrypt compare
- `String()` - JSON representation

### A3. New Actions: `actions/webhook_api_keys.go`

**Full CRUD Resource:** `WebhookAPIKeysResource`
- `GET /webhook_api_keys` - List all keys (admin only)
- `GET /webhook_api_keys/new` - Create form
- `POST /webhook_api_keys` - Create key (generate raw key, hash it, show once)
- `GET /webhook_api_keys/{id}` - Show key details
- `GET /webhook_api_keys/{id}/edit` - Edit form (name, active only)
- `PUT /webhook_api_keys/{id}` - Update
- `DELETE /webhook_api_keys/{id}` - Delete

**Key Generation:**
- Raw key format: `creaves_<uuid>`
- Store bcrypt hash in DB
- Show raw key **only once** in flash message after creation
- Display prefix in list view

### A4. New Webhook Receiver: `actions/webhook.go`

**Endpoint:** `POST /webhook/events`

**Authentication:**
- Header: `Authorization: Bearer <api_key>`
- Skip session auth middleware for this route
- Lookup key by hash prefix, verify with bcrypt
- Update `LastUsedAt` on success

**Request Body:**
```json
{
  "events": [
    {
      "id": "uuid",
      "instance_id": "center-brussels",
      "animal_id": 123,
      "event_type": "animal_discovered",
      "payload": {...},
      "created_at": "2026-04-24T10:00:00Z"
    }
  ]
}
```

**Processing:**
1. Authenticate API key
2. Validate request body
3. Insert events into `event_streams` (set `imported_at = NOW()`)
4. Process events immediately (synchronous):
   - For each event, call `eventProcessor.processEvent()`
   - Update `consolidated_animals` table
5. Return `200 OK` with summary

**Responses:**
- `200 OK` - Events processed successfully
- `401 Unauthorized` - Invalid or missing API key
- `422 Unprocessable Entity` - Invalid request body

### A5. Remove Legacy Import Infrastructure

**Delete Files:**
- [ ] `models/source_instance.go`
- [ ] `actions/source_instances.go`
- [ ] `actions/event_importer.go`

**Remove Routes from `actions/app.go`:**
- [ ] `app.GET("/import", ImportHandler)`
- [ ] `app.GET("/import/:source_instance_id", ImportFromSourceHandler)`
- [ ] `app.Resource("/source_instances", SourceInstancesResource{})`

**Update Files:**
- [ ] `actions/dashboard.go` - Remove SourceInstance stats, show unique instance count from event_streams
- [ ] `actions/consolidation_runner.go` - Remove import calls
- [ ] `actions/event_processor.go` - Keep only, verify it processes events correctly
- [ ] `templates/application.plush.html` - Replace "Source Instances" with "Webhook API Keys" in nav
- [ ] `templates/dashboard/index.plush.html` - Remove Import button, update stats display

### A6. Update EventStream Model

**File:** `models/event_stream.go`

**Changes:**
- `SourceDB` field: Leave in struct but stop populating (will be empty string)
- `ImportedAt` field: Keep, now means "received at via webhook"
- Add comment noting `SourceDB` is deprecated

### A7. Templates to Create

**Directory:** `templates/webhook_api_keys/`

- [ ] `index.plush.html` - List keys with prefix, active status, last used
- [ ] `new.plush.html` - Create form (name only, generates key)
- [ ] `edit.plush.html` - Edit form (name, active checkbox)
- [ ] `show.plush.html` - Key details, show full key only if just created

---

## Part B: Creaves App — Webhook Configuration & Pusher

### B1. Database Changes

#### Add delivered_at to Event Stream
**Migration:** `add_delivered_at_to_event_streams.up.fizz`
```
add_column("event_streams", "delivered_at", "timestamp", {null: true})
add_index("event_streams", ["delivered_at"], {})
```

**Update Model:** `models/event_stream.go`
- Add `DeliveredAt *time.Time` field
- Update `TableName()` if needed

### B2. Extend Config Settings

**File:** `models/config.go`

**Update ConfigSettings struct:**
```go
type ConfigSettings struct {
    EnableEventStream  bool   `json:"enable_event_stream"`
    WebhookEnabled     bool   `json:"webhook_enabled"`
    WebhookURL         string `json:"webhook_url"`
    WebhookAPIKey      string `json:"webhook_api_key"`      // Plaintext in DB
    WebhookBatchSize   int    `json:"webhook_batch_size"`   // Default: 1
    WebhookMaxPerMin   int    `json:"webhook_max_per_min"`  // Default: 60 (1/sec)
}
```

**Update DefaultSettings():**
```go
return ConfigSettings{
    EnableEventStream:  true,
    WebhookEnabled:     false,
    WebhookBatchSize:   1,
    WebhookMaxPerMin:   60,
}
```

### B3. Update Config Forms

**File:** `templates/config/_form.plush.html`

**Add Webhook Section:**
```html
<hr/>
<h5>Webhook Configuration</h5>
<p class="text-muted">Configure webhook to push events to Creaves Console.</p>

<!-- Webhook Enabled -->
<div class="form-group form-check">
  <input type="hidden" name="Settings.WebhookEnabled" value="false" />
  <input type="checkbox" class="form-check-input" id="webhook_enabled" 
         name="Settings.WebhookEnabled" value="true" 
         <%= if (settings.WebhookEnabled) { %>checked<% } %>>
  <label class="form-check-label" for="webhook_enabled">Enable Webhook</label>
</div>

<!-- Webhook URL -->
<%= f.InputTag("WebhookURL", {label: "Webhook URL", placeholder: "https://console.example.com/webhook/events"}) %>

<!-- API Key -->
<div class="form-group">
  <label for="webhook_api_key">API Key</label>
  <input type="password" class="form-control" id="webhook_api_key" 
         name="Settings.WebhookAPIKey" value="<%= settings.WebhookAPIKey %>">
  <small class="text-muted">API key from Creaves Console</small>
</div>

<!-- Batch Size -->
<%= f.InputTag("WebhookBatchSize", {label: "Batch Size", type: "number", min: 1, value: settings.WebhookBatchSize}) %>

<!-- Max Per Minute -->
<%= f.InputTag("WebhookMaxPerMin", {label: "Max Events Per Minute", type: "number", min: 1, value: settings.WebhookMaxPerMin}) %>
```

**Update:** `templates/config/show.plush.html` - Display webhook settings (mask API key)

### B4. Update Config Handlers

**File:** `actions/configs.go`

**Update Create handler:**
- Bind new fields: `Settings.WebhookEnabled`, `Settings.WebhookURL`, `Settings.WebhookAPIKey`, `Settings.WebhookBatchSize`, `Settings.WebhookMaxPerMin`
- Validate: if `WebhookEnabled`, URL must be non-empty and valid HTTPS in production

**Update Update handler:**
- Same binding as Create
- Update cached config if this is the current active config

### B5. Create Webhook Pusher: `actions/webhook_pusher.go`

**Types:**
```go
type WebhookPusher struct {
    client         *http.Client
    circuitBreaker *CircuitBreaker
    lastDelivery   time.Time
    eventsThisMin  int
    mu             sync.RWMutex
}

type CircuitBreaker struct {
    mu           sync.RWMutex
    failures     int
    lastFailure  time.Time
    state        string // "closed", "open", "half-open"
    failureThreshold int // 5
    resetTimeout     time.Duration // 60s
}
```

**Worker (Ticker-based):**
```go
var (
    webhookPusher *WebhookPusher
    webhookTicker *time.Ticker
    stopChan      chan bool
)

func init() {
    // Initialize after config is loaded (lazy init on first need)
}

func StartWebhookWorker() {
    if webhookTicker != nil {
        return // Already running
    }
    
    webhookTicker = time.NewTicker(5 * time.Second)
    stopChan = make(chan bool)
    
    go func() {
        for {
            select {
            case <-webhookTicker.C:
                if !webhookConfigured() {
                    continue
                }
                if webhookPusher.circuitBreaker.IsOpen() {
                    continue
                }
                if !webhookPusher.allowDelivery() {
                    continue // Rate limit hit
                }
                deliverBatch()
            case <-stopChan:
                return
            }
        }
    }()
}

func StopWebhookWorker() {
    if webhookTicker != nil {
        webhookTicker.Stop()
        close(stopChan)
        webhookTicker = nil
    }
}
```

**Rate Limiting:**
```go
func (wp *WebhookPusher) allowDelivery() bool {
    wp.mu.Lock()
    defer wp.mu.Unlock()
    
    now := time.Now()
    if now.Sub(wp.lastDelivery) >= time.Minute {
        wp.eventsThisMin = 0
    }
    
    config := GetCurrentConfig()
    settings, _ := config.GetSettings()
    maxPerMin := settings.WebhookMaxPerMin
    if maxPerMin <= 0 {
        maxPerMin = 60
    }
    
    if wp.eventsThisMin >= maxPerMin {
        return false
    }
    
    wp.eventsThisMin++
    return true
}
```

**Circuit Breaker:**
```go
func (cb *CircuitBreaker) IsOpen() bool {
    cb.mu.RLock()
    defer cb.mu.RUnlock()
    
    if cb.state == "open" {
        if time.Since(cb.lastFailure) > cb.resetTimeout {
            cb.state = "half-open"
            return false
        }
        return true
    }
    return false
}

func (cb *CircuitBreaker) RecordSuccess() {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    cb.failures = 0
    cb.state = "closed"
}

func (cb *CircuitBreaker) RecordFailure() {
    cb.mu.Lock()
    defer cb.mu.Unlock()
    cb.failures++
    cb.lastFailure = time.Now()
    if cb.failures >= cb.failureThreshold {
        cb.state = "open"
    }
}
```

**Delivery Logic:**
```go
func deliverBatch() error {
    config := GetCurrentConfig()
    settings, _ := config.GetSettings()
    
    // Query undelivered events
    events := &models.EventStreams{}
    err := models.DB.Where("delivered_at IS NULL").
        Order("created_at").
        Limit(settings.WebhookBatchSize).
        All(events)
    if err != nil {
        return err
    }
    
    if len(*events) == 0 {
        return nil
    }
    
    // Build payload
    payload := buildWebhookPayload(events)
    
    // Send HTTP POST
    req, _ := http.NewRequest("POST", settings.WebhookURL, bytes.NewBuffer(payload))
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("Authorization", "Bearer "+settings.WebhookAPIKey)
    
    resp, err := webhookPusher.client.Do(req)
    if err != nil {
        webhookPusher.circuitBreaker.RecordFailure()
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        webhookPusher.circuitBreaker.RecordFailure()
        return fmt.Errorf("webhook returned %d", resp.StatusCode)
    }
    
    // Mark as delivered
    tx := models.DB
    for _, event := range *events {
        now := time.Now()
        event.DeliveredAt = &now
        tx.Update(event)
    }
    
    webhookPusher.circuitBreaker.RecordSuccess()
    return nil
}
```

**Integration with Event Producer:**
```go
// In PublishEvent, after tx.Create(event):
if IsWebhookEnabled() && webhookTicker == nil {
    StartWebhookWorker()
}
```

**Graceful Shutdown:**
```go
// In cmd/app/main.go or actions/app.go shutdown handler
buffalo.BeforeShutdown(func() {
    StopWebhookWorker()
    // Drain remaining events?
})
```

### B6. Webhook Enable Check

**File:** `actions/configs.go`

**Add function:**
```go
func IsWebhookEnabled() bool {
    if CurrentConfig == nil {
        return false
    }
    settings, err := CurrentConfig.GetSettings()
    if err != nil {
        return false
    }
    return settings.WebhookEnabled && settings.WebhookURL != ""
}
```

---

## Part C: Migration Execution Order

### Creaves Console (Run First)
1. `drop_source_instances.down.fizz` + `drop_source_instances.up.fizz` - Remove legacy table
2. `create_webhook_api_keys.up.fizz` - New API key table

### Creaves App (Run Second)
1. `add_delivered_at_to_event_streams.up.fizz` - Track delivery status

---

## Part D: Testing Checklist

### Creaves Console Tests
- [ ] API key creation shows raw key once
- [ ] API key list shows prefix, not full key
- [ ] API key authentication works (bcrypt compare)
- [ ] Webhook rejects invalid bearer token (401)
- [ ] Webhook rejects missing auth header (401)
- [ ] Webhook accepts valid batched events (200)
- [ ] Events are processed synchronously into consolidated_animals
- [ ] Dashboard shows unique instance count from event_streams
- [ ] Source instances menu item removed
- [ ] Import button removed from dashboard

### Creaves App Tests
- [ ] Config form shows webhook section
- [ ] Config save persists webhook settings
- [ ] API key masked in config show page
- [ ] Event creation leaves `delivered_at = NULL`
- [ ] Worker starts when webhook enabled
- [ ] Worker delivers batch to console
- [ ] Batch size respected (test with size 1 and size 10)
- [ ] Rate limiting works (max per minute)
- [ ] Circuit breaker opens after 5 failures
- [ ] Circuit breaker closes after successful delivery
- [ ] Graceful shutdown drains pending deliveries

### End-to-End Tests
- [ ] Create animal in Creaves → appears in Console dashboard within 10 seconds
- [ ] Disable webhook → events stay undelivered
- [ ] Re-enable webhook → backlog delivers
- [ ] Console down → circuit breaker opens, Creaves stops trying
- [ ] Console recovers → circuit breaker closes, delivery resumes

---

## Part E: Implementation Order

**Phase 1: Creaves Console Foundation**
1. [ ] Create `models/webhook_api_key.go`
2. [ ] Create `migrations/create_webhook_api_keys`
3. [ ] Create `actions/webhook_api_keys.go`
4. [ ] Create `templates/webhook_api_keys/*.plush.html`
5. [ ] Add routes to `actions/app.go`

**Phase 2: Creaves Console Cleanup**
6. [ ] Create migration to drop `source_instances` table
7. [ ] Delete `models/source_instance.go`
8. [ ] Delete `actions/source_instances.go`
9. [ ] Delete `actions/event_importer.go`
10. [ ] Remove import routes from `actions/app.go`
11. [ ] Update `actions/dashboard.go` (remove SourceInstance stats)
12. [ ] Update `actions/consolidation_runner.go`
13. [ ] Update `templates/application.plush.html`
14. [ ] Update `templates/dashboard/index.plush.html`

**Phase 3: Creaves Console Webhook Receiver**
15. [ ] Create `actions/webhook.go`
16. [ ] Add webhook route to `actions/app.go` (skip auth)
17. [ ] Update `models/event_stream.go` (deprecate SourceDB)

**Phase 4: Creaves App Database**
18. [ ] Create migration `add_delivered_at_to_event_streams`
19. [ ] Update `models/event_stream.go` (add DeliveredAt)

**Phase 5: Creaves App Config**
20. [ ] Update `models/config.go` (extend ConfigSettings)
21. [ ] Update `actions/configs.go` (bind new fields, add IsWebhookEnabled)
22. [ ] Update `templates/config/_form.plush.html`
23. [ ] Update `templates/config/show.plush.html`

**Phase 6: Creaves App Webhook Pusher**
24. [ ] Create `actions/webhook_pusher.go`
25. [ ] Add graceful shutdown hook in `cmd/app/main.go`
26. [ ] Update `actions/event_producer.go` (start worker on event)

**Phase 7: Testing**
27. [ ] Run migrations
28. [ ] Test Creaves Console API key CRUD
29. [ ] Test webhook authentication
30. [ ] Test end-to-end event flow
31. [ ] Test circuit breaker and rate limiting
32. [ ] Test graceful shutdown

---

## Part F: Notes

### API Key Format
- Raw key: `creaves_<uuid>` (e.g., `creaves_550e8400-e29b-41d4-a716-446655440000`)
- Hash with bcrypt (cost 10)
- Store prefix: first 8 chars of uuid portion

### Batch Behavior
- Query: `WHERE delivered_at IS NULL ORDER BY created_at LIMIT ?`
- Mark batch delivered only after HTTP 200
- Failed batch stays undelivered, retried on next tick

### Circuit Breaker States
- **Closed:** Normal operation, delivery allowed
- **Open:** 5 consecutive failures, stop for 60s
- **Half-Open:** After 60s, allow 1 test delivery
- Success in half-open → closed, failure → open

### Rate Limiting
- Reset counter every minute
- Configurable per minute (default 60)
- Excess events wait for next delivery window

### Synchronous Processing (Console)
- Webhook handler processes events immediately
- Updates consolidated_animals before returning HTTP 200
- No background processor needed in console
- Grift tasks can still reprocess if needed

---

## References

- Creaves: `/Users/muaddib/dev/creaves.project/creaves/`
- Creaves Console: `/Users/muaddib/dev/creaves.project/creaves-console/`
- Buffalo Docs: https://gobuffalo.io/documentation/
- Pop ORM Docs: https://github.com/gobuffalo/pop
