package actions

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Review finding #2: the batch endpoint must enforce the SAME per-item
// guardrails as the single apply endpoint (§6.2) — status whitelist,
// mandatory skip/defer reason (§10-CP4), overridden (§4.7) and hors-délai
// (§10-A1) 409s, defer target parse + clamp (§10-CP4/L3) and the
// observation alert outcome (§10.1-6).
func TestCarePlanBatchGuardrails(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	f.feedRule(t, models.DB, due)
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	i1 := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "feeding")
	ref := itemRef(i1)

	type batchOut struct {
		Applied int                      `json:"applied"`
		Results []map[string]interface{} `json:"results"`
	}
	post := func(items ...map[string]interface{}) batchOut {
		code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{"items": items})
		require.Equal(t, http.StatusOK, code, "body: %s", raw)
		var out batchOut
		require.NoError(t, json.Unmarshal(raw, &out))
		return out
	}

	// invalid status → per-item error, nothing written
	out := post(map[string]interface{}{
		"source_type": ref["source_type"], "source_id": ref["source_id"],
		"animal_id": ref["animal_id"], "due_at": ref["due_at"],
		"status": "foo",
	})
	require.Equal(t, 0, out.Applied)
	require.Contains(t, out.Results[0]["error"], "status must be applied, skipped or deferred")

	// skip without reason → per-item error (§10-CP4)
	out = post(map[string]interface{}{
		"source_type": ref["source_type"], "source_id": ref["source_id"],
		"animal_id": ref["animal_id"], "due_at": ref["due_at"],
		"status": "skipped",
	})
	require.Equal(t, 0, out.Applied)
	require.Contains(t, out.Results[0]["error"], "reason is mandatory")

	// stale occurrence: yesterday 00:05 — its NEXT occurrence (today
	// 00:05) is already due → outside the apply window (§10-A1 hors
	// délai). Only the explicit late acknowledgment records it (§4b-A1).
	yst := time.Now().AddDate(0, 0, -1)
	dueY := time.Date(yst.Year(), yst.Month(), yst.Day(), 0, 5, 0, 0, time.Local)
	// fixture animals are intaked today — occurrences before the intake
	// instant are clamped away (§10-B3). Backdate animals.IntakeDate (the
	// planning anchor read by loadAnimalContexts) so yesterday's
	// occurrence exists for them.
	require.NoError(t, models.DB.RawQuery("UPDATE animals SET IntakeDate = ? WHERE id IN (?)",
		yst.AddDate(0, 0, -1), f.animalIDs).Exec())
	staleRule := f.feedRule(t, models.DB, dueY)
	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	var staleItem map[string]interface{}
	for _, it := range planItemsOf(t, body) {
		if it["source_id"] != staleRule.ID.String() || int(it["animal_id"].(float64)) != f.animalIDs[0] {
			continue
		}
		du, perr := time.Parse(time.RFC3339, it["due_at"].(string))
		require.NoError(t, perr)
		if du.Equal(dueY) { // yesterday's slot, not today's
			staleItem = it
			break
		}
	}
	require.NotNil(t, staleItem, "stale occurrence is planned")
	fref := itemRef(staleItem)
	out = post(map[string]interface{}{
		"source_type": fref["source_type"], "source_id": fref["source_id"],
		"animal_id": fref["animal_id"], "due_at": fref["due_at"],
	})
	require.Equal(t, 0, out.Applied, "stale occurrence must not be recordable")
	require.Contains(t, out.Results[0]["error"], "hors délai")

	// late=true bypasses ONLY the hors-délai bound for this past-due item
	out = post(map[string]interface{}{
		"source_type": fref["source_type"], "source_id": fref["source_id"],
		"animal_id": fref["animal_id"], "due_at": fref["due_at"],
		"late": true,
	})
	require.Equal(t, 1, out.Applied, "late acknowledgment records the past-due occurrence (§4b-A1)")

	// defer via batch: mandatory reason, RFC3339 target, CLAMPED row —
	// the old code stored a deferred row with a nil deferred_until (the
	// re-apply deadlock, review finding #2).
	out = post(map[string]interface{}{
		"source_type": ref["source_type"], "source_id": ref["source_id"],
		"animal_id": ref["animal_id"], "due_at": ref["due_at"],
		"status": "deferred", "note": "soin plus tard",
		"deferred_until": "not-a-time",
	})
	require.Equal(t, 0, out.Applied)
	require.Contains(t, out.Results[0]["error"], "deferred_until must be RFC3339")

	before := time.Now()
	out = post(map[string]interface{}{
		"source_type": ref["source_type"], "source_id": ref["source_id"],
		"animal_id": ref["animal_id"], "due_at": ref["due_at"],
		"status": "deferred", "note": "soin plus tard",
	})
	require.Equal(t, 1, out.Applied)

	row := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at = ?",
		ref["source_type"], ref["source_id"], f.animalIDs[0], due).First(row))
	require.Equal(t, models.ApplicationStatusDeferred, row.Status)
	require.False(t, row.DeferredUntil == nil || row.DeferredUntil.IsZero(), "batch defer must store a clamped target (no nil-target deadlock)")
	require.WithinDuration(t, before.Add(time.Hour), *row.DeferredUntil, 5*time.Minute, "§10-L3 default +1h")

	// re-applying the deferred occurrence via batch → already_done (§4.5),
	// NOT a UNIQUE error and NOT a second row
	out = post(map[string]interface{}{
		"source_type": ref["source_type"], "source_id": ref["source_id"],
		"animal_id": ref["animal_id"], "due_at": ref["due_at"],
	})
	require.Equal(t, 0, out.Applied)
	require.Equal(t, "already_done", out.Results[0]["status"])
}

