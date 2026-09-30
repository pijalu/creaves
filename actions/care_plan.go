package actions

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// Day plan HTTP layer (§6, §7.2): the caretaker work surface plus the
// fulfillment endpoints (apply/skip/defer, un-apply, batch apply).

// planItemRef locates one occurrence of the day plan (the applications
// UNIQUE key, §4.5).
type planItemRef struct {
	SourceType string    `json:"source_type"`
	SourceID   string    `json:"source_id"`
	AnimalID   int       `json:"animal_id"`
	DueAt      time.Time `json:"due_at"`
}

// planApplyRequest binds POST /care_plan/apply bodies.
type planApplyRequest struct {
	planItemRef
	Status        string `json:"status"` // applied | skipped | deferred
	Note          string `json:"note"`
	DeferredUntil string `json:"deferred_until"` // RFC3339, defer only
	Weight        string `json:"weight"`
	Answer        string `json:"answer"`
	Dosage        string `json:"dosage"` // manual medication dosage (§10-B6)
	// Late (round-2 §6.2-2, §4b-A1): explicit acknowledgment that a
	// PAST-DUE occurrence is being recorded after the fact. Bypasses ONLY
	// the apply-window 409 — future-due, already-recorded and overridden
	// occurrences still 409. Default (flag absent) byte-for-byte unchanged.
	Late bool `json:"late"`
}

