package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Handler tests for SyncTargetsResource (multi-hub sync): CRUD from the
// admin UI, per-target delivery counters, and the retry-undeliverable
// action (bugs.md #2). These run against the MySQL test DB: the sqlite
// pusher suite lacks the full application schema (animals, users, ...).

// requireMySQLSuite skips tests that need the full application schema when
// running under the sqlite-tagged pusher suite.
func requireMySQLSuite(t *testing.T) {
	t.Helper()
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}
	if models.DB.Dialect.Name() == "sqlite3" {
		t.Skip("MySQL-only: sqlite pusher suite lacks the full application schema")
	}
}

func seedHandlerSyncTarget(t *testing.T, name, webhookURL string, enabled bool) *models.SyncTarget {
	t.Helper()
	tx := searchTestDB(t)
	target := &models.SyncTarget{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             name,
		Enabled:          enabled,
		WebhookURL:       webhookURL,
		WebhookAPIKey:    "stored-secret-key",
		WebhookBatchSize: 5,
		WebhookMaxPerMin: 120,
	}
	require.NoError(t, tx.Create(target))
	t.Cleanup(func() {
		// Stop the delivery worker BEFORE touching the rows: the worker
		// shares models.DB with request handlers and a pop connection is
		// not goroutine-safe — a still-running worker mid-delivery races
		// with the next test's requests (StopWebhookWorker is synchronous
		// and waits for any in-flight delivery to exit).
		StopWebhookWorker()
		// Reset the "targets exist" short-circuit too: a stale true value
		// makes any later implicit worker start query the DB (and, with a
		// target still visible elsewhere, attempt deliveries) — bugs.md #10.
		SetSyncTargetsKnown(false)
		tx.RawQuery("DELETE FROM event_deliveries WHERE target_id = ?", target.ID.String()).Exec()
		tx.RawQuery("DELETE FROM sync_targets WHERE id = ?", target.ID.String()).Exec()
	})
	return target
}

func reloadSyncTarget(t *testing.T, id uuid.UUID) *models.SyncTarget {
	t.Helper()
	reloaded := &models.SyncTarget{}
	require.NoError(t, searchTestDB(t).Find(reloaded, id))
	return reloaded
}

// Create persists a new target with clamped limits, then the sync
// configuration page lists it.
func TestSyncTargetCreate(t *testing.T) {
	requireMySQLSuite(t)
	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_targets/new")

	name := "TS-target-" + uuid.Must(uuid.NewV4()).String()[:8]
	t.Cleanup(func() {
		// See seedHandlerSyncTarget: the worker must be stopped before the
		// rows go away — the create below wakes it (enabled target with a
		// URL) and it would keep delivering the backlog via models.DB.
		StopWebhookWorker()
		searchTestDB(t).RawQuery("DELETE FROM event_deliveries WHERE target_id IN (SELECT id FROM sync_targets WHERE name = ?)", name).Exec()
		searchTestDB(t).RawQuery("DELETE FROM sync_targets WHERE name = ?", name).Exec()
	})

	resp := postTodoForm(t, client, baseURL, "/sync_targets", token, url.Values{
		"Name":             {name},
		"Enabled":          {"true"},
		"WebhookURL":       {"http://console.example.org/webhook/events"},
		"WebhookAPIKey":    {"new-key-123"},
		"WebhookBatchSize": {"99999"}, // clamped to 100
		"WebhookMaxPerMin": {"0"},     // clamped to 1
	})
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("create status = %d, want redirect", resp.StatusCode)
	}
	// The create woke the delivery worker (enabled target with URL): stop
	// it now so it cannot query models.DB concurrently with the assertions
	// below or the next test's HTTP handlers.
	StopWebhookWorker()

	created := &models.SyncTarget{}
	require.NoError(t, searchTestDB(t).Where("name = ?", name).First(created))
	assert.True(t, created.Enabled)
	assert.Equal(t, "http://console.example.org/webhook/events", created.WebhookURL)
	assert.Equal(t, "new-key-123", created.WebhookAPIKey)
	assert.Equal(t, 100, created.WebhookBatchSize, "batch size clamped to max")
	assert.Equal(t, 1, created.WebhookMaxPerMin, "rate clamped to min")
}

// An enabled target without a webhook URL is rejected with the form
// re-rendered (validation), not silently stored.
func TestSyncTargetCreateEnabledRequiresURL(t *testing.T) {
	requireMySQLSuite(t)
	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_targets/new")

	name := "TS-target-" + uuid.Must(uuid.NewV4()).String()[:8]
	t.Cleanup(func() {
		searchTestDB(t).RawQuery("DELETE FROM sync_targets WHERE name = ?", name).Exec()
	})

	resp := postTodoForm(t, client, baseURL, "/sync_targets", token, url.Values{
		"Name":    {name},
		"Enabled": {"true"},
		// no WebhookURL
	})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("create without URL status = %d, want 422", resp.StatusCode)
	}
	count, err := searchTestDB(t).Where("name = ?", name).Count(&models.SyncTarget{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "invalid target must not be persisted")
}

