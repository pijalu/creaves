package actions

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Round-2 engine affordances HTTP tests (docs/care-plan-ux-fix-plan-round2.md
// §6.3/§6.4): the A1 late-record bounds, the A8 undo widening (any
// authenticated user, audited) and the A2 today-outtaken assembly scope.

// planItemDueAt finds a day-plan item by exact due time + kind (optionally
// restricted to one animal), returning its occurrence ref. The animalID
// filter keeps assertions deterministic: rules without a matcher match
// every in-care animal in the shared test DB.
func planItemDueAt(t *testing.T, body map[string]interface{}, kind string, due time.Time, animalID int) map[string]interface{} {
	t.Helper()
	for _, it := range planItemsOf(t, body) {
		if it["action_kind"] != kind {
			continue
		}
		if animalID != 0 && int(it["animal_id"].(float64)) != animalID {
			continue
		}
		d, err := time.Parse(time.RFC3339, it["due_at"].(string))
		require.NoError(t, err)
		if d.Equal(due) {
			return it
		}
	}
	return nil
}

// mkPlanAnimal creates a fixture animal with an intake `age` in the past.
// Occurrence generation clamps to [intake, outtake): yesterday slots need
// an intake before them, the fixture default (now−24h) is not enough.
func mkPlanAnimal(t *testing.T, f *planFixture, tx *pop.Connection, species string, age time.Duration) int {
	t.Helper()
	aid := f.mkAnimal(t, tx, species)
	a := &models.Animal{}
	require.NoError(t, tx.Find(a, aid))
	a.IntakeDate = time.Now().Add(-age)
	require.NoError(t, tx.Update(a))
	return aid
}

func planApplyBody(it map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	b := map[string]interface{}{
		"source_type": it["source_type"],
		"source_id":   it["source_id"],
		"animal_id":   it["animal_id"],
		"due_at":      it["due_at"],
	}
	for k, v := range extra {
		b[k] = v
	}
	return b
}

// §4b-A1: the late-record bounds — accepted on a past-due, unapplied,
// in-window occurrence; rejected on the future and on already-recorded
// occurrences; the flagless default still 409s (byte-for-byte).
func TestCarePlanLateRecordBounds(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()
	y := now.AddDate(0, 0, -1)
	yDay := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, y.Location())
	y0800 := yDay.Add(8 * time.Hour)
	y1900 := yDay.Add(19 * time.Hour)

	f.feedRule(t, tx, yDay, y0800, y1900)

	// A dedicated animal with an intake before yesterday: occurrence
	// generation clamps to [intake, outtake), so yesterday's slots need an
	// older intake than the fixture default (now−24h).
	aid := mkPlanAnimal(t, f, tx, "CP-LATE", 72*time.Hour)

	// A non-admin performs the late records — §4b-A1 is for every user.
	client, baseURL := planRegularClient(t)
	token := planToken(t, client, baseURL)

	q := fmt.Sprintf("/care_plan?from=%s&to=%s", yDay.Format("2006-01-02"), yDay.Format("2006-01-02"))
	_, body := planGetJSON(t, client, baseURL, q)

	// yesterday 08:00: successor 19:00 already due → out of the apply
	// window, past-due, unapplied → late record only.
	it0800 := planItemDueAt(t, body, "feeding", y0800, aid)
	require.NotNil(t, it0800, "yesterday 08:00 occurrence must be in the window")
	require.Equal(t, false, it0800["applicable"])

	// no flag → 409 exactly as before the round-2 change.
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, planApplyBody(it0800, nil))
	require.Equal(t, http.StatusConflict, code, "flagless default unchanged: %s", raw)

	// late flag → 201, normal application row.
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token,
		planApplyBody(it0800, map[string]interface{}{"late": true}))
	require.Equal(t, http.StatusCreated, code, "late record accepted: %s", raw)

	// already recorded → 409 (idempotent, late flag or not).
	code, _ = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token,
		planApplyBody(it0800, map[string]interface{}{"late": true}))
	require.Equal(t, http.StatusConflict, code)

	// yesterday 19:00: late accepted too (skip keeps its mandatory note).
	it1900 := planItemDueAt(t, body, "feeding", y1900, aid)
	require.NotNil(t, it1900)
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token,
		planApplyBody(it1900, map[string]interface{}{"status": "skipped", "note": "owner fed at home", "late": true}))
	require.Equal(t, http.StatusCreated, code, "late skip accepted: %s", raw)

	// the FUTURE stays unrecordable even with the flag: an occurrence the
	// plan window does not produce is unreachable — ReverifyItem answers
	// 409 before any late bound matters. (In-window future work stays
	// pre-recordable by design, §10-A1 apply window.)
	day3 := yDay.AddDate(0, 0, 4)
	r3 := f.feedRule(t, tx, day3, day3.Add(8*time.Hour))
	crafted := planApplyBody(it0800, map[string]interface{}{"late": true})
	crafted["source_id"] = r3.ID.String()
	crafted["due_at"] = day3.Add(8 * time.Hour).Format(time.RFC3339)
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, crafted)
	require.Equal(t, http.StatusConflict, code, "beyond-window future never recordable: %s", raw)
}

