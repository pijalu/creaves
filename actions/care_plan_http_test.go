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
	"sort"
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

	// double submit → 409 conflict (idempotent message, §4.5), still exactly one care/application
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusConflict, code, "body: %s", raw)
	require.Contains(t, string(raw), "already recorded (idempotent",
		"double apply must report idempotency, not the hors-délai window: %s", raw)
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

// TestCarePlanErrorContractDetail proves the 4xx bodies keep the server's
// real message in a NON-development env (buffalo's default error handler
// masks err.Error() to http.StatusText outside development — bugs.md H1).
// This test runs under GO_ENV=test, i.e. exactly the masked path.
func TestCarePlanErrorContractDetail(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	now := time.Now()

	// --- 422: skip without note keeps the §10-CP4 reason -------------------
	due := itemDueSoon(now)
	f.feedRule(t, models.DB, due)
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	it := findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "feeding")
	require.NotNil(t, it)

	req := itemRef(it)
	req["status"] = "skipped"
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusUnprocessableEntity, code, "body: %s", raw)
	require.Contains(t, string(raw), "reason is mandatory",
		"422 body must carry the real validation message, not 'Unprocessable Entity': %s", raw)
	require.Contains(t, string(raw), `"code":422`)

	// --- 409: out-of-window apply keeps the hors-délai message -------------
	// two slots 2h/1h ago: the earlier occurrence's window closed when the
	// later one became due (§10-A1) — both lie inside the plan window.
	t1 := now.Add(-2 * time.Hour).Truncate(time.Minute)
	t2 := now.Add(-1 * time.Hour).Truncate(time.Minute)
	f.feedRule(t, models.DB, t1, t1, t2)
	_, body = planGetJSON(t, client, baseURL, "/care_plan")
	var locked map[string]interface{}
	for _, cand := range planItemsOf(t, body) {
		if int(cand["animal_id"].(float64)) == f.animalIDs[0] &&
			cand["action_kind"] == "feeding" && cand["applicable"] == false {
			locked = cand
			break
		}
	}
	require.NotNil(t, locked, "plan must expose a hors-délai occurrence")

	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(locked))
	require.Equal(t, http.StatusConflict, code, "body: %s", raw)
	require.Contains(t, string(raw), "hors délai",
		"409 body must carry the real hors-délai message, not 'Conflict': %s", raw)
	require.Contains(t, string(raw), `"code":409`)

	// --- 403: non-admin on an admin plan endpoint keeps the message --------
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	code, raw = planDoJSON(t, reg, regURL, "POST", "/care_plan/unapply", regToken, itemRef(it))
	require.Equal(t, http.StatusForbidden, code, "body: %s", raw)
	require.Contains(t, string(raw), "admin only",
		"403 body must carry the real authz message, not 'Forbidden': %s", raw)
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
	// bugs.md U1: feeding items render as one cage × diet card, not tier
	// rows — the card carries the apply-group button and per-animal chips.
	require.Contains(t, string(raw), "plan-feeding-card", "template renders the feeding card")
	require.Contains(t, string(raw), "plan-dot-due", "feeding chips carry the per-animal status dot")
	require.Contains(t, string(raw), "plan-feeding-apply", "feeding card carries the apply-group button")
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
		if it["source_type"] == "animal" && it["source_id"] == followUp.ID.String() {
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

	// treatments row with the occurrence's per-time entry done
	// (bugs.md R5-3b, U25/D-e — the bitmap is dormant)
	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("animal_id = ? AND drug = ?", f.animalIDs[0], drug).All(treatments))
	require.Len(t, *treatments, 1)
	treatment := (*treatments)[0]
	require.Equal(t, "0.5 ml", treatment.Dosage)

	entries := models.TreatmentTimeEntries{}
	require.NoError(t, models.DB.Where("treatment_id = ?", treatment.ID).Order("due_at asc").All(&entries))
	require.NotEmpty(t, entries, "per-time entries seeded (U25/D-e)")
	doneAt := dueAt.Format("15:04")
	var done *models.TreatmentTimeEntry
	for i := range entries {
		e := &entries[i]
		if e.TimeLabel == doneAt {
			done = e
		}
	}
	require.NotNil(t, done, "entry for the occurrence time %s", doneAt)
	require.Equal(t, models.TreatmentEntryStatusDone, done.Status, "occurrence entry done")
	require.True(t, done.AppliedAt.Valid, "applied_at stamped")
	require.True(t, done.ApplicationID.Valid, "entry linked to the application")
	require.Equal(t, models.TreatmentEntrySourceProtocol, done.Source)
	// the dormant bitmap must NOT be written anymore (D-e)
	require.Zero(t, treatment.Timedonebitmap, "Timedonebitmap dormant (U25/D-e)")
}

