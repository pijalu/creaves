package actions

import (
	"creaves/models"
	"fmt"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The expected-set block of the resync page (ComputeSyncStatus) scans every
// animal + every state event of the instance. Running that synchronously
// inside the HTTP request timed out on low-end machines (the whole render
// blocks a pooled MySQL connection for the computation's duration). It now
// runs in the background: the page renders INSTANTLY with the last computed
// snapshot (nil until the first computation finishes — the template hides
// the section then), and a refresh is kicked off at most once per
// syncStatusMaxAge. Reference to the request transaction is never captured:
// it dies with the response, so the refresh uses models.DB.
const syncStatusMaxAge = 30 * time.Second

var (
	syncStatusMu        sync.Mutex
	syncStatusCache     *SyncStatus
	syncStatusCachedAt  time.Time
	syncStatusComputing bool
)

// cachedSyncStatus returns the latest snapshot and starts a background
// recompute when the cache is older than syncStatusMaxAge (single-flight).
func cachedSyncStatus() *SyncStatus {
	syncStatusMu.Lock()
	cached := syncStatusCache
	stale := time.Since(syncStatusCachedAt) > syncStatusMaxAge
	shouldStart := stale && !syncStatusComputing && models.DB != nil
	if shouldStart {
		syncStatusComputing = true
	}
	syncStatusMu.Unlock()
	if shouldStart {
		go func() {
			status, err := ComputeSyncStatus(models.DB, GetInstanceID())
			syncStatusMu.Lock()
			defer syncStatusMu.Unlock()
			if err == nil {
				syncStatusCache = status
				syncStatusCachedAt = time.Now()
			}
			syncStatusComputing = false
		}()
	}
	return cached
}

func WebhookResyncIndex(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
	c.Set("webhookEnabled", IsWebhookEnabled())
	// Expected-set visibility (phase 8): computed in the background — the
	// render itself only serves the cached snapshot, so the page never
	// blocks on the full animals/event_streams scan.
	c.Set("syncStatus", cachedSyncStatus())
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
