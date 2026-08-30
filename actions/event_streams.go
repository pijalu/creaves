package actions

import (
	"creaves/models"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/pkg/errors"
)

// EventStreamsResource is the resource for the EventStream model
type EventStreamsResource struct {
	buffalo.Resource
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

	events := &models.EventStreams{}
	q := tx.PaginateFromParams(c.Params())

	if err := q.Order("created_at desc").All(events); err != nil {
		return errors.WithStack(err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("pagination", q.Paginator)
		c.Set("eventStreams", events)
		return c.Render(http.StatusOK, r.HTML("event_streams/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(events))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(events))
	}).Respond(c)
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

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("eventStream", event)
		c.Set("payload", payload)
		c.Set("payloadJSON", payloadJSON)
		return c.Render(http.StatusOK, r.HTML("event_streams/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(event))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(event))
	}).Respond(c)
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