// Update changes name/URL/limits; a blank API key preserves the stored
// secret (write-only field policy for non-maintainers).
func TestSyncTargetUpdateBlankKeyPreservesStored(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-update", "http://old.example.org/webhook/events", true)

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_targets/"+target.ID.String()+"/edit")

	resp := postTodoForm(t, client, baseURL, "/sync_targets/"+target.ID.String(), token, url.Values{
		"_method":          {"PUT"},
		"Name":             {"TS-update-renamed"},
		"Enabled":          {"true"},
		"WebhookURL":       {"http://new.example.org/webhook/events"},
		"WebhookAPIKey":    {""}, // blank => keep stored
		"WebhookBatchSize": {"7"},
		"WebhookMaxPerMin": {"90"},
	})
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("update status = %d, want redirect", resp.StatusCode)
	}

	reloaded := reloadSyncTarget(t, target.ID)
	assert.Equal(t, "TS-update-renamed", reloaded.Name)
	assert.Equal(t, "http://new.example.org/webhook/events", reloaded.WebhookURL)
	assert.Equal(t, "stored-secret-key", reloaded.WebhookAPIKey, "blank submit must preserve the stored API key")
	assert.Equal(t, 7, reloaded.WebhookBatchSize)
	assert.Equal(t, 90, reloaded.WebhookMaxPerMin)
}

// The edit page must show the stored API key to a plain (non-maintainer)
// admin too: the form is requireAdmin-gated and bugs.md #7 requires the key
// to be visible — not hidden — for admin/maintainer roles.
func TestSyncTargetEditShowsKeyToPlainAdmin(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-edit", "http://console.example.org/webhook/events", true)

	client, baseURL := adminClientWithURL(t)
	resp, err := client.Get(baseURL + "/sync_targets/" + target.ID.String() + "/edit")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, _ := io.ReadAll(resp.Body)
	html := string(body)
	assert.Contains(t, html, "stored-secret-key", "plain admin must see the stored API key")
	assert.Contains(t, html, `type="text"`, "the API key input must not be masked for admins")
}

// Destroy removes the target AND its per-target delivery rows.
func TestSyncTargetDestroyRemovesDeliveries(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-destroy", "http://console.example.org/webhook/events", true)
	tx := searchTestDB(t)

	// One delivery row hanging off the target.
	ev := &models.EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "cfgtest-scope",
		AnimalID:   424242,
		EventType:  string(models.EventTypeAnimalState),
		Payload:    []byte(`{"animal":{}}`),
		CreatedAt:  time.Now(),
	}
	require.NoError(t, tx.Create(ev))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM event_streams WHERE id = ?", ev.ID.String()).Exec()
	})
	require.NoError(t, tx.Create(&models.EventDelivery{
		ID:        uuid.Must(uuid.NewV4()),
		EventID:   ev.ID,
		TargetID:  target.ID,
		Attempts:  3,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}))

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_targets/"+target.ID.String()+"/edit")

	resp := postTodoForm(t, client, baseURL, "/sync_targets/"+target.ID.String(), token, url.Values{
		"_method": {"DELETE"},
	})
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("destroy status = %d, want redirect", resp.StatusCode)
	}

	count, err := tx.Where("id = ?", target.ID).Count(&models.SyncTarget{})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "target row deleted")
	dCount, err := tx.Where("target_id = ?", target.ID.String()).Count(&models.EventDelivery{})
	require.NoError(t, err)
	assert.Equal(t, 0, dCount, "delivery rows deleted with the target")
}

// RetryUndeliverable resets capped delivery rows so the events re-enter the
// queue (bugs.md #2).
func TestSyncTargetRetryUndeliverable(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-retry", "http://console.example.org/webhook/events", true)
	tx := searchTestDB(t)

	ev := &models.EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "cfgtest-scope",
		AnimalID:   424243,
		EventType:  string(models.EventTypeAnimalState),
		Payload:    []byte(`{"animal":{}}`),
		CreatedAt:  time.Now(),
	}
	require.NoError(t, tx.Create(ev))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM event_streams WHERE id = ?", ev.ID.String()).Exec()
	})

	errMsg := "webhook POST returned status 401"
	// One undeliverable row (attempts at the cap) and one still-pending row
	// (below the cap) — only the capped one may be reset.
	capped := &models.EventDelivery{
		ID:        uuid.Must(uuid.NewV4()),
		EventID:   ev.ID,
		TargetID:  target.ID,
		Attempts:  models.MaxDeliveryAttempts,
		LastError: &errMsg,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, tx.Create(capped))

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/sync_configuration")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/sync_targets/"+target.ID.String()+"/retry_undeliverable", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	// buffalo v1.1.4 CSRF no longer reads the token from the query string:
	// send it in the X-CSRF-Token header instead.
	req.Header.Set("X-CSRF-Token", token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		Requeued int `json:"requeued"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, 1, out.Requeued, "one capped row re-queued")

	reloaded := &models.EventDelivery{}
	require.NoError(t, tx.Find(reloaded, capped.ID))
	assert.Equal(t, 0, reloaded.Attempts, "attempt counter reset")
	assert.Nil(t, reloaded.LastError, "last error cleared")
}
