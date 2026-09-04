package actions

import (
	"creaves/models"
	"fmt"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"log"
	"net/http"
	"strings"
)

func WebhookResyncIndex(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
	c.Set("webhookEnabled", IsWebhookEnabled())
	// Expected-set visibility (phase 8): per-animal hashes are recomputed
	// live, so keep this on the page render only — never in the polled
	// status.json endpoint.
	var syncStatus *SyncStatus
	if tx, ok := c.Value("tx").(*pop.Connection); ok {
		var err error
		syncStatus, err = ComputeSyncStatus(tx, GetInstanceID())
		if err != nil {
			log.Printf("failed to compute sync status: %v", err)
			syncStatus = nil
		}
	}
	c.Set("syncStatus", syncStatus)
	return c.Render(http.StatusOK, r.HTML("webhook_resync/index.plush.html"))
}
func WebhookResyncStart(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	instanceID := GetInstanceID()
	force := strings.TrimSpace(c.Request().FormValue("force")) != ""
	enableConfirmed := strings.TrimSpace(c.Request().FormValue("enable_webhook")) != ""
	if _, err := StartResync(tx, instanceID, 0, force); err != nil {
		if enableConfirmed && (err == ErrWebhookDisabled || strings.Contains(err.Error(), "webhook forwarding is disabled")) {
			if enableErr := EnableWebhookForwarding(tx); enableErr != nil {
				c.Flash().Add("danger", "Webhook forwarding is disabled and could not be enabled: "+enableErr.Error())
				c.Set("webhookEnabled", false)
				return c.Render(http.StatusConflict, r.HTML("webhook_resync/index.plush.html"))
			}
			if _, retryErr := StartResync(tx, instanceID, 0, force); retryErr != nil {
				c.Flash().Add("danger", retryErr.Error())
				c.Set("webhookEnabled", IsWebhookEnabled())
				return c.Render(http.StatusConflict, r.HTML("webhook_resync/index.plush.html"))
			}
			c.Flash().Add("success", "Webhook forwarding enabled — resync started.")
			return c.Redirect(http.StatusSeeOther, "/webhook_resync")
		}
		if err == ErrWebhookDisabled || strings.Contains(err.Error(), "webhook forwarding is disabled") {
			c.Flash().Add("danger", "Webhook forwarding is disabled — resync cannot run until it is enabled.")
			c.Set("webhookEnabled", false)
			c.Set("pendingForce", force)
			return c.Render(http.StatusConflict, r.HTML("webhook_resync/index.plush.html"))
		}
		return c.Error(http.StatusConflict, err)
	}
	return c.Redirect(http.StatusSeeOther, "/webhook_resync")
}
func WebhookResyncStatus(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	run := &models.ResyncRun{}
	// Latest run regardless of status: a FAILED run must stay visible on the
	// resync page after the worker exits (bug #1 — failures used to vanish
	// because only 'running' rows were reported).
	if err := tx.Order("created_at desc, started_at desc").First(run); err != nil {
		return c.Render(http.StatusOK, r.JSON(map[string]string{"status": "none"}))
	}
	return c.Render(http.StatusOK, r.JSON(run))
}

func WebhookResyncCancel(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}
	run := &models.ResyncRun{}
	if err := tx.Where("status = ?", "running").Order("started_at desc").First(run); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if err := CancelResync(tx, run.ID); err != nil {
		return c.Error(http.StatusInternalServerError, err)
	}
	return c.Redirect(http.StatusSeeOther, "/webhook_resync")
}
