package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// planError renders a 4xx JSON error body for the care-plan API with the
// real message — buffalo's default error handler masks err.Error() to
// http.StatusText outside development, which would strip the detail the
// UI toasts and API clients rely on (bugs.md H1).
func planError(c buffalo.Context, status int, err error) error {
	return c.Render(status, renderJSON(map[string]interface{}{
		"error": err.Error(),
		"code":  status,
	}))
}

// Plan service support helpers (§4.5/§6.2): source projection, defer
// clamping (§10-A2/CP4), un-apply (§10-CP1) and the fulfillment-destroy
// hooks (§10-CP1).

// sourceIDUUID parses the engine source id.
func sourceIDUUID(src careplan.PlanSource) uuid.UUID {
	id, _ := uuid.FromString(src.SourceID())
	return id
}

// sourceSnapshot renders the §4.5 source_snapshot audit document (name +
// payload captured at application time, immune to later edits).
func sourceSnapshot(src careplan.PlanSource) []byte {
	if s, ok := src.(*careplan.Source); ok {
		return s.SnapshotJSON()
	}
	b, _ := json.Marshal(struct {
		Name string `json:"name"`
		Kind string `json:"action_kind"`
	}{Name: src.Name(), Kind: src.ActionKind()})
	return b
}

// sourceSnapshotWithDosage annotates the snapshot with
// dosage_source="manual" when the caretaker typed the dosage at apply time
// (§10-B6) — the audit trail must show the number did not come from the
// payload / dosages table.
func sourceSnapshotWithDosage(src careplan.PlanSource, in PlanApplyInput) []byte {
	snap := sourceSnapshot(src)
	if in.Status != models.ApplicationStatusApplied ||
		src.ActionKind() != careplan.KindMedication ||
		strings.TrimSpace(in.Dosage) == "" {
		return snap
	}
	var doc map[string]interface{}
	if err := json.Unmarshal(snap, &doc); err != nil {
		return snap
	}
	doc["dosage_source"] = "manual"
	doc["manual_dosage"] = strings.TrimSpace(in.Dosage)
	b, err := json.Marshal(doc)
	if err != nil {
		return snap
	}
	return b
}

// jsonUnmarshalStrictish decodes a stored payload document leniently
// (unknown keys tolerated — the authoritative per-kind validation runs at
// save time, §4.2).
func jsonUnmarshalStrictish(raw []byte, v interface{}) error {
	return json.Unmarshal(raw, v)
}

// jsonMarshal encodes one JSON document.
func jsonMarshal(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

// fmtSscan scans one float from s.
func fmtSscan(s string, f *float64) (int, error) {
	return fmt.Sscanf(strings.TrimSpace(s), "%g", f)
}

// ---------------------------------------------------------------------------
// Defer clamp (§10-A2/CP4)
// ---------------------------------------------------------------------------

// ClampDeferredUntil clamps the requested snooze target to just before the
// next occurrence of the same source (§10-A2/CP4): a resurfaced item must
// still be applicable. Floor: due + 1 minute. When the source has no next
// occurrence the request passes through.
func ClampDeferredUntil(src careplan.PlanSource, ctx *careplan.AnimalContext, animalID int, dueAt, requested time.Time) time.Time {
	floor := dueAt.Add(time.Minute)
	if requested.Before(floor) {
		return floor
	}
	if src == nil || ctx == nil {
		return requested
	}
	windowEnd := dueAt.Add(30 * 24 * time.Hour)
	next := careplan.GenerateOccurrences(src, ctx, dueAt.Add(time.Second), windowEnd)
	for _, o := range next {
		if o.DueAt.After(dueAt) {
			limit := o.DueAt.Add(-time.Minute)
			if requested.After(limit) {
				return limit
			}
			return requested
		}
	}
	return requested
}

// ---------------------------------------------------------------------------
// Un-apply (§10-CP1 — admin-only at the handler)
// ---------------------------------------------------------------------------

// UnapplyPlanItem deletes the application row of one occurrence and —
// optionally — the linked fulfillment record (checkbox, default keep).
// Returns the destroyed fulfillment type ("" when kept).
func UnapplyPlanItem(tx *pop.Connection, sourceType, sourceID string, animalID int, dueAt time.Time, deleteFulfillment bool) (string, error) {
	app := &models.CarePlanApplication{}
	q := tx.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at = ?",
		sourceType, sourceID, animalID, dueAt)
	if err := q.First(app); err != nil {
		return "", fmt.Errorf("application not found: %w", err)
	}

	deleted := ""
	if deleteFulfillment && app.Status == models.ApplicationStatusApplied && app.FulfillmentID != planFulfillmentNone {
		switch app.FulfillmentType {
		case models.ApplicationFulfillmentCare:
			if err := tx.Destroy(&models.Care{ID: uuid.FromStringOrNil(app.FulfillmentID)}); err != nil {
				return "", err
			}
			deleted = app.FulfillmentType
		case models.ApplicationFulfillmentTreatment:
			d, derr := unapplyTreatmentFulfillment(tx, app)
			if derr != nil {
				return "", derr
			}
			deleted = d
		}
	}
	if err := tx.Destroy(app); err != nil {
		return "", err
	}
	return deleted, nil
}