// TestCareMedicationSameSlotKeepsDistinctEntries replaces the legacy
// same-bucket remarks-appendum behavior (bugs.md R5-3b, U25/D-e): a second
// same-slot apply gets its OWN done entry — per-time storage keeps every
// administration distinct; both applications still share the treatments
// row and are both recorded.
func TestCareMedicationSameSlotKeepsDistinctEntries(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	drug := "CP-Drug-" + f.marker
	// two occurrences inside the same §10-M1 bucket, both due today (the
	// fulfillment lookup keys the treatments row on the due day while the
	// row is dated at click time — same day is what makes them collide)
	now := time.Now()
	var slot1, slot2 time.Time
	found := false
	for off := 30; off <= 300 && !found; off += 15 {
		slot1 = now.Add(time.Duration(off) * time.Minute).Truncate(time.Minute)
		slot2 = slot1.Add(20 * time.Minute)
		if slot1.Year() == slot2.Year() && slot1.YearDay() == slot2.YearDay() &&
			treatmentBucketBit(slot1) == treatmentBucketBit(slot2) {
			found = true
		}
	}
	if !found {
		t.Skip("no two same-day, same-bucket future slots left today")
	}
	sched, err := json.Marshal(map[string]interface{}{
		"times":       []string{slot1.Format("15:04"), slot2.Format("15:04")},
		"anchor":      "fixed",
		"anchor_date": slot1.Format("2006-01-02"),
	})
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]interface{}{"drug": drug, "dosage": "0.5 ml"})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "Med-" + f.marker,
		ActionKind: "medication", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	// apply the first occurrence → creates the treatments row
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	occ := []map[string]interface{}{}
	for _, it := range items {
		if int(it["animal_id"].(float64)) == f.animalIDs[0] && it["action_kind"] == "medication" &&
			it["source_id"] == rule.ID.String() {
			occ = append(occ, it)
		}
	}
	require.GreaterOrEqual(t, len(occ), 2, "today's two occurrences expected (window also renders tomorrow's)")
	sort.Slice(occ, func(i, j int) bool {
		return occ[i]["due_at"].(string) < occ[j]["due_at"].(string)
	})

	req1 := itemRef(occ[0])
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req1)
	require.Equal(t, http.StatusCreated, code, "first apply: %s", raw)

	// second occurrence, same slot, with a note → distinct done entry
	req2 := itemRef(occ[1])
	req2["note"] = "notedose"
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req2)
	require.Equal(t, http.StatusCreated, code, "second apply: %s", raw)

	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("animal_id = ? AND drug = ?", f.animalIDs[0], drug).All(treatments))
	require.Len(t, *treatments, 1, "same-slot applies share one treatments row")

	entries := models.TreatmentTimeEntries{}
	require.NoError(t, models.DB.Where("treatment_id = ?", (*treatments)[0].ID).Order("due_at asc").All(&entries))
	require.Len(t, entries, 2, "one entry per expected time (U25/D-e)")
	require.Equal(t, models.TreatmentEntryStatusDone, entries[0].Status)
	require.Equal(t, models.TreatmentEntryStatusDone, entries[1].Status)
	require.True(t, entries[1].ApplicationID.Valid, "second entry linked to its application")
	require.Equal(t, "notedose", entries[1].Note.String, "note carried on the entry")

	napps, err := models.DB.Where("source_id = ? AND animal_id = ?", rule.ID, f.animalIDs[0]).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 2, napps, "both applications recorded")
}

