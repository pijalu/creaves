package actions

import (
	"fmt"
	"net/http"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// TreatmentTimeEntryToggle handles
// POST /treatments/{treatment_id}/entries/{entry_id}/toggle (bugs.md R5-3b,
// U25/D-e): the treatment-page clock-dot switch. It replaces the legacy
// bitmap toggle (PUT /treatmentschedule, removed §8.3) — the done state
// lives on the per-time entry (applied_at, user), never on Timedonebitmap.
//
// done → pending (revert, like the day-plan undo); pending/skipped → done
// (click time + current user). The treatment row itself is untouched.
func TreatmentTimeEntryToggle(c buffalo.Context) error {
	u := GetCurrentUser(c)
	if u == nil {
		return planError(c, http.StatusUnauthorized, fmt.Errorf("authentication required"))
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	treatment := &models.Treatment{}
	if err := tx.Find(treatment, c.Param("treatment_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	entry := &models.TreatmentTimeEntry{}
	if err := tx.Where("treatment_id = ? AND id = ?", treatment.ID, c.Param("entry_id")).First(entry); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if entry.Status == models.TreatmentEntryStatusDone {
		entry.Revert()
	} else {
		entry.MarkDone(time.Now(), u.ID)
	}
	if err := tx.Update(entry); err != nil {
		return err
	}

	return responder.Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, renderJSON(entry))
	}).Wants("html", func(c buffalo.Context) error {
		if back := safeBackParam(c); back != "/" {
			return c.Redirect(http.StatusSeeOther, back)
		}
		return c.Redirect(http.StatusSeeOther, "/treatments/%v", treatment.ID)
	}).Respond(c)
}
