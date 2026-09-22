package actions

import (
	"creaves/models"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// EventStreamsResource is the resource for the EventStream model
type EventStreamsResource struct {
	buffalo.Resource
}

// eventStreamRow decorates one EventStream with the delivery state
// aggregated over its event_deliveries rows (the legacy
// event_streams.delivery_attempts rollup column is no longer written, so the
// index derives attempts/blocked from the real per-target rows).
type eventStreamRow struct {
	models.EventStream
	// Attempts is the max attempts counter over the event's delivery rows.
	Attempts int `db:"attempts"`
	// Blocked is true when at least one target delivery is pending and
	// exhausted its retry budget (dead-letter queue membership).
	Blocked bool `db:"blocked"`
}

// eventStreamDeliveryView joins one event_deliveries row with its target
// name for the per-target attempts table on the show page.
type eventStreamDeliveryView struct {
	TargetName     string     `db:"target_name"`
	Attempts       int        `db:"attempts"`
	DeliveredAt    *time.Time `db:"delivered_at"`
	AcknowledgedAt *time.Time `db:"acknowledged_at"`
	LastError      *string    `db:"last_error"`
}

// Undeliverable reports whether this delivery row sits in the DLQ.
func (v eventStreamDeliveryView) Undeliverable() bool {
	return v.DeliveredAt == nil && v.Attempts >= models.MaxDeliveryAttempts
}

// listEventsSelect aggregates the per-target delivery state onto each event
// row. attempts = highest attempts counter (worst target); blocked = DLQ
// membership of at least one target.
const listEventsSelect = `SELECT event_streams.*,
	COALESCE(d.attempts, 0) AS attempts,
	COALESCE(d.blocked, 0) AS blocked
	FROM event_streams
	LEFT JOIN (
		SELECT event_id,
			MAX(attempts) AS attempts,
			MAX(CASE WHEN delivered_at IS NULL AND attempts >= %d THEN 1 ELSE 0 END) AS blocked
		FROM event_deliveries GROUP BY event_id
	) d ON d.event_id = event_streams.id`

// dlqClause matches events with at least one delivery row that exhausted its
// retry budget without being delivered (dead-letter queue).
func dlqClause() string {
	return fmt.Sprintf(`EXISTS (SELECT 1 FROM event_deliveries d WHERE d.event_id = event_streams.id AND d.delivered_at IS NULL AND d.attempts >= %d)`, models.MaxDeliveryAttempts)
}

// List gets all EventStreams. This function is mapped to the path
// GET /event_streams
func (v EventStreamsResource) List(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// pop's RawQuery ignores Where()/Order() modifiers (raw SQL mode), so
	// the filter and ordering are embedded directly into the statement.
	// deliveryFilterClause only emits constant clauses, no user input.
	sql := fmt.Sprintf(listEventsSelect, models.MaxDeliveryAttempts)
	if clause := deliveryFilterClause(c.Param("delivery")); clause != "" {
		sql += " WHERE " + clause
	}
	sql += " ORDER BY event_streams.created_at DESC"

	rows := &[]eventStreamRow{}
	q := tx.RawQuery(sql)
	paginated := tx.PaginateFromParams(c.Params())
	q.Paginator = paginated.Paginator

	if err := q.All(rows); err != nil {
		return errors.WithStack(err)
	}

	// dlqCount feeds the DLQ filter button badge.
	var dlqCount int
	countQ := fmt.Sprintf("SELECT COUNT(*) FROM event_streams WHERE %s", dlqClause())
	if err := tx.RawQuery(countQ).First(&dlqCount); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("pagination", q.Paginator)
		c.Set("eventStreams", rows)
		c.Set("deliveryFilter", c.Param("delivery"))
		c.Set("dlqCount", dlqCount)
		c.Set("maxAttempts", models.MaxDeliveryAttempts)
		return c.Render(http.StatusOK, r.HTML("event_streams/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(rows))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(rows))
	}).Respond(c)
}

// deliveryFilterClause maps the `delivery` query param to a SQL WHERE clause.
// Supported values: "" / "all" (no filter), "not-delivered" (delivered_at IS
// NULL), "delivered" (delivered_at IS NOT NULL) and "undeliverable" (DLQ:
// at least one pending delivery row at the attempts cap). Unknown values fall
// back to no filter so a bad link cannot blank the listing.
func deliveryFilterClause(delivery string) string {
	switch delivery {
	case "delivered":
		return "delivered_at IS NOT NULL"
	case "not-delivered":
		return "delivered_at IS NULL"
	case "undeliverable":
		return dlqClause()
	default:
		return ""
	}
}

// Show gets the data for one EventStream. This function is mapped to
// the path GET /event_streams/{event_stream_id}
func (v EventStreamsResource) Show(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	event := &models.EventStream{}
	if err := tx.Find(event, c.Param("event_stream_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Parse payload for template display
	payload, _ := event.GetPayload()

	// Convert payload to formatted JSON string for display
	var payloadJSON string
	if len(event.Payload) > 0 {
		var prettyJSON map[string]interface{}
		if err := json.Unmarshal(event.Payload, &prettyJSON); err == nil {
			formatted, _ := json.MarshalIndent(prettyJSON, "", "  ")
			payloadJSON = string(formatted)
		} else {
			payloadJSON = string(event.Payload)
		}
	} else {
		payloadJSON = "{}"
	}

	// Per-target delivery rows for the attempts table.
	deliveries := &[]eventStreamDeliveryView{}
	if err := tx.RawQuery(`SELECT st.name AS target_name, d.attempts, d.delivered_at, d.acknowledged_at, d.last_error
		FROM event_deliveries d JOIN sync_targets st ON st.id = d.target_id
		WHERE d.event_id = ? ORDER BY st.name`, event.ID.String()).All(deliveries); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("eventStream", event)
		c.Set("payload", payload)
		c.Set("payloadJSON", payloadJSON)
		c.Set("deliveries", deliveries)
		c.Set("maxAttempts", models.MaxDeliveryAttempts)
		return c.Render(http.StatusOK, r.HTML("event_streams/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(event))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(event))
	}).Respond(c)
}

// ResetAttempts re-queues one event for delivery: every still-pending
// delivery row of the event gets its attempts counter and last error
// cleared, the affected targets' circuit breakers are forgotten and the
// worker is woken for an immediate retry. Mapped to
// POST /event_streams/{event_stream_id}/reset_attempts
func (v EventStreamsResource) ResetAttempts(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	event := &models.EventStream{}
	if err := tx.Find(event, c.Param("event_stream_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	res, err := resetPendingDeliveries(tx, "event_id = ?", event.ID.String())
	if err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", fmt.Sprintf("%d deliveries re-queued", res))
		return c.Redirect(http.StatusSeeOther, "/event_streams/"+event.ID.String())
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(map[string]int{"requeued": res}))
	}).Respond(c)
}

// ResetUndeliverable re-queues every event sitting in the dead-letter queue
// (at least one pending delivery row at the attempts cap). Mapped to
// POST /event_streams/reset_undeliverable
func (v EventStreamsResource) ResetUndeliverable(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	res, err := resetPendingDeliveries(tx, fmt.Sprintf("delivered_at IS NULL AND attempts >= %d", models.MaxDeliveryAttempts))
	if err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", fmt.Sprintf("%d undeliverable deliveries re-queued", res))
		return c.Redirect(http.StatusSeeOther, "/event_streams?delivery=undeliverable")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(map[string]int{"requeued": res}))
	}).Respond(c)
}

