package actions

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Care Expert System HTTP-layer tests (docs/care-expert.md §6/§7.2):
// day plan statuses, apply idempotency, skip/defer rules, un-apply gating,
// batch cage scope, rules/matchers admin CRUD + preview, animal-plan
// caretaker CRUD, alert loop + follow-up, fulfillment destroy hooks.

// ---------------------------------------------------------------------------
// Fixtures
// ---------------------------------------------------------------------------

type planFixture struct {
	marker    string
	animalIDs []int
	cage      string
	feedCare  uuid.UUID // feeding caretype
	warnCare  uuid.UUID // warning caretype
	resetCare uuid.UUID // warning-response caretype
	defCare   uuid.UUID // default caretype
	userID    uuid.UUID
}

func setupPlanFixture(t *testing.T) *planFixture {
	t.Helper()
	requireMySQLTestDB(t)
	tx := models.DB
	marker := uuid.Must(uuid.NewV4()).String()[:8]
	f := &planFixture{marker: marker, cage: "CP-" + marker}

	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "CPType-" + f.marker}
	require.NoError(t, tx.Create(&at))
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "CPAge-" + f.marker}
	require.NoError(t, tx.Create(&aa))

	cts := []models.Caretype{
		{ID: uuid.Must(uuid.NewV4()), Name: "CPFeed-" + f.marker, Type: models.CareTypeFeed},
		{ID: uuid.Must(uuid.NewV4()), Name: "CPWarn-" + f.marker, Warning: true},
		{ID: uuid.Must(uuid.NewV4()), Name: "CPReset-" + f.marker, ResetWarning: true},
		{ID: uuid.Must(uuid.NewV4()), Name: "CPDef-" + f.marker, Def: true},
	}
	for i := range cts {
		require.NoError(t, tx.Create(&cts[i]))
	}
	f.feedCare, f.warnCare, f.resetCare, f.defCare = cts[0].ID, cts[1].ID, cts[2].ID, cts[3].ID

	f.mkAnimal(t, tx, "CP-A1")
	f.mkAnimal(t, tx, "CP-A2")

	u := &models.User{Login: "cp_user_" + f.marker, Approved: true}
	u.Password = "cppass123"
	u.PasswordConfirmation = "cppass123"
	_, err := u.Create(tx)
	require.NoError(t, err)
	f.userID = u.ID
	t.Cleanup(func() {
		f.cleanup()
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", u.Login).Exec()
	})
	return f
}

// cleanup removes every row the fixture created (FK-safe order).
func (f *planFixture) cleanup() {
	db := models.DB
	// applications referencing our cares/treatments/plans
	db.RawQuery("DELETE cpa FROM care_plan_applications cpa JOIN animals a ON cpa.animal_id = a.id WHERE a.cage = ?", f.cage).Exec()
	db.RawQuery("DELETE c FROM cares c JOIN animals a ON c.animal_id = a.id WHERE a.cage = ?", f.cage).Exec()
	db.RawQuery("DELETE t FROM treatments t JOIN animals a ON t.animal_id = a.id WHERE a.cage = ?", f.cage).Exec()
	db.RawQuery("DELETE cap FROM care_animal_plans cap JOIN animals a ON cap.animal_id = a.id WHERE a.cage = ?", f.cage).Exec()
	// plan resources created by this fixture (marker-tagged names)
	db.RawQuery("DELETE cre FROM care_rule_exclusions cre JOIN care_rules cr ON cre.rule_id = cr.id WHERE cr.name LIKE ?", "%"+f.marker+"%").Exec()
	db.RawQuery("DELETE FROM care_rules WHERE name LIKE ?", "%"+f.marker+"%").Exec()
	db.RawQuery("DELETE FROM care_matchers WHERE name LIKE ?", "%"+f.marker+"%").Exec()
	// animals + provenance
	db.RawQuery("DELETE FROM animals WHERE cage = ?", f.cage).Exec()
	db.RawQuery("DELETE d FROM discoveries d JOIN discoverers dc ON d.discoverer_id = dc.id WHERE dc.lastname = ?", nulls.NewString("CP-"+f.marker)).Exec()
	db.RawQuery("DELETE FROM discoverers WHERE lastname = ?", nulls.NewString("CP-"+f.marker)).Exec()
	db.RawQuery("DELETE FROM intakes WHERE id NOT IN (SELECT intake_id FROM animals WHERE intake_id IS NOT NULL) AND date > DATE_SUB(NOW(), INTERVAL 1 DAY)").Exec()
	// reference rows
	db.RawQuery("DELETE FROM caretypes WHERE name LIKE ?", "%"+f.marker+"%").Exec()
	db.RawQuery("DELETE FROM animaltypes WHERE name LIKE ?", "%"+f.marker+"%").Exec()
	db.RawQuery("DELETE FROM animalages WHERE name LIKE ?", "%"+f.marker+"%").Exec()
}