// ---------------------------------------------------------------------------
// §10-B6 / §10-L2 — manual dosage path (bugs.md M1)
// ---------------------------------------------------------------------------

// TestCareMedicationDosageRequiredFlow proves the §10-B6 contract end-to-end:
// dosage resolution failure (table path, no weight on record) → 422
// structured dosage_required body carrying the last-weight fields (§10-L2) →
// resubmit with a manual dosage → 201, the treatment row carries the manual
// dosage and the application snapshot records dosage_source="manual".
func TestCareMedicationDosageRequiredFlow(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	// table-driven dosage: drug + dosages row exist, but the animal has a
	// (stale) weight only — resolution fails via a MISSING dosage row would
	// also warn; here the drug has no dosage row for this animal type.
	drug := &models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "CP-DrugT-" + f.marker}
	require.NoError(t, models.DB.Create(drug))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})

	// §10-L2: the animal has a weighed care on record → the warning must
	// carry its value and date.
	weighedAt := time.Now().Add(-48 * time.Hour)
	require.NoError(t, models.DB.Create(&models.Care{
		Date: weighedAt, AnimalID: f.animalIDs[0], TypeID: f.defCare, Weight: nulls.NewString("310"),
	}))

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{
		"drug": drug.Name, "dosage_from_dosages_table": true,
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "MedT-" + f.marker,
		ActionKind: "medication", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "medication")

	// 1st attempt, no dosage → 422 dosage_required with §10-L2 weight fields
	req := itemRef(item)
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusUnprocessableEntity, code, "body: %s", raw)
	var errBody map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &errBody))
	require.Equal(t, "dosage_required", errBody["error"])
	require.Equal(t, "no_dosage_row", errBody["reason"])
	require.NotEmpty(t, errBody["detail"], "human detail for the dialog")
	require.Equal(t, "310", errBody["last_weight"], "§10-L2 last weight value")
	require.NotEmpty(t, errBody["last_weight_at"], "§10-L2 last weight date")

	// nothing written yet
	napps, err := models.DB.Where("source_id = ?", rule.ID).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 0, napps, "422 must not record an application")

	// resubmit with a manual dosage → 201, treatment carries it verbatim
	req["dosage"] = "0.12 ml"
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("animal_id = ? AND drug = ?", f.animalIDs[0], drug.Name).All(treatments))
	require.Len(t, *treatments, 1)
	require.Equal(t, "0.12 ml", (*treatments)[0].Dosage, "manual dosage stored verbatim")

	// application snapshot flags the manual source (audit trail)
	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("source_id = ? AND animal_id = ?", rule.ID, f.animalIDs[0]).First(app))
	var snap map[string]interface{}
	require.NoError(t, json.Unmarshal(app.SourceSnapshot, &snap))
	require.Equal(t, "manual", snap["dosage_source"])
	require.Equal(t, "0.12 ml", snap["manual_dosage"])
}

