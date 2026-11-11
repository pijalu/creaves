package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// "Protocols to confirm" feature tests (BUG-52e): the MANUAL
// needs_confirmation flag on care_animal_plans — toggle endpoint, the
// admin report screen, and the NeedsReview exposure on the feeding
// summary. The converter NEVER sets this flag; only humans do.

// confirmPlan creates a care_animal_plans row on a fixture animal. The
// fixture cleanup removes it (cage-keyed join).
func confirmPlan(t *testing.T, f *planFixture, animalID int, name string, flagged bool) *models.CareAnimalPlan {
	t.Helper()
	p := &models.CareAnimalPlan{
		ID:                uuid.Must(uuid.NewV4()),
		AnimalID:          animalID,
		Name:              name,
		ActionKind:        "feeding",
		ActionPayload:     json.RawMessage(`{"food":"crickets"}`),
		Schedule:          careScheduleJSON(t, itemDueSoon(time.Now())),
		NeedsConfirmation: flagged,
		Active:            true,
	}
	require.NoError(t, models.DB.Create(p))
	return p
}

func TestCarePlanConfirmationsToggle(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)

	p := confirmPlan(t, f, f.animalIDs[0], "CTOG-"+f.marker, false)

	// Bare toggle: flag goes on.
	code, body := planDoJSON(t, client, baseURL, "POST",
		"/care_animal_plans/"+p.ID.String()+"/confirmation", token, nil)
	require.Equal(t, http.StatusOK, code, "toggle on: %s", body)
	var out models.CareAnimalPlan
	require.NoError(t, json.Unmarshal(body, &out))
	require.True(t, out.NeedsConfirmation, "flag must be ON after bare toggle")
	require.True(t, models.DB.Reload(p) == nil)
	require.True(t, p.NeedsConfirmation)

	// Explicit needs_confirmation=0 resolves (the report's Resolve action).
	code, body = planDoJSON(t, client, baseURL, "POST",
		"/care_animal_plans/"+p.ID.String()+"/confirmation?needs_confirmation=0", token, nil)
	require.Equal(t, http.StatusOK, code, "resolve: %s", body)
	require.NoError(t, models.DB.Reload(p))
	require.False(t, p.NeedsConfirmation)

	// Explicit needs_confirmation=1 re-flags; unknown value falls back to
	// the plain toggle (current false → true).
	code, _ = planDoJSON(t, client, baseURL, "POST",
		"/care_animal_plans/"+p.ID.String()+"/confirmation?needs_confirmation=1", token, nil)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, models.DB.Reload(p))
	require.True(t, p.NeedsConfirmation)
	code, _ = planDoJSON(t, client, baseURL, "POST",
		"/care_animal_plans/"+p.ID.String()+"/confirmation?needs_confirmation=bogus", token, nil)
	require.Equal(t, http.StatusOK, code)
	require.NoError(t, models.DB.Reload(p))
	require.False(t, p.NeedsConfirmation, "bogus param falls back to toggle")

	// Unknown plan → 404.
	code, _ = planDoJSON(t, client, baseURL, "POST",
		"/care_animal_plans/"+uuid.Must(uuid.NewV4()).String()+"/confirmation", token, nil)
	require.Equal(t, http.StatusNotFound, code)

	// A restricted (non-admin but unrestricted) caretaker may flag too.
	regClient, regURL := planRegularClient(t)
	regToken := planToken(t, regClient, regURL)
	code, body = planDoJSON(t, regClient, regURL, "POST",
		"/care_animal_plans/"+p.ID.String()+"/confirmation?needs_confirmation=1", regToken, nil)
	require.Equal(t, http.StatusOK, code, "caretaker flag: %s", body)
	require.NoError(t, models.DB.Reload(p))
	require.True(t, p.NeedsConfirmation)
}