// CarePlanIndex handles GET /care_plan (§7.2): the day plan. JSON returns
// the read model; HTML renders the work surface.
func CarePlanIndex(c buffalo.Context) error {
	tx := planTx(c)
	now := time.Now()
	from, to := planWindowParams(c, now)

	plan, err := BuildDayPlan(tx, now, from, to)
	if err != nil {
		return err
	}

	// ?kind=<action_kind> narrows the JSON read model to one kind (§8.3:
	// the retired /feeding page redirects here with kind=feeding). The
	// work screen keeps the FULL plan (CP2 root cause removed) — HTML
	// narrowing is the view model's single zone×kind filter pass.
	kindFilter := ""
	if kind := c.Param("kind"); kind != "" {
		if err := careplan.ValidateActionKind(kind); err != nil {
			return planError(c, http.StatusUnprocessableEntity, err)
		}
		kindFilter = kind
	}

	// Work screen (Phase 2, U2): compact/detailed view + zone filter.
	view := c.Param("view")
	if view != ViewDetailed {
		view = ViewCompact
	}
	zone := c.Param("zone")

	return responder.Wants("html", func(c buffalo.Context) error {
		// CP2: the screen renders the unfiltered plan; the view model
		// narrows it in ONE zone×kind pass (§7.2 stage 3). An unknown
		// zone is whitelist-validated against the zones table — a stale
		// link flashes and resets instead of silently hiding every card.
		if zone != "" {
			exists, zerr := zoneExists(tx, zone)
			if zerr != nil {
				return zerr
			}
			if !exists {
				c.Flash().Add("warning", T.Translate(c, "care_plan.zone.unknown"))
				return c.Redirect(http.StatusFound, planSelfPath(view, "", kindFilter, c.Param("back")))
			}
		}
		c.Set("view", BuildDayPlanView(plan, view, zone, kindFilter, now, c.Param("back")))
		return c.Render(http.StatusOK, r.HTML("/care_plan/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		rows := planJSONRows(narrowPlanByKind(plan, kindFilter))
		return c.Render(http.StatusOK, r.JSON(map[string]interface{}{
			"from": plan.From, "to": plan.To, "items": rows,
		}))
	}).Respond(c)
}

// narrowPlanByKind returns a shallow copy of plan with only the items of
// the given action kind — the JSON read model's backward-compatible
// narrowing (§8.3). An empty kind returns plan unchanged.
func narrowPlanByKind(plan *DayPlan, kind string) *DayPlan {
	if kind == "" {
		return plan
	}
	narrowed := *plan
	narrowed.Items = make([]careplan.PlanItem, 0, len(plan.Items))
	for _, it := range plan.Items {
		if src := it.Occurrence.Source; src != nil && src.ActionKind() == kind {
			narrowed.Items = append(narrowed.Items, it)
		}
	}
	return &narrowed
}

// zoneExists reports whether the zone name exists (work-screen whitelist).
func zoneExists(tx *pop.Connection, name string) (bool, error) {
	n, err := tx.Where("zone = ?", name).Count(&models.Zone{})
	return n > 0, err
}

// planWindowParams parses the ?from=&to= query overrides (§6.1).
func planWindowParams(c buffalo.Context, now time.Time) (time.Time, time.Time) {
	from, to := DefaultPlanWindow(now)
	if raw := c.Param("from"); raw != "" {
		if t, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
			from = t
		}
	}
	if raw := c.Param("to"); raw != "" {
		if t, err := time.ParseInLocation("2006-01-02", raw, now.Location()); err == nil {
			to = t.Add(24*time.Hour - time.Nanosecond)
		}
	}
	return from, to
}

// planItemJSON is the JSON projection of one plan item.
type planItemJSON struct {
	SourceType   string    `json:"source_type"`
	SourceID     string    `json:"source_id"`
	SourceName   string    `json:"source_name"`
	ActionKind   string    `json:"action_kind"`
	AnimalID     int       `json:"animal_id"`
	AnimalLabel  string    `json:"animal_label"`
	Zone         string    `json:"zone"`
	Cage         string    `json:"cage"`
	DueAt        time.Time `json:"due_at"`
	Status       string    `json:"status"`
	Applicable   bool      `json:"applicable"`
	Detail       string    `json:"detail,omitempty"`
	OverriddenBy string    `json:"overridden_by,omitempty"`
	// bugs.md U3: done-tier link to the fulfillment record (additive).
	FulfillmentType string `json:"fulfillment_type,omitempty"`
	FulfillmentID   string `json:"fulfillment_id,omitempty"`
}

func planJSONRows(plan *DayPlan) []planItemJSON {
	items := make([]planItemJSON, 0, len(plan.Items))
	for i := range plan.Items {
		it := &plan.Items[i]
		src := it.Occurrence.Source
		if src == nil {
			continue
		}
		row := planItemJSON{
			SourceType:   string(src.SourceType()),
			SourceID:     src.SourceID(),
			SourceName:   DisplayName(src.Name()),
			ActionKind:   src.ActionKind(),
			AnimalID:     it.Occurrence.AnimalID,
			DueAt:        it.Occurrence.DueAt,
			Status:       string(it.Status),
			Applicable:   it.Applicable,
			Detail:       planDetail(src),
			OverriddenBy: it.OverriddenBy,
		}
		if a, ok := plan.AnimalRow(it.Occurrence.AnimalID); ok {
			row.AnimalLabel = animalLabel(a)
			row.Zone = a.Zone.String
			row.Cage = a.Cage.String
		}
		if app := it.Application; app != nil && !app.FulfillmentDeleted {
			row.FulfillmentType = app.FulfillmentType
			row.FulfillmentID = app.FulfillmentID
		}
		items = append(items, row)
	}
	return items
}

// lateRecordAllowed implements the §4b-A1 late-record bound: the flag
// bypasses ONLY the hors-délai rejection for PAST-DUE occurrences — the
// future stays unrecordable even with the flag (defense-in-depth; genuine
// plan state cannot produce a non-applicable future item).
func lateRecordAllowed(late bool, item *careplan.PlanItem, now time.Time) bool {
	return late && !item.Occurrence.DueAt.After(now)
}

// CarePlanApply handles POST /care_plan/apply (§6.2): applied fulfillment,
// skip (mandatory reason) or defer (mandatory reason + clamped target).
func CarePlanApply(c buffalo.Context) error {
	tx := planTx(c)
	in := &planApplyRequest{}
	if err := c.Bind(in); err != nil {
		return err
	}
	u := GetCurrentUser(c)
	if u == nil {
		return planError(c, http.StatusUnauthorized, fmt.Errorf("authentication required"))
	}

	status := in.Status
	if status == "" {
		status = models.ApplicationStatusApplied
	}
	if status != models.ApplicationStatusApplied &&
		status != models.ApplicationStatusSkipped &&
		status != models.ApplicationStatusDeferred {
		return planError(c, http.StatusUnprocessableEntity, fmt.Errorf("status must be applied, skipped or deferred"))
	}
	if status != models.ApplicationStatusApplied && strings.TrimSpace(in.Note) == "" {
		return planError(c, http.StatusUnprocessableEntity, fmt.Errorf("reason is mandatory for skip and defer (§10-CP4)"))
	}

	now := time.Now()
	rev, err := ReverifyItem(tx, in.planItemRef, now)
	if err != nil {
		if errors.Is(err, errItemNotReproduced) {
			return planError(c, http.StatusConflict, err)
		}
		return err
	}
	item := rev.Item
	if item.Status == careplan.StatusApplied || item.Status == careplan.StatusSkipped || item.Status == careplan.StatusDeferred {
		// §4.5: the occurrence already has a recorded application — the
		// generic hors-délai message would be misleading here.
		return planError(c, http.StatusConflict, fmt.Errorf("occurrence already recorded (idempotent, §4.5)"))
	}
	// Round-2 §6.2-2 (§4b-A1): an explicit `late` acknowledgment lets a
	// caretaker record a missed occurrence after the fact (depupdate
	// parity). Bounded: past-due only (the future stays unrecordable),
	// unapplied (checked above) and inside the plan window (ReverifyItem
	// already 409s anything the window no longer produces). Without the
	// flag the default is unchanged — hors délai 409.
	if !item.Applicable && !lateRecordAllowed(in.Late, item, now) {
		return planError(c, http.StatusConflict, fmt.Errorf("occurrence is out of its apply window (hors délai, §10-A1)"))
	}
	if item.Status == careplan.StatusOverridden {
		return planError(c, http.StatusConflict, fmt.Errorf("occurrence is overridden by animal plan %q", item.OverriddenBy))
	}

	input := PlanApplyInput{
		Status: status,
		Note:   strings.TrimSpace(in.Note),
		Weight: in.Weight,
		Answer: in.Answer,
		Dosage: in.Dosage,
	}
	if status == models.ApplicationStatusDeferred {
		requested := time.Now().Add(time.Hour) // §10-L3 default +1h
		if in.DeferredUntil != "" {
			t, err := time.Parse(time.RFC3339, in.DeferredUntil)
			if err != nil {
				return planError(c, http.StatusUnprocessableEntity, fmt.Errorf("deferred_until must be RFC3339"))
			}
			requested = t
		}
		clamped := ClampDeferredUntil(item.Occurrence.Source, rev.Ctx, item.Occurrence.AnimalID, item.Occurrence.DueAt, requested)
		input.DeferredUntil = &clamped
	}

	// observation alert outcome (§10.1-6): computed from the source payload
	if status == models.ApplicationStatusApplied && item.Occurrence.Source.ActionKind() == careplan.KindObservation {
		payload := parsePlanPayload(item.Occurrence.Source)
		input.AnswerIsAlert = payload.AlertOn != nil && in.Answer == *payload.AlertOn
	}

	app, err := writePlanApplication(tx, item.Occurrence.Source, item.Occurrence.AnimalID, item.Occurrence.DueAt, time.Now(), u.ID, input)
	if err != nil {
		var dre *DosageRequiredError
		if errors.As(err, &dre) {
			// §10-B6: never blocks — the client resubmits with a manual dosage.
			return c.Render(http.StatusUnprocessableEntity, renderJSON(map[string]interface{}{
				"error":          "dosage_required",
				"detail":         dre.Warning.Detail,
				"reason":         dre.Warning.Reason,
				"last_weight":    dre.Warning.LastWeight,
				"last_weight_at": dre.Warning.LastWeightAt,
				"code":           http.StatusUnprocessableEntity,
			}))
		}
		if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "1062") {
			return planError(c, http.StatusConflict, fmt.Errorf("occurrence already recorded (idempotent, §4.5)"))
		}
		return planError(c, http.StatusUnprocessableEntity, err)
	}
	return c.Render(http.StatusCreated, renderJSON(app))
}