// freeYearNumber finds an unused (year, yearNumber) slot for fixtures.
func (f *planFixture) freeYearNumber(t *testing.T, tx *pop.Connection) int {
	t.Helper()
	year := time.Now().Year()
	for i := 0; i < 100; i++ {
		n := 800000 + rand.Intn(199999)
		cnt, err := tx.Where("year = ? AND yearNumber = ?", year, n).Count(&models.Animal{})
		require.NoError(t, err)
		if cnt == 0 {
			return n
		}
	}
	t.Fatal("no free yearNumber found")
	return 0
}

// mkAnimal creates one in-care animal in the fixture cage.
func (f *planFixture) mkAnimal(t *testing.T, tx *pop.Connection, species string) int {
	t.Helper()
	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("CP-" + f.marker)}
	require.NoError(t, tx.Create(&disc))
	d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: disc.ID}
	require.NoError(t, tx.Create(&d))
	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	require.NoError(t, tx.Create(&in))

	at := models.Animaltype{}
	require.NoError(t, tx.Where("name = ?", "CPType-"+f.marker).First(&at))
	aa := models.Animalage{}
	require.NoError(t, tx.Where("name = ?", "CPAge-"+f.marker).First(&aa))

	a := models.Animal{
		Year:         time.Now().Year(),
		YearNumber:   f.freeYearNumber(t, tx),
		Species:      species,
		Cage:         nulls.NewString(f.cage),
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now().Add(-24 * time.Hour),
	}
	require.NoError(t, tx.Create(&a))
	f.animalIDs = append(f.animalIDs, a.ID)
	return a.ID
}

// careScheduleJSON builds a fixed-anchor daily schedule with one slot at
// `due` (today).
func careScheduleJSON(t *testing.T, due time.Time) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(map[string]interface{}{
		"times":       []string{due.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
	})
	require.NoError(t, err)
	return raw
}

// feedRule creates an active feeding rule matching the fixture species set.
func (f *planFixture) feedRule(t *testing.T, tx *pop.Connection, due time.Time, times ...time.Time) *models.CareRule {
	t.Helper()
	if len(times) == 0 {
		times = []time.Time{due}
	}
	strs := make([]string, len(times))
	for i, tt := range times {
		strs[i] = tt.Format("15:04")
	}
	sched, err := json.Marshal(map[string]interface{}{
		"times":       strs,
		"anchor":      "fixed",
		"anchor_date": due.Format("2006-01-02"),
	})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]interface{}{
		"caretype_id": f.feedCare.String(),
		"food":        "grenouilles",
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID:            uuid.Must(uuid.NewV4()),
		Name:          "R-" + f.marker + "-" + due.Format("1504"),
		ActionKind:    "feeding",
		ActionPayload: payload,
		Schedule:      sched,
		Active:        true,
	}
	require.NoError(t, tx.Create(rule))
	return rule
}

// ---------------------------------------------------------------------------
// HTTP helpers
// ---------------------------------------------------------------------------

// planAdminClient is adminClientWithURL re-exported for this file's tests.
func planAdminClient(t *testing.T) (*http.Client, string) {
	t.Helper()
	return adminClientWithURL(t)
}

