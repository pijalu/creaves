package actions

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Treatment-time entry toggle endpoint tests (bugs.md R5-3e, U25/D-e):
// auth (401), unknown ids (404), pending→done→pending round trip, sibling
// entries untouched, Timedonebitmap never written (dormant).

func toggleEntryFixture(t *testing.T) (*planFixture, *models.Treatment, models.TreatmentTimeEntries) {
	t.Helper()
	f := setupPlanFixture(t)

	tr := &models.Treatment{
		Date:       time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local),
		AnimalID:   f.animalIDs[0],
		Drug:       "ToggleDrug-" + f.marker,
		Dosage:     "1 ml",
		Timebitmap: models.Treatement_MORNING,
	}
	require.NoError(t, models.DB.Create(tr))

	entries := models.TreatmentTimeEntries{
		{TreatmentID: tr.ID, AnimalID: tr.AnimalID, DueAt: tr.Date.Add(8 * time.Hour), TimeLabel: "08:00", Status: models.TreatmentEntryStatusPending, Source: models.TreatmentEntrySourceManual},
		{TreatmentID: tr.ID, AnimalID: tr.AnimalID, DueAt: tr.Date.Add(12 * time.Hour), TimeLabel: "12:00", Status: models.TreatmentEntryStatusPending, Source: models.TreatmentEntrySourceManual},
	}
	for i := range entries {
		require.NoError(t, models.DB.Create(&entries[i]))
	}
	return f, tr, entries
}

func reloadToggleEntries(t *testing.T, treatmentID uuid.UUID) models.TreatmentTimeEntries {
	t.Helper()
	entries := models.TreatmentTimeEntries{}
	require.NoError(t, models.DB.Where("treatment_id = ?", treatmentID).All(&entries))
	entries.SortByTime()
	return entries
}

func TestToggleEntryUnauthorized(t *testing.T) {
	_, tr, entries := toggleEntryFixture(t)

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	// Anonymous session: CSRF first (global middleware), then the Authorize
	// gate bounces the request to the login form. The handler's own 401 is
	// defense in depth (unreachable behind Authorize).
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

	req, err := http.NewRequest("POST", srv.URL+"/treatments/"+tr.ID.String()+"/entries/"+entries[0].ID.String()+"/toggle", nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-CSRF-Token", string(m[1]))
	resp, err = client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "unauthenticated toggle bounces to login")

	// The flip must NOT have happened.
	first := reloadToggleEntries(t, tr.ID).FindByLabel("08:00")
	require.NotNil(t, first)
	require.Equal(t, models.TreatmentEntryStatusPending, first.Status)
}

func TestToggleEntryNotFound(t *testing.T) {
	f, tr, entries := toggleEntryFixture(t)
	client, baseURL := planRegularClient(t)
	token := planToken(t, client, baseURL)

	// Unknown treatment id (valid v4 uuid, no row).
	code, raw := planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+uuid.Must(uuid.NewV4()).String()+"/entries/"+entries[0].ID.String()+"/toggle", token, nil)
	require.Equal(t, http.StatusNotFound, code, "body: %s", raw)

	// Known treatment, unknown entry id.
	code, raw = planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+tr.ID.String()+"/entries/"+uuid.Must(uuid.NewV4()).String()+"/toggle", token, nil)
	require.Equal(t, http.StatusNotFound, code, "body: %s", raw)

	// Entry belonging to another treatment must not be reachable through
	// this treatment's path.
	other := &models.Treatment{Date: tr.Date, AnimalID: f.animalIDs[1], Drug: "Other-" + f.marker}
	require.NoError(t, models.DB.Create(other))
	code, raw = planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+other.ID.String()+"/entries/"+entries[0].ID.String()+"/toggle", token, nil)
	require.Equal(t, http.StatusNotFound, code, "body: %s", raw)
}

