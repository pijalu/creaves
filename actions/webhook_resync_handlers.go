package actions

import (
	"creaves/models"
	"fmt"
	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"net/http"
)

func WebhookResyncIndex(c buffalo.Context) error {
	if user := GetCurrentUser(c); user == nil || !user.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required"))
	}
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
	if _, err := StartResync(tx, instanceID, 0); err != nil {
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
	if err := tx.Where("status = ?", "running").Order("started_at desc").First(run); err != nil {
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
