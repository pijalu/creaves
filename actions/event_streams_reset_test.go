package actions

import (
	"encoding/json"
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Handler tests for the event-stream DLQ feature (bugs.md "Event stream —
// reset attempts, visible attempts, DLQ category"): per-event reset,
// bulk DLQ reset, and the undeliverable filter clause. MySQL-only like the
// sync target handler tests (full schema required).

// seedResettableEvent creates one event with one capped (DLQ) and one
// still-pending delivery row, plus cleanup.
func seedResettableEvent(t *testing.T, target *models.SyncTarget) (*models.EventStream, *models.EventDelivery) {
	t.Helper()
	tx := searchTestDB(t)

	ev := &models.EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "dlq-test",
		AnimalID:   990001,
		EventType:  string(models.EventTypeAnimalState),
		Payload:    []byte(`{"animal":{}}`),
		CreatedAt:  time.Now(),
	}
	require.NoError(t, tx.Create(ev))

	errMsg := "dial tcp: connection refused"
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

	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM event_deliveries WHERE event_id = ?", ev.ID.String()).Exec()
		tx.RawQuery("DELETE FROM event_streams WHERE id = ?", ev.ID.String()).Exec()
	})
	return ev, capped
}

// TestDeliveryFilterClauseUndeliverable pins the DLQ clause mapping.
func TestDeliveryFilterClauseUndeliverable(t *testing.T) {
	clause := deliveryFilterClause("undeliverable")
	assert.Contains(t, clause, "event_deliveries")
	assert.Contains(t, clause, "delivered_at IS NULL")
	assert.Contains(t, clause, "attempts >= 25")
	// existing mappings unchanged
	assert.Equal(t, "", deliveryFilterClause("bogus"))
}

// TestEventStreamsResetAttempts posts to the per-event reset endpoint and
// verifies the capped row re-enters the queue (attempts=0, error cleared).
func TestEventStreamsResetAttempts(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-es-reset", "http://console.example.org/webhook/events", true)
	ev, capped := seedResettableEvent(t, target)

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/event_streams")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/event_streams/"+ev.ID.String()+"/reset_attempts", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	req.URL.RawQuery = "authenticity_token=" + url.QueryEscape(token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		Requeued int `json:"requeued"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	assert.Equal(t, 1, out.Requeued, "one delivery row re-queued")

	tx := searchTestDB(t)
	reloaded := &models.EventDelivery{}
	require.NoError(t, tx.Find(reloaded, capped.ID))
	assert.Equal(t, 0, reloaded.Attempts, "attempt counter reset")
	assert.Nil(t, reloaded.LastError, "last error cleared")
}

// TestEventStreamsResetAttempts404 posts to the reset endpoint with an
// unknown event id and expects a 404.
func TestEventStreamsResetAttempts404(t *testing.T) {
	requireMySQLSuite(t)
	client, baseURL := adminClientWithURL(t)

	req, err := http.NewRequest(http.MethodPost, baseURL+"/event_streams/"+uuid.Must(uuid.NewV4()).String()+"/reset_attempts", nil)
	require.NoError(t, err)
	// The endpoint is JSON (see the sibling reset tests); the Accept header
	// must be set so the request matches how the endpoint is consumed — and
	// mw-csrf's HTML-form inspection path is not applicable to it.
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// TestEventStreamsResetUndeliverable posts to the bulk DLQ reset endpoint
// and verifies only capped rows are reset (pending rows below the cap are
// untouched — they still carry their attempt counter).
func TestEventStreamsResetUndeliverable(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-es-reset-bulk", "http://console.example.org/webhook/events", true)
	ev, capped := seedResettableEvent(t, target)

	tx := searchTestDB(t)
	// Second delivery row below the cap — must NOT be touched by the bulk
	// reset (it is still in the normal retry queue).
	pending := &models.EventDelivery{
		ID:        uuid.Must(uuid.NewV4()),
		EventID:   ev.ID,
		TargetID:  uuid.Must(uuid.NewV4()), // other (nonexistent) target: fine for the counter check
		Attempts:  3,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	require.NoError(t, tx.Create(pending))

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/event_streams")

	req, err := http.NewRequest(http.MethodPost, baseURL+"/event_streams/reset_undeliverable", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	req.URL.RawQuery = "authenticity_token=" + url.QueryEscape(token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var out struct {
		Requeued int `json:"requeued"`
	}
	require.NoError(t, json.NewDecoder(resp.Body).Decode(&out))
	// At least our capped row was reset (other DLQ rows from parallel
	// failures may join in a dirty dev-test DB, hence >=).
	assert.GreaterOrEqual(t, out.Requeued, 1, "capped row re-queued")

	reloaded := &models.EventDelivery{}
	require.NoError(t, tx.Find(reloaded, capped.ID))
	assert.Equal(t, 0, reloaded.Attempts, "capped row reset")

	untouched := &models.EventDelivery{}
	require.NoError(t, tx.Find(untouched, pending.ID))
	assert.Equal(t, 3, untouched.Attempts, "below-cap row untouched")
}

// TestEventStreamsListDLQFilter verifies the undeliverable filter returns
// the DLQ event (JSON endpoint) and hides it under the delivered filter.
// The delivery row points at a DISABLED target so the pusher worker (alive
// during the test app run) never picks it up mid-assertion.
func TestEventStreamsListDLQFilter(t *testing.T) {
	requireMySQLSuite(t)
	target := seedHandlerSyncTarget(t, "TS-es-dlq-filter", "http://console.example.org/webhook/events", false)
	ev, _ := seedResettableEvent(t, target)

	client, baseURL := adminClientWithURL(t)

	get := func(query string) []map[string]interface{} {
		req, err := http.NewRequest(http.MethodGet, baseURL+"/event_streams?"+query, nil)
		require.NoError(t, err)
		req.Header.Set("Accept", "application/json")
		resp, err := client.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		var rows []map[string]interface{}
		require.NoError(t, json.NewDecoder(resp.Body).Decode(&rows))
		return rows
	}

	findEvent := func(rows []map[string]interface{}) map[string]interface{} {
		for _, r := range rows {
			if r["id"] == ev.ID.String() {
				return r
			}
		}
		return nil
	}

	dlq := get("delivery=undeliverable&per_page=300")
	row := findEvent(dlq)
	require.NotNil(t, row, "DLQ event listed under delivery=undeliverable")
	assert.Equal(t, true, row["Blocked"], "blocked aggregate exposed")
	assert.EqualValues(t, models.MaxDeliveryAttempts, row["Attempts"], "attempts aggregate exposed")

	delivered := get("delivery=delivered&per_page=300")
	assert.Nil(t, findEvent(delivered), "DLQ event hidden under delivery=delivered")
}