func TestToggleEntryRoundTrip(t *testing.T) {
	f, tr, entries := toggleEntryFixture(t)
	client, baseURL := planRegularClient(t)
	token := planToken(t, client, baseURL)

	bitmapBefore := tr.Timedonebitmap

	// pending -> done: response carries the flipped entry, DB confirms the
	// apply stamp (click time + user), the sibling entry stays untouched
	// and the bitmap column is never written (D-e dormant).
	code, raw := planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+tr.ID.String()+"/entries/"+entries[0].ID.String()+"/toggle", token, nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.Contains(t, string(raw), `"status":"done"`)

	all := reloadToggleEntries(t, tr.ID)
	require.Len(t, all, 2)
	first := all.FindByLabel("08:00")
	require.NotNil(t, first)
	require.Equal(t, models.TreatmentEntryStatusDone, first.Status)
	require.True(t, first.AppliedAt.Valid)
	require.True(t, first.UserID.Valid)
	second := all.FindByLabel("12:00")
	require.NotNil(t, second)
	require.Equal(t, models.TreatmentEntryStatusPending, second.Status, "toggling one entry leaves the sibling pending")
	require.False(t, second.AppliedAt.Valid)

	fresh := &models.Treatment{}
	require.NoError(t, models.DB.Find(fresh, tr.ID))
	require.Equal(t, bitmapBefore, fresh.Timedonebitmap, "Timedonebitmap dormant: toggle never writes it")
	require.Zero(t, fresh.Timedonebitmap)

	// done -> pending: the apply stamp is cleared, application link too.
	code, raw = planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+tr.ID.String()+"/entries/"+entries[0].ID.String()+"/toggle", token, nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.Contains(t, string(raw), `"status":"pending"`)

	all = reloadToggleEntries(t, tr.ID)
	first = all.FindByLabel("08:00")
	require.NotNil(t, first)
	require.Equal(t, models.TreatmentEntryStatusPending, first.Status)
	require.False(t, first.AppliedAt.Valid)
	require.False(t, first.UserID.Valid)
	require.False(t, first.ApplicationID.Valid, "revert clears the application link")

	// A done entry with an application link reverts to a clean pending.
	app := &models.CarePlanApplication{
		SourceType:      models.ApplicationSourceRule,
		SourceID:        uuid.Must(uuid.NewV4()),
		SourceSnapshot:  []byte(`{}`),
		AnimalID:        f.animalIDs[0],
		DueAt:           time.Now(),
		AppliedAt:       time.Now(),
		UserID:          f.userID,
		FulfillmentType: models.ApplicationFulfillmentTreatment,
		FulfillmentID:   first.ID.String(),
		Status:          models.ApplicationStatusApplied,
	}
	require.NoError(t, models.DB.Create(app))
	require.NoError(t, models.DB.Where("treatment_id = ? AND time_label = ?", tr.ID, "08:00").First(first))
	first.Status = models.TreatmentEntryStatusDone
	first.ApplicationID = nulls.NewUUID(app.ID)
	require.NoError(t, models.DB.Update(first))

	code, raw = planDoJSON(t, client, baseURL, "POST",
		"/treatments/"+tr.ID.String()+"/entries/"+first.ID.String()+"/toggle", token, nil)
	require.Equal(t, http.StatusOK, code, "body: %s", raw)

	first = reloadToggleEntries(t, tr.ID).FindByLabel("08:00")
	require.NotNil(t, first)
	require.Equal(t, models.TreatmentEntryStatusPending, first.Status)
	require.False(t, first.ApplicationID.Valid, "revert clears the application link")
}

func TestToggleEntryHTMLRedirect(t *testing.T) {
	_, tr, entries := toggleEntryFixture(t)
	client, baseURL := planRegularClient(t)
	token := planToken(t, client, baseURL)

	// HTML navigation (the treatment-page button posts form-encoded and
	// follows the redirect): 303 back to the treatment page.
	form := url.Values{"authenticity_token": {token}}
	req, err := http.NewRequest("POST", baseURL+"/treatments/"+tr.ID.String()+"/entries/"+entries[0].ID.String()+"/toggle", strings.NewReader(form.Encode()))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/treatments/"+tr.ID.String(), resp.Header.Get("Location"))
}

