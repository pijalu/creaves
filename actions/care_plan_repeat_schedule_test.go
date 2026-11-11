package actions

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// Repeat schedule save/update expansion (BUG-36 decision 3): the four
// create/update handlers (care_animal_plans + care_rules) canonicalize the
// submitted schedule through careplan.NormalizeScheduleJSON — when the
// document carries a repeat block, stored times are re-derived server-side
// and client-supplied times are overwritten. Invalid repeat documents fail
// with 422 and the BUG-33 field-error contract (no raw Go/JSON internals,
// no §/BUG references).

// repeatSchedule builds a fixed-anchor schedule with a repeat block and,
// optionally, tampered times.
func repeatSchedule(t *testing.T, due time.Time, from, to string, every int, times []string) json.RawMessage {
	t.Helper()
	doc := map[string]interface{}{
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
		"repeat": map[string]interface{}{
			"from":          from,
			"to":            to,
			"every_minutes": every,
		},
	}
	if times != nil {
		doc["times"] = times
	}
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	return raw
}

func planVerrsSchedule(t *testing.T, raw []byte) []string {
	t.Helper()
	var body struct {
		Errors map[string][]string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(raw, &body), "body: %s", raw)
	msgs, ok := body.Errors["schedule"]
	require.True(t, ok, "no schedule field error in %s", raw)
	return msgs
}

// assertCleanErrorMessage enforces the BUG-33 contract on one message.
func assertCleanErrorMessage(t *testing.T, msg string) {
	t.Helper()
	require.NotContains(t, msg, "careplan:", "raw internal prefix leaked: %q", msg)
	require.NotContains(t, msg, "§", "opaque spec reference leaked: %q", msg)
	require.NotContains(t, msg, "BUG-", "opaque bug reference leaked: %q", msg)
	require.NotContains(t, msg, "json:", "raw Go/JSON error leaked: %q", msg)
}