// TestCareMedicationManualDosageSameBucket covers bugs.md U11: when a
// treatment row for the same animal/drug/day/bucket already exists (e.g.
// pre-created by a converted series) and the apply supplies a manual dosage,
// the operational row must carry the manual dosage verbatim — not keep the
// stale plan dosage.
func TestCareMedicationManualDosageSameBucket(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	drug := &models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "CP-DrugB-" + f.marker}
	require.NoError(t, models.DB.Create(drug))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{
		"drug": drug.Name, "dosage": "0.05 ml",
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "MedB-" + f.marker,
		ActionKind: "medication", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	item := mustItem(t, planItemsOf(t, body), f.animalIDs[0], "medication")

	// Pre-existing same-bucket row, not yet done, carrying the plan dosage —
	// the shape converted treatment series produce.
	dueAt, err := time.Parse(time.RFC3339, item["due_at"].(string))
	require.NoError(t, err)
	bit := treatmentBucketBit(dueAt)
	dayStart := time.Date(dueAt.Year(), dueAt.Month(), dueAt.Day(), 0, 0, 0, 0, dueAt.Location())
	pre := &models.Treatment{
		Date: dayStart, AnimalID: f.animalIDs[0], Drug: drug.Name,
		Dosage: "0.05 ml", Timebitmap: bit, Timedonebitmap: 0,
	}
	require.NoError(t, models.DB.Create(pre))

	// Apply with a manual dosage → 201, the same row is completed AND the
	// dosage is corrected verbatim (§10-B6 / U11).
	req := itemRef(item)
	req["dosage"] = "0.12 ml (manuel)"
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("animal_id = ? AND drug = ?", f.animalIDs[0], drug.Name).All(treatments))
	require.Len(t, *treatments, 1, "same-bucket apply completes the existing row, no duplicate")
	require.Equal(t, pre.ID, (*treatments)[0].ID)
	require.Equal(t, "0.12 ml (manuel)", (*treatments)[0].Dosage, "U11: manual dosage stored verbatim")

	// U25/D-e: the done state moved to the per-time entry.
	entries := models.TreatmentTimeEntries{}
	require.NoError(t, models.DB.Where("treatment_id = ?", pre.ID).All(&entries))
	require.Len(t, entries, 1, "entry created on the legacy row")
	require.Equal(t, models.TreatmentEntryStatusDone, entries[0].Status, "bucket entry done")
	require.True(t, entries[0].ApplicationID.Valid, "entry linked to the application")
	require.Zero(t, (*treatments)[0].Timedonebitmap, "Timedonebitmap dormant (U25/D-e)")
}