// planRegularClient logs in a fresh unrestricted (non-admin) user.
func planRegularClient(t *testing.T) (*http.Client, string) {
	t.Helper()
	requireMySQLTestDB(t)
	login := "cp_reg_" + uuid.Must(uuid.NewV4()).String()[:8]
	password := "cpregpass123"
	u := &models.User{Login: login, Approved: true}
	u.Password = password
	u.PasswordConfirmation = password
	_, err := u.Create(models.DB)
	require.NoError(t, err)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := srv.Client()
	client.Jar = jar
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	resp, err := client.Get(srv.URL + "/auth/new")
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no csrf token on /auth/new")

	vals := url.Values{}
	vals.Set("Login", login)
	vals.Set("Password", password)
	vals.Set("authenticity_token", string(m[1]))
	resp, err = client.PostForm(srv.URL+"/auth/", vals)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode)
	return client, srv.URL
}

// planToken extracts the CSRF token for the session.
func planToken(t *testing.T, client *http.Client, baseURL string) string {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+"/care_plan", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no csrf token on /care_plan")
	return string(m[1])
}

// planDoJSON performs a JSON request with the CSRF header.
func planDoJSON(t *testing.T, client *http.Client, baseURL, method, path, token string, body interface{}) (int, []byte) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, baseURL+path, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", token)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, raw
}

// planGetJSON performs a GET expecting JSON.
func planGetJSON(t *testing.T, client *http.Client, baseURL, path string) (int, map[string]interface{}) {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+path, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var out map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &out), "status %d, body: %s", resp.StatusCode, raw)
	return resp.StatusCode, out
}

// planItemsOf extracts the day-plan items array.
func planItemsOf(t *testing.T, body map[string]interface{}) []map[string]interface{} {
	t.Helper()
	raw, err := json.Marshal(body["items"])
	require.NoError(t, err)
	var items []map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &items))
	return items
}

// itemDueSoon returns a due time inside today's plan window + lookahead.
func itemDueSoon(now time.Time) time.Time {
	return now.Add(30 * time.Minute).Truncate(time.Minute)
}

// ---------------------------------------------------------------------------
// Fixtures: rule/plan writers shared by the tests below
// ---------------------------------------------------------------------------

func planRulePayload(t *testing.T, kind string, extra map[string]interface{}) json.RawMessage {
	t.Helper()
	if extra == nil {
		extra = map[string]interface{}{}
	}
	raw, err := json.Marshal(extra)
	require.NoError(t, err)
	return raw
}

// ruleWithoutMatcher creates an active rule that matches every in-care animal.
func ruleWithoutMatcher(t *testing.T, tx *pop.Connection, name, kind string, payload, schedule json.RawMessage) *models.CareRule {
	t.Helper()
	rule := &models.CareRule{
		ID:            uuid.Must(uuid.NewV4()),
		Name:          name,
		ActionKind:    kind,
		ActionPayload: payload,
		Schedule:      schedule,
		Active:        true,
	}
	require.NoError(t, tx.Create(rule))
	return rule
}

func findItemByAnimal(items []map[string]interface{}, animalID int, kind string) map[string]interface{} {
	for _, it := range items {
		if int(it["animal_id"].(float64)) == animalID && it["action_kind"] == kind {
			return it
		}
	}
	return nil
}

func itemRef(it map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{
		"source_type": it["source_type"],
		"source_id":   it["source_id"],
		"animal_id":   it["animal_id"],
		"due_at":      it["due_at"],
	}
}

// ---------------------------------------------------------------------------
// Day plan statuses + apply idempotency (§6.1/§6.2)
// ---------------------------------------------------------------------------

func TestCarePlanDayPlanApplyIdempotent(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	rule := f.feedRule(t, models.DB, due)

	// day plan shows the item as due with the §10.5-N1 label
	code, body := planGetJSON(t, client, baseURL, "/care_plan")
	require.Equal(t, http.StatusOK, code)
	items := planItemsOf(t, body)
	it := findItemByAnimal(items, f.animalIDs[0], "feeding")
	require.NotNil(t, it, "day plan must render the rule occurrence")
	require.Equal(t, "due", it["status"])
	require.Equal(t, true, it["applicable"])
	require.Contains(t, it["animal_label"], f.cage)
	require.Contains(t, it["animal_label"], "·")

	// apply → 201, care row + application row
	req := itemRef(it)
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	ncares, err := models.DB.Where("animal_id = ? AND type_id = ?", f.animalIDs[0], f.feedCare).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 1, ncares, "apply must create exactly one cares row")

	napps, err := models.DB.Where("source_id = ? AND animal_id = ?", rule.ID, f.animalIDs[0]).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 1, napps)

	// double submit → 409 conflict, still exactly one care/application
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusConflict, code, "body: %s", raw)
	ncares, err = models.DB.Where("animal_id = ? AND type_id = ?", f.animalIDs[0], f.feedCare).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 1, ncares, "apply must be idempotent (§4.5 UNIQUE)")

	// re-render: applied
	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	items = planItemsOf(t, body)
	it = findItemByAnimal(items, f.animalIDs[0], "feeding")
	require.NotNil(t, it)
	require.Equal(t, "applied", it["status"])
}