// §4b-A8: any authenticated user may undo any record; the undo is
// audit-logged; a missing ref 404s (no 403 branch remains).
func TestUnapplyAnyRecordByAnyUser(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()
	future := now.Add(30 * time.Minute).Truncate(time.Minute)

	f.feedRule(t, tx, future, future)

	// own animal → deterministic ref (empty matcher matches the whole
	// shared test DB); a future slot needs no special intake.
	aid := mkPlanAnimal(t, f, tx, "CP-UNDO", 72*time.Hour)

	admin, aURL := planAdminClient(t)
	adminTok := planToken(t, admin, aURL)
	reg, rURL := planRegularClient(t)
	regTok := planToken(t, reg, rURL)

	_, body := planGetJSON(t, admin, aURL, "/care_plan")
	it := planItemDueAt(t, body, "feeding", future, aid)
	require.NotNil(t, it)
	animalID := int(it["animal_id"].(float64))

	// admin records a SKIP (round-1: only admins could ever undo it)…
	code, raw := planDoJSON(t, admin, aURL, "POST", "/care_plan/apply", adminTok,
		planApplyBody(it, map[string]interface{}{"status": "skipped", "note": "round-2 undo test"}))
	require.Equal(t, http.StatusCreated, code, "%s", raw)

	// …and the non-admin undoes it — 200 where round-1 answered 403.
	// Body = the flat occurrence ref (planItemRef fields) + no fulfillment
	// delete (the feeding care row stays, default keep).
	undo := planApplyBody(it, nil)
	undo["delete_fulfillment"] = false
	code, raw = planDoJSON(t, reg, rURL, "POST", "/care_plan/unapply", regTok, undo)
	require.Equal(t, http.StatusOK, code, "A8: any user undoes any record: %s", raw)

	cnt, err := tx.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at = ?",
		it["source_type"], it["source_id"], animalID, future).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "application row removed")

	// the undo is audit-logged (depupdate parity).
	audits, err := tx.Where("animal_id = ? AND entity = ? AND action = ?",
		animalID, models.AuditEntityCarePlanApplication, models.AuditActionDelete).Count(&models.AnimalAudit{})
	require.NoError(t, err)
	require.GreaterOrEqual(t, audits, 1, "undo leaves an audit entry")

	// a missing ref 404s — the admin answer before the gate removal.
	missing := planApplyBody(it, nil)
	missing["source_id"] = uuid.Must(uuid.NewV4()).String()
	missing["due_at"] = future.Format(time.RFC3339)
	_, raw = planDoJSON(t, reg, rURL, "POST", "/care_plan/unapply", regTok, missing)
	require.Contains(t, string(raw), "not found")
}

