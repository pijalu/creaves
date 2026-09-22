package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// SyncTargetRow pairs one sync target with its delivery counters for the
// synchronization page list.
type SyncTargetRow struct {
	Target *models.SyncTarget
	Counts models.DeliveryCounts
}

// SyncTargetsResource is the resource for the SyncTarget model. Routes are
// registered explicitly in app.go (no List/Show: the list lives on
// /sync_configuration).
type SyncTargetsResource struct {
	buffalo.Resource
}

// parseTargetLimits parses and clamps the per-target delivery limits from
// form parameters into their documented ranges (batch 1-100, rate 1-10000),
// so an invalid value can never reach the delivery worker (SQL LIMIT,
// request size).
func parseTargetLimits(c buffalo.Context) (batchSize, maxPerMin int) {
	batchSize = 1
	maxPerMin = 60
	if bs := c.Param("WebhookBatchSize"); bs != "" {
		fmt.Sscanf(bs, "%d", &batchSize)
	}
	if mpm := c.Param("WebhookMaxPerMin"); mpm != "" {
		fmt.Sscanf(mpm, "%d", &maxPerMin)
	}
	if batchSize < 1 {
		batchSize = 1
	}
	if batchSize > 100 {
		batchSize = 100
	}
	if maxPerMin < 1 {
		maxPerMin = 1
	}
	if maxPerMin > 10000 {
		maxPerMin = 10000
	}
	return batchSize, maxPerMin
}

// bindSyncTarget merges the submitted form values into target. A blank API
// key preserves the stored secret (the form field is write-only for
// non-maintainers), so callers pass the stored row on updates.
func bindSyncTarget(c buffalo.Context, target *models.SyncTarget) {
	target.Name = c.Param("Name")
	target.Enabled = paramIsTrue(c, "Enabled")
	target.WebhookURL = c.Param("WebhookURL")
	if key := c.Param("WebhookAPIKey"); key != "" {
		target.WebhookAPIKey = key
	}
	target.WebhookBatchSize, target.WebhookMaxPerMin = parseTargetLimits(c)
}

// refreshSyncTargetsKnown recomputes the in-memory "at least one enabled
// target" flag after a CRUD change so the delivery worker resumes (or
// stops) hitting the database. Delivery correctness never depends on the
// flag: a stale true only costs one empty query per wake.
func refreshSyncTargetsKnown(tx *pop.Connection) {
	known, err := models.HasEnabledSyncTarget(tx)
	if err != nil {
		return
	}
	SetSyncTargetsKnown(known)
}

// renderTargetForm re-renders the target form with validation errors.
// newRecord selects the "new" vs "edit" template.
func renderTargetForm(c buffalo.Context, target *models.SyncTarget, verrs interface{}, newRecord bool) error {
	c.Set("errors", verrs)
	c.Set("target", target)
	template := "sync_targets/edit.plush.html"
	if newRecord {
		template = "sync_targets/new.plush.html"
	}
	return c.Render(http.StatusUnprocessableEntity, r.HTML(template))
}

// New renders the form for creating a new SyncTarget.
// Mapped to GET /sync_targets/new
func (v SyncTargetsResource) New(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}
	target := &models.SyncTarget{
		Enabled:          true,
		WebhookBatchSize: 1,
		WebhookMaxPerMin: 60,
	}
	c.Set("target", target)
	return c.Render(http.StatusOK, r.HTML("sync_targets/new.plush.html"))
}

// Create adds a SyncTarget to the DB. Mapped to POST /sync_targets
func (v SyncTargetsResource) Create(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	target := &models.SyncTarget{ID: uuid.Must(uuid.NewV4())}
	bindSyncTarget(c, target)

	verrs, err := tx.ValidateAndCreate(target)
	if err != nil {
		return errors.WithStack(err)
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			return renderTargetForm(c, target, verrs, true)
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Respond(c)
	}

	refreshSyncTargetsKnown(tx)
	if target.Deliverable() {
		// A new destination appeared: deliver the backlog without waiting
		// for the fallback tick.
		EnsureWebhookWorkerRunning()
		signalWebhookWake()
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Sync target created successfully")
		return c.Redirect(http.StatusSeeOther, "/sync_configuration")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(target))
	}).Respond(c)
}