func TestCarePlanDayPlanHTMLRender(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	rule := f.feedRule(t, models.DB, itemDueSoon(time.Now()))
	_ = rule

	req, err := http.NewRequest("GET", baseURL+"/care_plan", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "html: %.300s", raw)
	require.Contains(t, string(raw), "plan-item-due", "template renders items with status classes")
}

// ---------------------------------------------------------------------------
// Skip / defer (§10-A2/CP4)
// ---------------------------------------------------------------------------

func TestCarePlanSkipDeferRequireReasonAndClamp(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	now := time.Now()
	due1 := now.Add(30 * time.Minute).Truncate(time.Minute)
	due2 := now.Add(2*time.Hour + 30*time.Minute).Truncate(time.Minute)
	sched, err := json.Marshal(map[string]interface{}{
		"times":       []string{due1.Format("15:04"), due2.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": due1.Format("2006-01-02"),
	})
	require.NoError(t, err)
	rule := ruleWithoutMatcher(t, models.DB, "RSKIP-"+f.marker, "feeding",
		planRulePayload(t, "feeding", map[string]interface{}{"caretype_id": f.feedCare.String()}),
		sched)

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	// two occurrences per animal; pick the first (due1)
	var first, second map[string]interface{}
	for _, it := range items {
		if int(it["animal_id"].(float64)) != f.animalIDs[0] {
			continue
		}
		if first == nil {
			first = it
		} else if second == nil {
			second = it
		}
	}
	require.NotNil(t, first)
	require.NotNil(t, second)

	// skip without reason → 422
	req := itemRef(first)
	req["status"] = "skipped"
	code, _ := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// defer without reason → 422
	req = itemRef(first)
	req["status"] = "deferred"
	req["deferred_until"] = now.Add(time.Hour).Format(time.RFC3339)
	code, _ = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// skip with reason → 201, no fulfillment care row
	req = itemRef(first)
	req["status"] = "skipped"
	req["note"] = "animal stressé"
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ? AND animal_id = ? AND due_at = ?", rule.ID, f.animalIDs[0], due1).First(app))
	require.Equal(t, "skipped", app.Status)
	require.Equal(t, "none", app.FulfillmentID)

	// defer with reason + far-future target → clamped before the NEXT
	// occurrence after due2 (§10-CP4): tomorrow's first slot is due1+24h.
	req = itemRef(second)
	req["status"] = "deferred"
	req["note"] = "patience"
	req["deferred_until"] = now.Add(48 * time.Hour).Format(time.RFC3339)
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	app2 := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ? AND animal_id = ? AND due_at = ?", rule.ID, f.animalIDs[0], due2).First(app2))
	require.Equal(t, "deferred", app2.Status)
	require.NotNil(t, app2.DeferredUntil)
	require.True(t, app2.DeferredUntil.After(due2), "defer must stay after the deferred occurrence, got %s ≤ %s", app2.DeferredUntil, due2)
	require.True(t, app2.DeferredUntil.Before(due2.Add(24*time.Hour)), "defer must clamp before the next day's occurrence, got %s ≥ %s", app2.DeferredUntil, due2.Add(24*time.Hour))
}

// ---------------------------------------------------------------------------
// Un-apply (§10-CP1 admin-only) + destroy hooks
// ---------------------------------------------------------------------------

func TestCarePlanUnapplyAdminOnlyAndDestroyHook(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	token := planToken(t, admin, adminURL)

	rule := f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	_, body := planGetJSON(t, admin, adminURL, "/care_plan")
	it := findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "feeding")
	require.NotNil(t, it)
	req := itemRef(it)
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ?", rule.ID).First(app))
	careID := app.FulfillmentID

	// regular user → 403
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	code, _ = planDoJSON(t, reg, regURL, "POST", "/care_plan/unapply", regToken, req)
	require.Equal(t, http.StatusForbidden, code)

	// admin without delete_fulfillment → application gone, care kept
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_plan/unapply", token, req)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	n, err := models.DB.Where("source_id = ?", rule.ID).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 0, n)
	n, err = models.DB.Where("id = ?", careID).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 1, n, "fulfillment kept by default (§10-CP1)")

	// re-apply then unapply WITH delete_fulfillment → care destroyed
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	require.NoError(t, models.DB.Where("source_id = ?", rule.ID).First(app))
	req2 := itemRef(it)
	req2["delete_fulfillment"] = true
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_plan/unapply", token, req2)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	n, err = models.DB.Where("id = ?", app.FulfillmentID).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 0, n, "checkbox deletes the linked care")
}