// CarePlanUnapply handles POST /care_plan/unapply: correction path.
// Round-2 §4b-A8 (user decision): widened to depupdate parity — any
// authenticated user may undo any record (applied/skipped/deferred, any
// kind). The round-1 admin gate was removed; every undo is audit-logged
// (best-effort, depupdate pattern) with the deleted application row as the
// change record.
func CarePlanUnapply(c buffalo.Context) error {
	u := GetCurrentUser(c)
	if u == nil {
		return planError(c, http.StatusUnauthorized, fmt.Errorf("authentication required"))
	}
	tx := planTx(c)
	in := &struct {
		planItemRef
		DeleteFulfillment bool `json:"delete_fulfillment"`
	}{}
	if err := c.Bind(in); err != nil {
		return err
	}
	// Audit snapshot (§4b-A8): the exact application row being undone —
	// the same row UnapplyPlanItem deletes. A missing ref 404s here, the
	// same answer the endpoint gave admins before the gate removal.
	auditApp := &models.CarePlanApplication{}
	aq := tx.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at = ?",
		in.SourceType, in.SourceID, in.AnimalID, in.DueAt)
	if err := aq.First(auditApp); err != nil {
		return planError(c, http.StatusNotFound, fmt.Errorf("application not found"))
	}
	deleted, err := UnapplyPlanItem(tx, in.SourceType, in.SourceID, in.AnimalID, in.DueAt, in.DeleteFulfillment)
	if err != nil {
		return planError(c, http.StatusNotFound, err)
	}
	auditAnimalChange(c, tx, in.AnimalID, models.AuditEntityCarePlanApplication,
		auditEntityID(auditApp.ID), models.AuditActionDelete, *auditApp, nil)
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"status":              "unapplied",
		"deleted_fulfillment": deleted,
	}))
}