// §4b-A2 (bugs.md Dash-9): animals outtaken TODAY stay in the day-plan
// assemblies and their past occurrences are recordable via the late path;
// animals outtaken before today never enter.
func TestOuttakenTodayInAssemblies(t *testing.T) {
	f := setupPlanFixture(t)
	tx := models.DB
	now := time.Now()
	y := now.AddDate(0, 0, -1)
	yDay := time.Date(y.Year(), y.Month(), y.Day(), 0, 0, 0, 0, y.Location())
	y0800 := yDay.Add(8 * time.Hour)
	y1900 := yDay.Add(19 * time.Hour)

	f.feedRule(t, tx, yDay, y0800, y1900)

	ot := &models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "CPOut-" + f.marker}
	require.NoError(t, tx.Create(ot))
	t.Cleanup(func() { models.DB.RawQuery("DELETE FROM outtaketypes WHERE id = ?", ot.ID).Exec() })

	mkOuttaken := func(species string, date time.Time) int {
		aid := f.mkAnimal(t, tx, species)
		// intake before yesterday's slots: occurrence generation clamps
		// to [intake, outtake), the fixture default (now−24h) is too late.
		a := &models.Animal{}
		require.NoError(t, tx.Find(a, aid))
		a.IntakeDate = now.Add(-72 * time.Hour)
		out := &models.Outtake{ID: uuid.Must(uuid.NewV4()), Date: date, TypeID: ot.ID}
		require.NoError(t, tx.Create(out))
		aidLocal := aid
		outID := out.ID
		t.Cleanup(func() {
			// FK-safe: clear animals.outtake_id BEFORE dropping the outtake
			// row (the fixture cage cleanup then deletes the animals).
			models.DB.RawQuery("UPDATE animals SET outtake_id = NULL WHERE id = ?", aidLocal).Exec()
			models.DB.RawQuery("DELETE FROM outtakes WHERE id = ?", outID).Exec()
		})
		a.OuttakeID = nulls.NewUUID(out.ID)
		require.NoError(t, tx.Update(a))
		return aid
	}

	todayOut := mkOuttaken("CP-OT-today", now)
	oldOut := mkOuttaken("CP-OT-old", yDay.Add(-24*time.Hour))

	plan, err := BuildDayPlan(tx, now, time.Time{}, time.Time{})
	require.NoError(t, err)

	// today-outtaken: in the assembly, with the Outtake row preloaded…
	a, ok := plan.AnimalRow(todayOut)
	require.True(t, ok, "today-outtaken animal stays in the assemblies (Dash-9)")
	require.NotNil(t, a.Outtake)

	// …before-today: never enters.
	_, ok = plan.AnimalRow(oldOut)
	require.False(t, ok, "older outtakes never enter the assemblies")

	// its past occurrence is late-recordable (ReverifyItem reproduces it).
	client, baseURL := planRegularClient(t)
	token := planToken(t, client, baseURL)
	q := fmt.Sprintf("/care_plan?from=%s&to=%s", yDay.Format("2006-01-02"), yDay.Format("2006-01-02"))
	_, body := planGetJSON(t, client, baseURL, q)
	it := planItemDueAt(t, body, "feeding", y0800, todayOut)
	require.NotNil(t, it)
	require.Equal(t, float64(todayOut), it["animal_id"].(float64))
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token,
		planApplyBody(it, map[string]interface{}{"late": true}))
	require.Equal(t, http.StatusCreated, code, "outtaken-today late record: %s", raw)

	var app models.CarePlanApplication
	err = tx.Where("source_type = ? AND source_id = ? AND animal_id = ? AND due_at = ?",
		it["source_type"], it["source_id"], todayOut, y0800).First(&app)
	require.NoError(t, err)
	require.Equal(t, models.ApplicationStatusApplied, app.Status)
}