// Edit renders the edit form for a SyncTarget.
// Mapped to GET /sync_targets/{sync_target_id}/edit
func (v SyncTargetsResource) Edit(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	target := &models.SyncTarget{}
	if err := tx.Find(target, c.Param("sync_target_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// API-key policy: the form is admin/maintainer-only (requireAdmin) and
	// the stored key must be visible to both roles (bug: admins could not
	// verify the key against the Console-issued one). A blank submit still
	// preserves the stored secret (see bindSyncTarget).
	c.Set("target", target)
	return c.Render(http.StatusOK, r.HTML("sync_targets/edit.plush.html"))
}

// Update changes a SyncTarget in the DB.
// Mapped to PUT /sync_targets/{sync_target_id}
func (v SyncTargetsResource) Update(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	target := &models.SyncTarget{}
	if err := tx.Find(target, c.Param("sync_target_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	bindSyncTarget(c, target)

	verrs, err := tx.ValidateAndUpdate(target)
	if err != nil {
		return errors.WithStack(err)
	}
	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			return renderTargetForm(c, target, verrs, false)
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Respond(c)
	}

	refreshSyncTargetsKnown(tx)
	if target.Deliverable() {
		EnsureWebhookWorkerRunning()
		signalWebhookWake()
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Sync target updated successfully")
		return c.Redirect(http.StatusSeeOther, "/sync_configuration")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(target))
	}).Respond(c)
}

// Destroy deletes a SyncTarget from the DB together with its per-target
// delivery rows (they carry no meaning without the target). Events already
// delivered elsewhere are unaffected; the event_streams rollup columns
// recompute against the remaining enabled targets on the next delivery.
// Mapped to DELETE /sync_targets/{sync_target_id}
func (v SyncTargetsResource) Destroy(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	target := &models.SyncTarget{}
	if err := tx.Find(target, c.Param("sync_target_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.RawQuery("DELETE FROM event_deliveries WHERE target_id = ?", target.ID.String()).Exec(); err != nil {
		return errors.WithStack(err)
	}
	if err := tx.Destroy(target); err != nil {
		return errors.WithStack(err)
	}

	webhookPusher.forgetTarget(target.ID)
	refreshSyncTargetsKnown(tx)

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", "Sync target deleted successfully")
		return c.Redirect(http.StatusSeeOther, "/sync_configuration")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(target))
	}).Respond(c)
}

// RetryUndeliverable resets the attempt counter of every undeliverable
// delivery row of one target so the events re-enter the delivery queue,
// then wakes the worker for an immediate retry. Mapped to
// POST /sync_targets/{sync_target_id}/retry_undeliverable
func (v SyncTargetsResource) RetryUndeliverable(c buffalo.Context) error {
	if _, err := requireAdmin(c); err != nil {
		return err
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	target := &models.SyncTarget{}
	if err := tx.Find(target, c.Param("sync_target_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	res, err := tx.RawQuery(`UPDATE event_deliveries SET attempts = 0, last_error = NULL, updated_at = ?
		WHERE target_id = ? AND delivered_at IS NULL AND attempts >= ?`,
		time.Now(), target.ID.String(), models.MaxDeliveryAttempts).ExecWithCount()
	if err != nil {
		return errors.WithStack(err)
	}

	if res > 0 {
		// The target credentials/route were fixed: reopen its circuit
		// breaker so the next wake actually retries instead of sitting out
		// the remaining open timeout.
		webhookPusher.forgetTarget(target.ID)
		EnsureWebhookWorkerRunning()
		signalWebhookWake()
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", fmt.Sprintf("%d undeliverable events re-queued", res))
		return c.Redirect(http.StatusSeeOther, "/sync_configuration")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(map[string]int{"requeued": res}))
	}).Respond(c)
}