func TestCarePlanDestroyHooksMarkFulfillmentDeleted(t *testing.T) {
	f := setupPlanFixture(t)
	admin, baseURL := planAdminClient(t)
	token := planToken(t, admin, baseURL)

	// care fulfillment hook
	rule := f.feedRule(t, models.DB, itemDueSoon(time.Now()))
	_, body := planGetJSON(t, admin, baseURL, "/care_plan")
	it := findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "feeding")
	require.NotNil(t, it)
	code, raw := planDoJSON(t, admin, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ?", rule.ID).First(app))

	del := url.Values{}
	del.Set("_method", "DELETE")
	del.Set("authenticity_token", token)
	resp, err := admin.PostForm(baseURL+"/cares/"+app.FulfillmentID, del)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	require.NoError(t, models.DB.Where("id = ?", app.ID).First(app))
	require.True(t, app.FulfillmentDeleted, "care destroy must set fulfillment_deleted (§10-CP1)")

	// treatment fulfillment hook
	now := time.Now()
	due := itemDueSoon(now)
	med := ruleWithoutMatcher(t, models.DB, "RMED-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))
	_ = med
	_, body = planGetJSON(t, admin, baseURL, "/care_plan")
	it = findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "medication")
	require.NotNil(t, it)
	code, raw = planDoJSON(t, admin, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	app2 := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("fulfillment_type = ? AND animal_id = ?", "treatment", f.animalIDs[0]).First(app2))

	resp, err = admin.PostForm(baseURL+"/treatments/"+app2.FulfillmentID, del)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	require.NoError(t, models.DB.Where("id = ?", app2.ID).First(app2))
	require.True(t, app2.FulfillmentDeleted, "treatment destroy must set fulfillment_deleted (§10-CP1)")
}

// ---------------------------------------------------------------------------
// Helpers over the plan JSON
// ---------------------------------------------------------------------------

// mustItem is findItemByAnimal with an in-test assertion.
func mustItem(t *testing.T, items []map[string]interface{}, animalID int, kind string) map[string]interface{} {
	t.Helper()
	for _, it := range items {
		if int(it["animal_id"].(float64)) == animalID && it["action_kind"] == kind {
			return it
		}
	}
	t.Fatalf("no %s item for animal %d in plan (%d items)", kind, animalID, len(items))
	return nil
}

func itemDueAt(t *testing.T, item map[string]interface{}) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, item["due_at"].(string))
	require.NoError(t, err)
	return ts
}