// Review finding #2 (alert outcome): a batch-recorded observation with the
// alert_on answer must run the §10-CP3/M3 alert loop exactly like the
// single endpoint.
func TestCarePlanBatchObservationAlertLoop(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{
		"prompt":                "CP-batch-ctrl " + f.marker,
		"alert_on":              "no",
		"alert_follow_up_hours": 1,
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "ObsBatch-" + f.marker,
		ActionKind: "observation", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "observation")
	ref := itemRef(item)
	ref["answer"] = "no" // == alert_on → alert outcome

	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{
		"items": []map[string]interface{}{ref},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var out struct {
		Applied int `json:"applied"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, 1, out.Applied)

	var warnCount int
	require.NoError(t, models.DB.RawQuery(
		"SELECT COUNT(*) AS c FROM cares WHERE animal_id = ? AND type_id IN (SELECT id FROM caretypes WHERE warning = 1)", f.animalIDs[0]).First(&warnCount))
	require.Equal(t, 1, warnCount, "batch alert answer writes the Warning care (§10.1-6)")

	followUp := &models.CareAnimalPlan{}
	require.NoError(t, models.DB.Where("animal_id = ? AND active = ? AND name LIKE ?",
		f.animalIDs[0], true, "Vérifier alerte:%").First(followUp), "follow-up plan auto-created (§10-CP3)")
}

// Review finding #6: an overridden occurrence is reported as overridden
// (the structural cause), never as hors délai — overridden items are
// ALWAYS outside the apply window (itemApplicable, §10-A1), so the old
// check order made the wrong message unavoidable.
func TestOverriddenMessagePrecedence(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	f.feedRule(t, models.DB, due)

	// kind-level replacing plan on animal 1 → its rule occurrences render
	// overridden (§4.7)
	planPayload, err := json.Marshal(map[string]interface{}{
		"caretype_id": f.feedCare.String(),
		"food":        "vers",
	})
	require.NoError(t, err)
	planSched, err := json.Marshal(map[string]interface{}{
		"times":       []string{due.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
	})
	require.NoError(t, err)
	ap := &models.CareAnimalPlan{
		ID:            uuid.Must(uuid.NewV4()),
		AnimalID:      f.animalIDs[0],
		Name:          "P-" + f.marker,
		ActionKind:    "feeding",
		ActionPayload: planPayload,
		Schedule:      planSched,
		ReplacesKind:  true,
		Active:        true,
	}
	require.NoError(t, models.DB.Create(ap))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "feeding")
	require.Equal(t, "overridden", item["status"], "fixture produces an overridden item")
	ref := itemRef(item)

	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, ref)
	require.Equal(t, http.StatusConflict, code, "body: %s", raw)
	require.Contains(t, string(raw), "overridden by animal plan")
	require.NotContains(t, string(raw), "hors délai", "overridden must win the message precedence")

	// batch agrees (same shared pre-flight)
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{
		"items": []map[string]interface{}{ref},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.Contains(t, string(raw), "overridden by animal plan")
	require.NotContains(t, string(raw), "hors délai")
}