func TestAnimalAndTreatmentAppliedTimesUseBrowserLocaleInAllForks(t *testing.T) {
	for _, path := range []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
	} {
		raw := readTemplate(t, path)
		require.Contains(t, raw, `data-applied-at="<%= entry.AppliedAt.Time.Format("2006-01-02T15:04:05Z07:00") %>"`, path)
		require.Contains(t, raw, `data-applied-at="<%= prow.AppliedAt %>"`, path)
		require.Contains(t, raw, `toLocaleTimeString(undefined, {hour: '2-digit', minute: '2-digit'})`, path)
		require.NotContains(t, raw, `entry.AppliedAt.Time.Format("15:04")`, path)
	}
	for _, path := range []string{
		"../templates/treatments/show.plush.html",
		"../templates/treatments/show.plush.fr.html",
		"../templates/treatments/show.plush.de.html",
		"../templates/treatments/show.plush.nl.html",
	} {
		raw := readTemplate(t, path)
		require.Contains(t, raw, `entry.AppliedAt.Time.Format("2006-01-02T15:04:05Z07:00")`, path)
		require.Contains(t, raw, `toLocaleTimeString(undefined, {hour: '2-digit', minute: '2-digit'})`, path)
	}
}

// TestTreatmentsShowRendersEntryRows guards the R5-3c read path (bugs.md
// U25/D-e): the show page renders one row per expected time from the
// eager-loaded entries. Regression: a plush for-loop written as
// `for (entry in treatment.Entries)` (instead of `for (entry) in ...`)
// compiled fine and only blew up at render time — "entry: unknown
// identifier" — so the page must actually be rendered in a test.
func TestTreatmentsShowRendersEntryRows(t *testing.T) {
	_, tr, entries := toggleEntryFixture(t)
	entries[0].Status = models.TreatmentEntryStatusDone
	entries[0].AppliedAt = nulls.NewTime(time.Date(tr.Date.Year(), tr.Date.Month(), tr.Date.Day(), 12, 34, 0, 0, time.UTC))
	require.NoError(t, models.DB.Update(&entries[0]))
	client, baseURL := planAdminClient(t)

	req, err := http.NewRequest("GET", baseURL+"/treatments/"+tr.ID.String(), nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)

	require.NotContains(t, html, "unknown identifier", "plush render error page leaked")
	require.NotContains(t, html, "Error Trace", "debug error page leaked")
	for _, e := range entries {
		require.Contains(t, html, `data-entry-id="`+e.ID.String()+`"`, "missing entry row for %s", e.TimeLabel)
		require.Contains(t, html, "<strong>"+e.TimeLabel+"</strong>")
	}
	require.Contains(t, html, `class="js-local-time" datetime="`+time.Date(tr.Date.Year(), tr.Date.Month(), tr.Date.Day(), 12, 34, 0, 0, time.UTC).Format(time.RFC3339)+`"`)
	for _, fork := range []string{"../templates/treatments/show.plush.html", "../templates/treatments/show.plush.fr.html", "../templates/treatments/show.plush.de.html", "../templates/treatments/show.plush.nl.html"} {
		template := readTemplate(t, fork)
		require.Contains(t, template, `toLocaleTimeString(undefined, {hour: '2-digit', minute: '2-digit'})`, fork)
		require.Contains(t, template, `entry.AppliedAt.Time.Format("2006-01-02T15:04:05Z07:00")`, fork)
	}

	require.Contains(t, html, "entry-toggle", "per-entry toggle buttons missing")
	require.NotContains(t, html, "Schedule (Morning", "legacy 3-bucket schedule block must stay removed (R5-3c)")
}