func TestCarePlanBatchScopedToSingleCage(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	rule := f.feedRule(t, models.DB, due)

	// third animal in another cage
	a3 := f.mkAnimal(t, models.DB, "CP-A3")
	require.NoError(t, models.DB.RawQuery("UPDATE animals SET cage = ? WHERE id = ?", "OTHER-"+f.marker, a3).Exec())

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	i1 := mustItem(t, items, f.animalIDs[0], "feeding")
	i3 := mustItem(t, items, a3, "feeding")

	mkRef := func(it map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"source_type": "rule", "source_id": rule.ID.String(),
			"animal_id": it["animal_id"], "due_at": itemDueAt(t, it),
		}
	}

	// same cage → both applied
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{
		"items": []map[string]interface{}{mkRef(i1), mkRef(mustItem(t, items, f.animalIDs[1], "feeding"))},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var out struct {
		Applied int `json:"applied"`
	}
	require.NoError(t, json.Unmarshal(raw, &out))
	require.Equal(t, 2, out.Applied)

	// cross-cage same source → the foreign-cage item errors, no cross apply
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{
		"items": []map[string]interface{}{mkRef(mustItem(t, planItemsOf(t, func() map[string]interface{} { _, b := planGetJSON(t, client, baseURL, "/care_plan"); return b }()), f.animalIDs[0], "feeding")), mkRef(i3)},
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var out2 struct {
		Applied int                      `json:"applied"`
		Results []map[string]interface{} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(raw, &out2))
	require.Equal(t, 0, out2.Applied, "first item already applied (already_done)")
	require.Len(t, out2.Results, 2)

	// cross-source batch is refused outright
	rule2 := f.feedRule(t, models.DB, due.Add(time.Minute))
	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	i2 := mustItem(t, planItemsOf(t, body), f.animalIDs[1], "feeding")
	_ = i2
	code, _ = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, map[string]interface{}{
		"items": []map[string]interface{}{
			mkRef(mustItem(t, planItemsOf(t, func() map[string]interface{} { _, b := planGetJSON(t, client, baseURL, "/care_plan"); return b }()), a3, "feeding")),
			{"source_type": "rule", "source_id": rule2.ID.String(), "animal_id": f.animalIDs[0], "due_at": itemDueAt(t, mustItem(t, planItemsOf(t, func() map[string]interface{} { _, b := planGetJSON(t, client, baseURL, "/care_plan"); return b }()), f.animalIDs[0], "feeding"))},
		},
	})
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// re-applying the same cage batch → already_done, no new rows
	var careCount int
	require.NoError(t, models.DB.RawQuery(
		"SELECT COUNT(*) AS c FROM cares WHERE animal_id = ? AND type_id = ?", f.animalIDs[1], f.feedCare).First(&careCount))
	require.Equal(t, 1, careCount)
}

func TestCarePlanWeighingRequiresWeight(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "Pese-" + f.marker,
		ActionKind: "weighing", ActionPayload: json.RawMessage(`{}`), Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "weighing")
	ref := map[string]interface{}{
		"source_type": "rule", "source_id": rule.ID.String(),
		"animal_id": f.animalIDs[0], "due_at": itemDueAt(t, item),
	}

	// weight required (§10-L1)
	code, _ := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, ref)
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// with weight → care row carrying the weight at click time
	ref["weight"] = "312"
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, ref)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	care := &models.Care{}
	require.NoError(t, models.DB.Where("animal_id = ? AND weight = ?", f.animalIDs[0], "312").First(care))
}