// TestCareMedicationDosageRequiredNoWeight covers the no-weight-on-record
// branch (§10-L2: weight fields absent) plus the batch path reporting
// dosage_required per item.
func TestCareMedicationDosageRequiredNoWeight(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	drug := &models.Drug{ID: uuid.Must(uuid.NewV4()), Name: "CP-DrugW-" + f.marker}
	require.NoError(t, models.DB.Create(drug))
	at := models.Animaltype{}
	require.NoError(t, models.DB.Where("name = ?", "CPType-"+f.marker).First(&at))
	dosRow := &models.Dosage{
		ID: uuid.Must(uuid.NewV4()), DrugID: drug.ID, AnimaltypeID: at.ID,
		Enabled: true, DosagePerGrams: nulls.NewFloat64(0.005), DosagePerGramsUnit: nulls.NewString("ml"),
	}
	require.NoError(t, models.DB.Create(dosRow))
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM dosages WHERE id = ?", dosRow.ID).Exec()
		models.DB.RawQuery("DELETE FROM drugs WHERE id = ?", drug.ID).Exec()
	})

	due := itemDueSoon(time.Now())
	sched := careScheduleJSON(t, due)
	payload, err := json.Marshal(map[string]interface{}{
		"drug": drug.Name, "dosage_from_dosages_table": true,
	})
	require.NoError(t, err)
	rule := &models.CareRule{
		ID: uuid.Must(uuid.NewV4()), Name: "MedW-" + f.marker,
		ActionKind: "medication", ActionPayload: payload, Schedule: sched, Active: true,
	}
	require.NoError(t, models.DB.Create(rule))

	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	it1 := mustItem(t, items, f.animalIDs[0], "medication")

	// single apply: no weight anywhere → 422, no weight fields
	code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it1))
	require.Equal(t, http.StatusUnprocessableEntity, code, "body: %s", raw)
	var errBody map[string]interface{}
	require.NoError(t, json.Unmarshal(raw, &errBody))
	require.Equal(t, "dosage_required", errBody["error"])
	require.Equal(t, "no_weight", errBody["reason"])
	require.Empty(t, errBody["last_weight"], "no weight on record → field empty")

	// batch path: per-item dosage_required, then per-item manual resubmit
	it2 := mustItem(t, items, f.animalIDs[1], "medication")
	batch := map[string]interface{}{"items": []map[string]interface{}{itemRef(it1), itemRef(it2)}}
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token, batch)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	var batchBody struct {
		Applied int                      `json:"applied"`
		Results []map[string]interface{} `json:"results"`
	}
	require.NoError(t, json.Unmarshal(raw, &batchBody))
	require.Equal(t, 0, batchBody.Applied)
	require.Len(t, batchBody.Results, 2)
	for _, r := range batchBody.Results {
		require.Equal(t, "dosage_required", r["status"], "batch item: %v", r)
		require.Equal(t, "no_weight", r["reason"])
	}

	// resubmit the batch with manual dosages → both applied
	b1, b2 := itemRef(it1), itemRef(it2)
	b1["dosage"], b2["dosage"] = "0.1 ml", "0.2 ml"
	code, raw = planDoJSON(t, client, baseURL, "POST", "/care_plan/apply_batch", token,
		map[string]interface{}{"items": []map[string]interface{}{b1, b2}})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, json.Unmarshal(raw, &batchBody))
	require.Equal(t, 2, batchBody.Applied, "body: %s", raw)
	for _, r := range batchBody.Results {
		require.Equal(t, "applied", r["status"], "batch item: %v", r)
	}

	treatments := &models.Treatments{}
	require.NoError(t, models.DB.Where("drug = ?", drug.Name).All(treatments))
	require.Len(t, *treatments, 2)
	byAnimal := map[int]string{}
	for _, tr := range *treatments {
		byAnimal[tr.AnimalID] = tr.Dosage
	}
	require.Equal(t, "0.1 ml", byAnimal[f.animalIDs[0]])
	require.Equal(t, "0.2 ml", byAnimal[f.animalIDs[1]])
}

// ---------------------------------------------------------------------------
// §10-CP1 rework: a non-admin may undo an APPLIED MEDICATION only (the slot
// toggle is designed to be reversible); every other kind stays admin-only.
// ---------------------------------------------------------------------------

// TestCarePlanUnapplyMedicationNonAdmin: regular user applies a medication
// (slot toggle), then undoes it with delete_fulfillment → application row
// gone, treatments row destroyed. A second unapply (nothing applied) → 403.
func TestCarePlanUnapplyMedicationNonAdmin(t *testing.T) {
	f := setupPlanFixture(t)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)

	now := time.Now()
	due := itemDueSoon(now)
	ruleWithoutMatcher(t, models.DB, "RMED-UNDO-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))

	_, body := planGetJSON(t, reg, regURL, "/care_plan")
	it := findItemByAnimal(planItemsOf(t, body), f.animalIDs[0], "medication")
	require.NotNil(t, it)
	req := itemRef(it)

	// apply (no confirm needed for medication toggles — server side unchanged)
	code, raw := planDoJSON(t, reg, regURL, "POST", "/care_plan/apply", regToken, req)
	require.Equal(t, http.StatusCreated, code, "body: %s", raw)

	app := &models.CarePlanApplication{}
	require.NoError(t, models.DB.Where("fulfillment_type = ? AND animal_id = ?", "treatment", f.animalIDs[0]).First(app))
	require.Equal(t, "applied", app.Status)
	treatmentID := app.FulfillmentID

	// non-admin undo WITH delete_fulfillment → allowed for medication
	req["delete_fulfillment"] = true
	code, raw = planDoJSON(t, reg, regURL, "POST", "/care_plan/unapply", regToken, req)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	n, err := models.DB.Where("id = ?", app.ID).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 0, n, "application row removed by undo")
	n, err = models.DB.Where("id = ?", treatmentID).Count(&models.Treatments{})
	require.NoError(t, err)
	require.Equal(t, 0, n, "treatments row destroyed with delete_fulfillment")

	// nothing applied anymore → unapply again must be the uniform 403 (no
	// information leak about which refs ever had an application row)
	delete(req, "delete_fulfillment")
	code, _ = planDoJSON(t, reg, regURL, "POST", "/care_plan/unapply", regToken, req)
	require.Equal(t, http.StatusForbidden, code)
}