// unapplyTreatmentFulfillment releases one application's share of a
// treatments row (bugs.md R5-3b, U25/D-e — per-time entries):
//
//   - sibling applications on the same row (§10-M1 same-slot collisions):
//     only THIS application's entry is removed — the row stays for them.
//   - apply-created row (exactly this application's entry): the row is
//     destroyed with its entries (pre-R5-3 fulfillment destroy parity).
//   - otherwise: the entry reverts to pending — the row keeps its sibling
//     entries (the other expected times of the day, E2E-3 step 3).
//   - legacy/migrated rows without an application-linked entry: the row is
//     destroyed (pre-R5-3 behavior; entries cascade).
func unapplyTreatmentFulfillment(tx *pop.Connection, app *models.CarePlanApplication) (string, error) {
	// Same-bucket collisions (§10-M1) share ONE treatments row between
	// several applications — the row must survive while a sibling exists.
	var siblings []models.CarePlanApplication
	if err := tx.Where("fulfillment_type = ? AND fulfillment_id = ? AND id != ?",
		app.FulfillmentType, app.FulfillmentID, app.ID).All(&siblings); err != nil {
		return "", err
	}
	treatmentID := uuid.FromStringOrNil(app.FulfillmentID)
	entries, err := loadTreatmentEntries(tx, treatmentID)
	if err != nil {
		return "", err
	}
	own := entries.FindByApplication(app.ID)

	if len(siblings) > 0 {
		return "", destroyOwnEntry(tx, own)
	}
	if own == nil || len(entries) == 1 {
		// own == nil: legacy/migrated row without an application-linked
		// entry — pre-R5-3 behavior, destroy the row (entries cascade).
		// one entry: apply-created row — destroy the row with its entry.
		if err := tx.Destroy(&models.Treatment{ID: treatmentID}); err != nil {
			return "", err
		}
		return app.FulfillmentType, nil
	}
	return "", revertOwnEntry(tx, own)
}

// destroyOwnEntry drops only this application's entry — the sibling's
// record must stay intact.
func destroyOwnEntry(tx *pop.Connection, own *models.TreatmentTimeEntry) error {
	if own == nil {
		return nil
	}
	return tx.Destroy(own)
}

// revertOwnEntry flips the application's entry back to pending; the row
// keeps its sibling entries (the other expected times of the day).
func revertOwnEntry(tx *pop.Connection, own *models.TreatmentTimeEntry) error {
	own.Revert()
	return tx.Update(own)
}

// ---------------------------------------------------------------------------
// Fulfillment destroy hooks (§10-CP1)
// ---------------------------------------------------------------------------

// MarkPlanApplicationFulfillmentDeleted flags the applications whose linked
// care/treatment row was just destroyed: the plan keeps the audit trail and
// renders "record deleted" instead of a dangling link.
func MarkPlanApplicationFulfillmentDeleted(tx *pop.Connection, fulfillmentType, fulfillmentID string) error {
	if fulfillmentID == "" || fulfillmentID == planFulfillmentNone {
		return nil
	}
	return tx.RawQuery(
		"UPDATE care_plan_applications SET fulfillment_deleted = ? WHERE fulfillment_type = ? AND fulfillment_id = ?",
		true, fulfillmentType, fulfillmentID).Exec()
}