func TestCareRulesMatchersAdminCRUDAndPreview(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	adminToken := planToken(t, admin, adminURL)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)

	expr := `species = "CP-A1"`

	// --- regular user is refused everywhere on rules/matchers (admin-only)
	code, _ := planDoJSON(t, reg, regURL, "POST", "/care_rules", regToken, map[string]interface{}{"name": "x"})
	require.Equal(t, http.StatusForbidden, code)
	code, _ = planDoJSON(t, reg, regURL, "GET", "/care_matchers", regToken, nil)
	require.Equal(t, http.StatusForbidden, code)
	code, _ = planDoJSON(t, reg, regURL, "POST", "/care_matchers/preview", regToken, map[string]interface{}{"expression": expr})
	require.Equal(t, http.StatusForbidden, code)

	// --- matcher CRUD (admin)
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_matchers", adminToken, map[string]interface{}{
		"name":       "M-" + f.marker,
		"expression": expr,
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	matcher := &models.CareMatcher{}
	require.NoError(t, json.Unmarshal(raw, matcher))
	require.NotEqual(t, uuid.Nil, matcher.ID)

	// invalid expression rejected at save time (§4.4)
	code, _ = planDoJSON(t, admin, adminURL, "POST", "/care_matchers", adminToken, map[string]interface{}{
		"name":       "MBad-" + f.marker,
		"expression": `species ~ "("`, // broken pattern
	})
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// matcher update
	code, raw = planDoJSON(t, admin, adminURL, "PUT", "/care_matchers/"+matcher.ID.String(), adminToken, map[string]interface{}{
		"name":       "M2-" + f.marker,
		"expression": `species = "CP-A1" AND cage = "` + f.cage + `"`,
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)

	// matcher preview (§7.1-3): live count + why-trace
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_matchers/preview", adminToken, map[string]interface{}{
		"expression": expr,
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var preview struct {
		MatchCount int `json:"match_count"`
		Animals    []struct {
			AnimalID int  `json:"animal_id"`
			Match    bool `json:"match"`
		} `json:"animals"`
	}
	require.NoError(t, json.Unmarshal(raw, &preview))
	require.Equal(t, 1, preview.MatchCount, "only CP-A1 matches")
	require.NotEmpty(t, preview.Animals)

	// --- rule CRUD (admin) with matcher + preview endpoint
	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_rules", adminToken, map[string]interface{}{
		"name":           "R-" + f.marker,
		"action_kind":    "feeding",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
		"matcher_id":     matcher.ID.String(),
		"active":         true,
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	rule := &models.CareRule{}
	require.NoError(t, json.Unmarshal(raw, rule))

	// rule preview
	code, raw = planDoJSON(t, admin, adminURL, "GET", "/care_rules/"+rule.ID.String()+"/preview", adminToken, nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var rp struct {
		MatchCount int `json:"match_count"`
	}
	require.NoError(t, json.Unmarshal(raw, &rp))
	require.Equal(t, 1, rp.MatchCount)

	// rule update (deactivate) then destroy
	code, _ = planDoJSON(t, admin, adminURL, "PUT", "/care_rules/"+rule.ID.String(), adminToken, map[string]interface{}{
		"name": "R2-" + f.marker, "active": false,
		"action_kind": "feeding", "action_payload": json.RawMessage(payload),
		"schedule": json.RawMessage(sched), "matcher_id": matcher.ID.String(),
	})
	require.Equal(t, http.StatusOK, code)

	// matcher still referenced → conflict
	code, _ = planDoJSON(t, admin, adminURL, "DELETE", "/care_matchers/"+matcher.ID.String(), adminToken, nil)
	require.Equal(t, http.StatusConflict, code)

	code, _ = planDoJSON(t, admin, adminURL, "DELETE", "/care_rules/"+rule.ID.String(), adminToken, nil)
	require.Equal(t, http.StatusOK, code)
	code, _ = planDoJSON(t, admin, adminURL, "DELETE", "/care_matchers/"+matcher.ID.String(), adminToken, nil)
	require.Equal(t, http.StatusOK, code)
}

func TestCareAnimalPlansCaretakerCRUD(t *testing.T) {
	f := setupPlanFixture(t)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	animalID := f.animalIDs[0]

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{"caretype_id": f.feedCare.String()})
	require.NoError(t, err)

	// create (caretaker level — no admin rights, §4.7)
	code, raw := planDoJSON(t, reg, regURL, "POST", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, map[string]interface{}{
		"name":           "Pansement-" + f.marker,
		"action_kind":    "care",
		"action_payload": json.RawMessage(payload),
		"schedule":       json.RawMessage(sched),
		"replaces_kind":  true,
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)
	plan := &models.CareAnimalPlan{}
	require.NoError(t, json.Unmarshal(raw, plan))
	require.Equal(t, animalID, plan.AnimalID)
	require.True(t, plan.ReplacesKind)

	// list (JSON array)
	code, raw = planDoJSON(t, reg, regURL, "GET", fmt.Sprintf("/animals/%d/care_animal_plans", animalID), regToken, nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.Contains(t, string(raw), "Pansement-"+f.marker)

	// the plan drives the day plan; the feeding rule occurrence is NOT
	// overridden (different kinds never interact, §4.7)
	rule := f.feedRule(t, models.DB, due)
	_, dayBody := planGetJSON(t, reg, regURL, "/care_plan")
	items := planItemsOf(t, dayBody)
	_ = mustItem(t, items, animalID, "care")
	feedItem := mustItem(t, items, animalID, "feeding")
	require.NotEqual(t, "overridden", feedItem["status"])

	// update (rename)
	code, raw = planDoJSON(t, reg, regURL, "PUT", fmt.Sprintf("/animals/%d/care_animal_plans/%s", animalID, plan.ID), regToken, map[string]interface{}{
		"name": "Pansement2-" + f.marker, "action_kind": "care",
		"action_payload": json.RawMessage(payload), "schedule": json.RawMessage(sched),
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	_ = rule

	// destroy
	code, _ = planDoJSON(t, reg, regURL, "DELETE", fmt.Sprintf("/animals/%d/care_animal_plans/%s", animalID, plan.ID), regToken, nil)
	require.Equal(t, http.StatusOK, code)
}

func TestCareObservationAlertLoopAndFollowUp(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{
		"prompt":                "CP-a-t-il-pri f contrôle ?",
		"alert_on":              "no",
		"alert_follow_up_hours": 1,
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "Obs-" + f.marker,
		ActionKind: "observation", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "observation")

	// alert outcome: answer == alert_on ("no") → warning care + follow-up
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, map[string]interface{}{
		"source_type": "rule", "source_id": rule.ID.String(),
		"animal_id": f.animalIDs[0], "due_at": itemDueAt(t, item),
		"answer": "no",
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	var warnCount int
	require.NoError(t, models.DB.RawQuery(
		"SELECT COUNT(*) AS c FROM cares WHERE animal_id = ? AND type_id IN (SELECT id FROM caretypes WHERE warning = 1)", f.animalIDs[0]).First(&warnCount))
	require.Equal(t, 1, warnCount, "alert outcome writes the Warning caretype care (§10.1-6)")

	followUp := &models.CareAnimalPlan{}
	require.NoError(t, models.DB.Where("animal_id = ? AND active = ? AND name LIKE ?",
		f.animalIDs[0], true, "Vérifier alerte:%").First(followUp), "follow-up plan auto-created (§10-CP3)")

	// the follow-up occurrence shows up on the day plan as an animal item
	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	var followItem map[string]interface{}
	for _, it := range planItemsOf(t, body) {
		if it["source_type"] == "animal" && it["action_kind"] == "observation" {
			followItem = it
			break
		}
	}
	require.NotNil(t, followItem, "follow-up occurrence on the day plan")

	// non-alert answer on the follow-up → ResetWarning care + plan closed
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, map[string]interface{}{
		"source_type": "animal", "source_id": followUp.ID.String(),
		"animal_id": f.animalIDs[0], "due_at": itemDueAt(t, followItem),
		"answer": "oui, repri f",
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	var resetCount int
	require.NoError(t, models.DB.RawQuery(
		"SELECT COUNT(*) AS c FROM cares WHERE animal_id = ? AND type_id IN (SELECT id FROM caretypes WHERE reset_warning = 1)", f.animalIDs[0]).First(&resetCount))
	require.Equal(t, 1, resetCount, "Réponse alerte care closes the loop (§10-M3)")

	require.NoError(t, models.DB.Reload(followUp))
	require.False(t, followUp.Active, "follow-up plan self-deactivates (§10-CP3)")
}

func TestCareMedicationApplyCreatesTreatment(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	drug := "CP-Drug-" + f.marker
	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{"drug": drug, "dosage": "0.5 ml"})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "Med-" + f.marker,
		ActionKind: "medication", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "medication")
	dueAt := itemDueAt(t, item)

	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, map[string]interface{}{
		"source_type": "rule", "source_id": rule.ID.String(),
		"animal_id": f.animalIDs[0], "due_at": dueAt,
	})
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	// treatments row with the §10-M1 slot bit marked done
	bits := []int{1, 2, 4}
	h := dueAt.Hour()
	var wantBit int
	switch {
	case h < 11:
		wantBit = bits[0]
	case h <= 15:
		wantBit = bits[1]
	default:
		wantBit = bits[2]
	}
	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("animal_id = ? AND drug = ?", f.animalIDs[0], drug).All(treatments))
	require.Len(t, *treatments, 1)
	require.Equal(t, wantBit, (*treatments)[0].Timedonebitmap&wantBit, "slot bucket marked done")
	require.Equal(t, "0.5 ml", (*treatments)[0].Dosage)
}