// TestCarePlanUnapplyNonMedicationNonAdminDenied: feeding stays admin-only
// for regular users even when applied.
func TestCarePlanUnapplyNonMedicationNonAdminDenied(t *testing.T) {
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
	require.NoError(t, models.DB.Where("source_id = ?", rule.ID).First(&models.CarePlanApplication{}))

	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)
	code, _ = planDoJSON(t, reg, regURL, "POST", "/care_plan/unapply", regToken, req)
	require.Equal(t, http.StatusForbidden, code, "feeding undo must stay admin-only for regular users")
}

// TestCarePlanUnapplyMedicationSiblingGuard (§10-M1): two medication
// applications sharing ONE treatments row (same bucket collision) — undoing
// one with delete_fulfillment must NOT destroy the shared row while the
// sibling application still points at it.
func TestCarePlanUnapplyMedicationSiblingGuard(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	token := planToken(t, admin, adminURL)

	now := time.Now()
	due := itemDueSoon(now) // same bucket for both rules
	ruleWithoutMatcher(t, models.DB, "RMED-S1-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrugA-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))
	ruleWithoutMatcher(t, models.DB, "RMED-S2-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrugA-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, due))

	_, body := planGetJSON(t, admin, adminURL, "/care_plan")
	items := planItemsOf(t, body)
	var meds []map[string]interface{}
	for _, it := range items {
		if int(it["animal_id"].(float64)) == f.animalIDs[0] && it["action_kind"] == "medication" && it["source_type"] == "rule" && it["status"] == "due" {
			meds = append(meds, it)
		}
	}
	require.Len(t, meds, 2, "both medication rules must produce an occurrence")

	// apply both → §10-M1: both applications share one treatments row
	var apps []models.CarePlanApplication
	for _, it := range meds {
		code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_plan/apply", token, itemRef(it))
		require.Equal(t, http.StatusCreated, code, "body: %s", raw)
		app := &models.CarePlanApplication{}
		require.NoError(t, models.DB.Where("source_id = ? AND animal_id = ?", it["source_id"], f.animalIDs[0]).First(app))
		apps = append(apps, *app)
	}
	require.Equal(t, apps[0].FulfillmentID, apps[1].FulfillmentID, "same-bucket medications must share one treatments row (§10-M1)")
	require.Equal(t, "treatment", apps[0].FulfillmentType)
	sharedTreatmentID := apps[0].FulfillmentID

	// undo the first WITH delete_fulfillment → sibling guard keeps the row
	req1 := itemRef(meds[0])
	req1["delete_fulfillment"] = true
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_plan/unapply", token, req1)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	n, err := models.DB.Where("id = ?", apps[0].ID).Count(&models.CarePlanApplication{})
	require.NoError(t, err)
	require.Equal(t, 0, n, "first application row removed")
	n, err = models.DB.Where("id = ?", sharedTreatmentID).Count(&models.Treatments{})
	require.NoError(t, err)
	require.Equal(t, 1, n, "§10-M1: shared treatments row must survive while the sibling application uses it")

	// undo the second → last reference gone → row destroyed
	req2 := itemRef(meds[1])
	req2["delete_fulfillment"] = true
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_plan/unapply", token, req2)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	n, err = models.DB.Where("id = ?", sharedTreatmentID).Count(&models.Treatments{})
	require.NoError(t, err)
	require.Equal(t, 0, n, "treatments row destroyed once the last application releases it")
}