func TestCareAnimalPlanRepeatCreateExpandsTimes(t *testing.T) {
	f := setupPlanFixture(t)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	animalID := f.animalIDs[0]
	due := time.Now()

	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	// repeat without times → derived server-side
	sched := repeatSchedule(t, due, "09:00", "19:00", 300, nil)
	code, raw := planDoJSON(t, reg, regURL, "POST", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, map[string]interface{}{
		"name":           "RepA-" + f.marker,
		"action_kind":    "care",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	plan := &models.CareAnimalPlan{}
	require.NoError(t, json.Unmarshal(raw, plan))

	stored := &models.CareAnimalPlan{}
	require.NoError(t, models.DB.Find(stored, plan.ID))
	var doc struct {
		Times  []string `json:"times"`
		Repeat *struct {
			From         string `json:"from"`
			To           string `json:"to"`
			EveryMinutes int    `json:"every_minutes"`
		} `json:"repeat"`
	}
	require.NoError(t, json.Unmarshal(stored.Schedule, &doc))
	require.Equal(t, []string{"09:00", "14:00", "19:00"}, doc.Times, "times re-derived from repeat")
	require.NotNil(t, doc.Repeat, "repeat block preserved")
	require.Equal(t, "09:00", doc.Repeat.From)
	require.Equal(t, "19:00", doc.Repeat.To)
	require.Equal(t, 300, doc.Repeat.EveryMinutes)

	// tampered times are overwritten by the expansion
	sched = repeatSchedule(t, due, "09:00", "19:00", 300, []string{"03:33", "23:59"})
	code, raw = planDoJSON(t, reg, regURL, "PUT", fmt.Sprintf("/animals/%d/care_animal_plans/%s", animalID, plan.ID), regToken, map[string]interface{}{
		"name": "RepA2-" + f.marker, "action_kind": "care",
		"action_payload": json.RawMessage(payload), "schedule": json.RawMessage(sched),
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, models.DB.Find(stored, plan.ID))
	require.NoError(t, json.Unmarshal(stored.Schedule, &doc))
	require.Equal(t, []string{"09:00", "14:00", "19:00"}, doc.Times, "tampered times overwritten (PUT)")
	require.NotNil(t, doc.Repeat, "repeat preserved through PUT round-trip")
}

func TestCareAnimalPlanRepeatInvalidReturns422(t *testing.T) {
	f := setupPlanFixture(t)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	animalID := f.animalIDs[0]
	due := time.Now()

	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	for name, sched := range map[string]json.RawMessage{
		"zero period":   repeatSchedule(t, due, "09:00", "19:00", 0, nil),
		"bad from time": repeatSchedule(t, due, "9:00", "19:00", 300, nil),
	} {
		code, raw := planDoJSON(t, reg, regURL, "POST", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, map[string]interface{}{
			"name":           "RepBad-" + name + "-" + f.marker,
			"action_kind":    "care",
			"action_payload": json.RawMessage(payload),
			"schedule":       json.RawMessage(sched),
		})
		require.Equal(t, http.StatusUnprocessableEntity, code, "%s — body: %s", name, raw)
		for _, msg := range planVerrsSchedule(t, raw) {
			assertCleanErrorMessage(t, msg)
		}
	}

	// malformed schedule document (lone-surrogate escape: bindable as a
	// raw message, unparseable downstream) — same clean 422 contract.
	code, raw := planDoJSON(t, reg, regURL, "POST", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, map[string]interface{}{
		"name":           "RepBad-malformed-" + f.marker,
		"action_kind":    "care",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(`{"times":["\ud800"]}`),
	})
	require.Equal(t, http.StatusUnprocessableEntity, code, "malformed — body: %s", raw)
	for _, msg := range planVerrsSchedule(t, raw) {
		assertCleanErrorMessage(t, msg)
	}
}

func TestCareRuleRepeatCreateUpdateExpandsTimes(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	adminToken := planToken(t, admin, adminURL)
	due := time.Now()

	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	// create with tampered times → overwritten
	sched := repeatSchedule(t, due, "08:00", "20:00", 240, []string{"00:01"})
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_rules", adminToken, map[string]interface{}{
		"name":           "R-Rep-" + f.marker,
		"action_kind":    "feeding",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
		"active":         true,
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	rule := &models.CareRule{}
	require.NoError(t, json.Unmarshal(raw, rule))

	stored := &models.CareRule{}
	require.NoError(t, models.DB.Find(stored, rule.ID))
	var doc struct {
		Times  []string `json:"times"`
		Repeat *struct {
			From         string `json:"from"`
			To           string `json:"to"`
			EveryMinutes int    `json:"every_minutes"`
		} `json:"repeat"`
	}
	require.NoError(t, json.Unmarshal(stored.Schedule, &doc))
	require.Equal(t, []string{"08:00", "12:00", "16:00", "20:00"}, doc.Times, "tampered times overwritten (rule create)")
	require.NotNil(t, doc.Repeat)
	require.Equal(t, 240, doc.Repeat.EveryMinutes)

	// update switching to an overnight repeat window → expansion wraps
	sched = repeatSchedule(t, due, "22:00", "06:00", 240, nil)
	code, raw = planDoJSON(t, admin, adminURL, "PUT", "/care_rules/"+rule.ID.String(), adminToken, map[string]interface{}{
		"name": "R-Rep2-" + f.marker, "active": true,
		"action_kind": "feeding", "action_payload": json.RawMessage(payload),
		"schedule": json.RawMessage(sched),
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, models.DB.Find(stored, rule.ID))
	require.NoError(t, json.Unmarshal(stored.Schedule, &doc))
	require.Equal(t, []string{"02:00", "06:00", "22:00"}, doc.Times, "overnight expansion, sorted ascending")
	require.NotNil(t, doc.Repeat, "repeat preserved through PUT round-trip")
	require.Equal(t, "22:00", doc.Repeat.From)
	require.Equal(t, "06:00", doc.Repeat.To)
}

func TestCareRuleRepeatInvalidReturns422(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	adminToken := planToken(t, admin, adminURL)
	due := time.Now()

	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	sched := repeatSchedule(t, due, "09:00", "19:00", -5, nil)
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_rules", adminToken, map[string]interface{}{
		"name":           "R-RepBad-" + f.marker,
		"action_kind":    "feeding",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
		"active":         true,
	})
	require.Equal(t, http.StatusUnprocessableEntity, code, "body: %s", raw)
	for _, msg := range planVerrsSchedule(t, raw) {
		assertCleanErrorMessage(t, msg)
	}
}

func TestCarePlanSlotsModeUnchangedByNormalization(t *testing.T) {
	f := setupPlanFixture(t)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	animalID := f.animalIDs[0]
	due := itemDueSoon(time.Now())

	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	// slots mode (no repeat block): the submitted times stay exactly as sent
	sched := careScheduleJSON(t, due)
	want := due.Format("15:04")
	code, raw := planDoJSON(t, reg, regURL, "POST", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, map[string]interface{}{
		"name":           "Slots-" + f.marker,
		"action_kind":    "care",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	plan := &models.CareAnimalPlan{}
	require.NoError(t, json.Unmarshal(raw, plan))

	stored := &models.CareAnimalPlan{}
	require.NoError(t, models.DB.Find(stored, plan.ID))
	require.Contains(t, string(stored.Schedule), `"`+want+`"`, "slots times kept")
	require.False(t, strings.Contains(string(stored.Schedule), "repeat"), "no repeat key materialized: %s", stored.Schedule)
}