// CarePlanApplyBatch handles POST /care_plan/apply_batch (§6.2): applies a
// list of item keys from a SINGLE (source × cage) group. Each item runs the
// same per-item transaction path — failures (re-verify, UNIQUE) are
// reported per item, never abort the batch.
func CarePlanApplyBatch(c buffalo.Context) error {
	tx := planTx(c)
	u := GetCurrentUser(c)
	if u == nil {
		return planError(c, http.StatusUnauthorized, fmt.Errorf("authentication required"))
	}
	in := &struct {
		Items []planApplyRequest `json:"items"`
	}{}
	if err := c.Bind(in); err != nil {
		return err
	}
	if len(in.Items) == 0 {
		return planError(c, http.StatusUnprocessableEntity, fmt.Errorf("no items"))
	}

	// §10.1-5/N2: the cage is the aggregation ceiling — every item must
	// belong to the same source and the same cage.
	first := in.Items[0]
	cageRef := ""
	for _, ref := range in.Items {
		if ref.SourceID != first.SourceID || ref.SourceType != first.SourceType {
			return planError(c, http.StatusUnprocessableEntity, fmt.Errorf("batch must stay within one source (§10.1-5)"))
		}
	}

	results := make([]map[string]interface{}, 0, len(in.Items))
	applied := 0
	for _, ref := range in.Items {
		res := map[string]interface{}{
			"animal_id": ref.AnimalID,
		}
		rev, rerr := ReverifyItem(tx, ref.planItemRef, time.Now())
		if rerr != nil {
			if errors.Is(rerr, errItemNotReproduced) {
				res["status"] = "already_done"
			} else {
				res["status"] = "error"
				res["error"] = rerr.Error()
			}
			results = append(results, res)
			continue
		}
		item := rev.Item
		cage := rev.Animal.Cage.String
		if cageRef == "" {
			cageRef = cage
		}
		if cage != cageRef {
			res["status"] = "error"
			res["error"] = "batch must stay within one cage (§10.1-5)"
			results = append(results, res)
			continue
		}
		input := PlanApplyInput{
			Status: ref.Status,
			Note:   ref.Note,
			Weight: ref.Weight,
			Answer: ref.Answer,
			Dosage: ref.Dosage,
		}
		if input.Status == "" {
			input.Status = models.ApplicationStatusApplied
		}
		_, err := writePlanApplication(tx, item.Occurrence.Source, item.Occurrence.AnimalID, item.Occurrence.DueAt, time.Now(), u.ID, input)
		if err != nil {
			var dre *DosageRequiredError
			if errors.As(err, &dre) {
				// §10-B6: not an error — the row comes back with a manual dosage.
				res["status"] = "dosage_required"
				res["detail"] = dre.Warning.Detail
				res["reason"] = dre.Warning.Reason
				res["last_weight"] = dre.Warning.LastWeight
				res["last_weight_at"] = dre.Warning.LastWeightAt
			} else if strings.Contains(err.Error(), "Duplicate entry") || strings.Contains(err.Error(), "UNIQUE") || strings.Contains(err.Error(), "1062") {
				res["status"] = "already_done"
			} else {
				res["status"] = "error"
				res["error"] = err.Error()
			}
		} else {
			res["status"] = "applied"
			applied++
		}
		results = append(results, res)
	}
	return c.Render(http.StatusOK, renderJSON(map[string]interface{}{
		"applied": applied,
		"cage":    cageRef,
		"results": results,
	}))
}