// resetPendingDeliveries clears attempts/last_error on the event_deliveries
// rows matching the given WHERE fragment, forgets the circuit breaker of
// every affected target and wakes the webhook worker so the retry happens
// immediately. Returns the number of rows reset.
func resetPendingDeliveries(tx *pop.Connection, where string, args ...interface{}) (int, error) {
	// Collect affected target IDs first so their circuit breakers can be
	// reopened after the reset.
	targetIDs := &[]uuid.UUID{}
	if err := tx.RawQuery("SELECT DISTINCT target_id FROM event_deliveries WHERE "+where, args...).All(targetIDs); err != nil {
		return 0, err
	}

	q := "UPDATE event_deliveries SET attempts = 0, last_error = NULL, updated_at = ? WHERE " + where
	params := append([]interface{}{time.Now()}, args...)
	res, err := tx.RawQuery(q, params...).ExecWithCount()
	if err != nil {
		return 0, err
	}

	if res > 0 {
		for _, id := range *targetIDs {
			webhookPusher.forgetTarget(id)
		}
		EnsureWebhookWorkerRunning()
		signalWebhookWake()
	}
	return res, nil
}

// ClearAll deletes ALL EventStreams from the DB. This function is mapped
// to the path DELETE /event_streams
func (v EventStreamsResource) ClearAll(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	if err := tx.RawQuery("DELETE FROM event_streams").Exec(); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "All events deleted successfully")
		return c.Redirect(http.StatusSeeOther, "/event_streams")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(map[string]string{"status": "ok"}))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(map[string]string{"status": "ok"}))
	}).Respond(c)
}

// Destroy deletes an EventStream from the DB. This function is mapped
// to the path DELETE /event_streams/{event_stream_id}
func (v EventStreamsResource) Destroy(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	event := &models.EventStream{}
	if err := tx.Find(event, c.Param("event_stream_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(event); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Event deleted successfully")
		return c.Redirect(http.StatusSeeOther, "/event_streams")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(event))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(event))
	}).Respond(c)
}