func TestCarePlanConfirmationsToggleRedirect(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	token := planToken(t, client, baseURL)
	p := confirmPlan(t, f, f.animalIDs[0], "CREDIR-"+f.marker, false)

	// HTML client with a local back target → 303 to it.
	form := url.Values{}
	form.Set("authenticity_token", token)
	form.Set("needs_confirmation", "1")
	form.Set("back", "/care_plan?kind=feeding")
	req, err := http.NewRequest("POST", baseURL+"/care_animal_plans/"+p.ID.String()+"/confirmation", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/care_plan?kind=feeding", resp.Header.Get("Location"))

	// BUG-R5: an external back target must not survive sanitisation.
	require.NoError(t, models.DB.Reload(p))
	require.True(t, p.NeedsConfirmation)
	form = url.Values{}
	form.Set("authenticity_token", token)
	form.Set("needs_confirmation", "0")
	form.Set("back", "https://evil.example/phish")
	req, err = http.NewRequest("POST", baseURL+"/care_animal_plans/"+p.ID.String()+"/confirmation", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	resp, err = client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/reports/confirmations", resp.Header.Get("Location"), "external back target must fall back to the report")
}

func TestCarePlanConfirmationsReport(t *testing.T) {
	f := setupPlanFixture(t)
	admin, baseURL := planAdminClient(t)

	flagA := confirmPlan(t, f, f.animalIDs[0], "CREP-A-"+f.marker, true)
	flagB := confirmPlan(t, f, f.animalIDs[1], "CREP-B-"+f.marker, true)
	quiet := confirmPlan(t, f, f.animalIDs[0], "CREP-Q-"+f.marker, false)

	// Admin sees the flagged plans grouped by animal; the unflagged one
	// must not appear.
	req, err := http.NewRequest("GET", baseURL+"/reports/confirmations", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := admin.Do(req)
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "report: %.200s", raw)
	html := string(raw)
	require.Contains(t, html, flagA.Name)
	require.Contains(t, html, flagB.Name)
	require.NotContains(t, html, quiet.Name, "unflagged plans stay off the report")
	require.Contains(t, html, "Resolve")

	// The non-admin (unrestricted caretaker) gets 403 on the admin report.
	regClient, regURL := planRegularClient(t)
	req, err = http.NewRequest("GET", regURL+"/reports/confirmations", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	resp, err = regClient.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	// Resolving removes the row from the report (buildConfirmationsView).
	tx := models.DB
	plans := &models.CareAnimalPlans{}
	require.NoError(t, tx.Where("needs_confirmation = ?", true).All(plans))
	view, err := buildConfirmationsView(tx, plans)
	require.NoError(t, err)
	require.False(t, view.Empty())
	require.NoError(t, tx.RawQuery(
		"UPDATE care_animal_plans SET needs_confirmation = ? WHERE id IN (?, ?)",
		false, flagA.ID, flagB.ID).Exec())
	plans = &models.CareAnimalPlans{}
	require.NoError(t, tx.Where("needs_confirmation = ?", true).All(plans))
	view, err = buildConfirmationsView(tx, plans)
	require.NoError(t, err)
	require.True(t, view.Empty())
}

// TestFeedingPlanSummariesNeedsReview covers the NeedsReview switch the
// animal page's feeding summary renders from the MANUAL flag.
func TestFeedingPlanSummariesNeedsReview(t *testing.T) {
	flagged := models.CareAnimalPlan{
		ActionKind:        "feeding",
		ActionPayload:     json.RawMessage(`{"food":"mealworms"}`),
		Schedule:          json.RawMessage(`{"times":["07:00"],"weekdays":[0,1,2,3,4,5,6]}`),
		NeedsConfirmation: true,
		Active:            true,
	}
	clean := models.CareAnimalPlan{
		ActionKind:    "feeding",
		ActionPayload: json.RawMessage(`{"food":"crickets"}`),
		Schedule:      json.RawMessage(`{"times":["19:00"],"weekdays":[0,1,2,3,4,5,6]}`),
		Active:        true,
	}
	inactive := flagged
	inactive.Active = false

	plans := models.CareAnimalPlans{flagged, clean, inactive}
	got := feedingPlanSummaries(&plans)
	require.Len(t, got, 2, "inactive plans are skipped")
	require.True(t, got[0].NeedsReview)
	require.Equal(t, "mealworms", got[0].Diet)
	require.Equal(t, "07:00", got[0].TimesLabel)
	require.False(t, got[1].NeedsReview)

	// A flagged NON-feeding plan never reaches the feeding summary.
	wrong := flagged
	wrong.ActionKind = "medication"
	plans = models.CareAnimalPlans{wrong}
	require.Empty(t, feedingPlanSummaries(&plans))
}
