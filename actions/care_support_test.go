package actions

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Issue #158: heat source / oxygen on cares with automatic "Soin" care,
// previous-weight lookup for the ±10% warning, heat source suggestions and
// per-user care note templates.
// ---------------------------------------------------------------------------

// csTSoins ensures a "Soin" care type exists in the test DB and returns its
// id. When the canonical type is missing (fresh test DB) it is created and
// removed at cleanup.
func csTSoins(t *testing.T, tx *pop.Connection) uuid.UUID {
	t.Helper()
	ct := &models.Caretype{}
	err := tx.Where("LOWER(name) = ?", "soin").First(ct)
	if err == nil {
		return ct.ID
	}
	ct = &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "Soin"}
	require.NoError(t, tx.Create(ct), "create Soin caretype fixture")
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec()
	})
	return ct.ID
}

// csTCare creates a care row directly and fails the test on error.
func csTCare(t *testing.T, tx *pop.Connection, care *models.Care) {
	t.Helper()
	require.NoError(t, tx.Create(care), "care fixture creation")
}

func TestSupportCareNote158(t *testing.T) {
	tt := []struct {
		name    string
		heat    nulls.String
		oxygen  bool
		want    string
	}{
		{"heat only", nulls.NewString("Lampe chauffante"), false, "Source de chaleur : Lampe chauffante"},
		{"oxygen only", nulls.String{}, true, "O2"},
		{"both", nulls.NewString("Tapis"), true, "Source de chaleur : Tapis / O2"},
		{"blank heat + oxygen", nulls.NewString("  "), true, "O2"},
		{"nothing", nulls.String{}, false, ""},
	}
	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, supportCareNote(tc.heat, tc.oxygen))
		})
	}
}

func TestCareNeedsSupportCare158(t *testing.T) {
	require.False(t, careNeedsSupportCare(&models.Care{}), "plain care needs no support care")
	require.True(t, careNeedsSupportCare(&models.Care{HeatSource: nulls.NewString("Lampe")}))
	require.True(t, careNeedsSupportCare(&models.Care{Oxygen: true}))
	// blank heat source is not support
	require.False(t, careNeedsSupportCare(&models.Care{HeatSource: nulls.NewString(" ")}))
}

func TestPreviousCareWeight158(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	ct := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "TSPrevW-" + fx.marker}
	require.NoError(t, tx.Create(ct))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec() })

	// Wall-clock UTC base: previousCareWeight compares against stored
	// (driver-formatted) values, so the test speaks in UTC terms end to end.
	now := time.Now().UTC().Truncate(time.Second)
	c3 := &models.Care{ID: uuid.Must(uuid.NewV4()), Date: now.Add(-72 * time.Hour), AnimalID: fx.animalC, TypeID: ct.ID, Weight: nulls.NewString("100")}
	c2 := &models.Care{ID: uuid.Must(uuid.NewV4()), Date: now.Add(-48 * time.Hour), AnimalID: fx.animalC, TypeID: ct.ID, Weight: nulls.NewString("150,5")}
	cNote := &models.Care{ID: uuid.Must(uuid.NewV4()), Date: now.Add(-24 * time.Hour), AnimalID: fx.animalC, TypeID: ct.ID, Weight: nulls.NewString("")} // empty weight, must be ignored
	csTCare(t, tx, c3)
	csTCare(t, tx, c2)
	csTCare(t, tx, cNote)
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", fx.animalC).Exec()
	})

	// latest weighted care strictly before now
	got := previousCareWeight(tx, fx.animalC, now, nulls.UUID{})
	require.True(t, got.Valid)
	require.Equal(t, "150,5", got.String)

	// strictly before a past date skips later cares
	got = previousCareWeight(tx, fx.animalC, now.Add(-48*time.Hour), nulls.UUID{})
	require.True(t, got.Valid)
	require.Equal(t, "100", got.String)

	// excludeID drops the otherwise-matching care (edit case)
	got = previousCareWeight(tx, fx.animalC, now, nulls.NewUUID(c2.ID))
	require.True(t, got.Valid)
	require.Equal(t, "100", got.String)

	// animal without cares → invalid
	got = previousCareWeight(tx, fx.animalA, now, nulls.UUID{})
	require.False(t, got.Valid)
}

func TestCreateAutoSupportCare158(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	soinID := csTSoins(t, tx)

	ct := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "TSAutoS-" + fx.marker}
	require.NoError(t, tx.Create(ct))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", fx.animalC).Exec()
		tx.RawQuery("DELETE FROM caretypes WHERE id IN (?, ?)", ct.ID, soinID).Exec()
	})

	c := &buffalo.DefaultContext{}
	now := time.Now().Truncate(time.Second)

	care := &models.Care{
		ID:         uuid.Must(uuid.NewV4()),
		Date:       now,
		AnimalID:   fx.animalC,
		TypeID:     ct.ID,
		Weight:     nulls.NewString("180"),
		HeatSource: nulls.NewString("Lampe chauffante"),
		Oxygen:     true,
	}
	csTCare(t, tx, care)

	// the automatic "Soin" care is created with a canonical note
	createAutoSupportCare(c, tx, care)
	soin := &models.Care{}
	q := tx.Where("animal_id = ? AND type_id = ?", fx.animalC, soinID)
	require.NoError(t, q.First(soin), "expected an automatic Soin care")
	require.Equal(t, "Source de chaleur : Lampe chauffante / O2", soin.Note.String)
	require.WithinDuration(t, care.Date, soin.Date, time.Second)

	// re-running the same edit does not duplicate it (submission guard)
	createAutoSupportCare(c, tx, care)
	cnt, err := tx.Where("animal_id = ? AND type_id = ?", fx.animalC, soinID).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 1, cnt, "duplicate Soin care must be blocked")

	// a plain care without heat/oxygen never produces a Soin care
	plain := &models.Care{ID: uuid.Must(uuid.NewV4()), Date: now, AnimalID: fx.animalC, TypeID: ct.ID, Weight: nulls.NewString("181")}
	csTCare(t, tx, plain)
	createAutoSupportCare(c, tx, plain)
	cnt, err = tx.Where("animal_id = ? AND type_id = ?", fx.animalC, soinID).Count(&models.Care{})
	require.NoError(t, err)
	require.Equal(t, 1, cnt, "no Soin care for a plain care")
}

func TestSuggestionsHeatSource158(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)

	ct := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "TSHSug-" + fx.marker}
	require.NoError(t, tx.Create(ct))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM caretypes WHERE id = ?", ct.ID).Exec() })

	heat := "TSLampe-" + fx.marker
	care := &models.Care{
		ID:         uuid.Must(uuid.NewV4()),
		Date:       time.Now().Truncate(time.Second),
		AnimalID:   fx.animalC,
		TypeID:     ct.ID,
		HeatSource: nulls.NewString(heat),
	}
	csTCare(t, tx, care)
	t.Cleanup(func() { tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", fx.animalC).Exec() })

	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)
	resp, err := client.Get(baseURL + "/suggestions/heat_source?q=" + heat)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	var got []string
	require.NoError(t, json.Unmarshal(body, &got), "body: %s", body)
	require.Contains(t, got, heat)
}
// careTemplatesToken fetches a CSRF token for the current session from any
// authenticated page (the layout renders it in the csrf-token meta tag).
// Required because mw-csrf rejects form POSTs without authenticity_token.
func careTemplatesToken(t *testing.T, client *http.Client, baseURL, path string) string {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s for csrf token", path)
	m := csrfTokenRe.FindSubmatch(body)
	require.NotNil(t, m, "no csrf token on %s", path)
	return string(m[1])
}

// TestCareTemplatesAdminCRUD158 (bugs.md bug 6): note templates are an
// admin-managed shared pool. Admin HTML flow: list, create, 422 on invalid,
// edit, update, delete — the created template is owned by the admin.
func TestCareTemplatesAdminCRUD158(t *testing.T) {
	login, password := feedingGuideUser(t, true)
	client, baseURL := feedingGuideLogin(t, login, password)

	cu := &models.User{}
	require.NoError(t, models.DB.Where("login = ?", login).First(cu))

	token := careTemplatesToken(t, client, baseURL, "/care_templates")

	// create (form POST → 303 back to the list)
	resp, err := client.PostForm(baseURL+"/care_templates", url.Values{
		"Name":               {"TPL admin"},
		"Content":            {"Animal calme, à surveiller."},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	row := &models.CareTemplate{}
	require.NoError(t, models.DB.Where("name = ?", "TPL admin").First(row))
	require.Equal(t, cu.ID, row.UserID, "template must be owned by the creating admin")
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM care_templates WHERE id = ?", row.ID).Exec()
	})

	// list renders it with its owner
	resp, err = client.Get(baseURL + "/care_templates")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "TPL admin")
	require.Contains(t, string(body), login)

	// invalid create → 422 re-render
	resp, err = client.PostForm(baseURL+"/care_templates", url.Values{
		"Name":               {" "},
		"Content":            {""},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)

	// edit form renders the template
	resp, err = client.Get(baseURL+"/care_templates/"+row.ID.String()+"/edit")
	require.NoError(t, err)
	body, err = io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, string(body), "TPL admin")

	// update via _method=PUT (buffalo method override)
	resp, err = client.PostForm(baseURL+"/care_templates/"+row.ID.String(), url.Values{
		"_method":            {"PUT"},
		"Name":               {"TPL admin renamed"},
		"Content":            {"updated content"},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	updated := &models.CareTemplate{}
	require.NoError(t, models.DB.Find(updated, row.ID))
	require.Equal(t, "TPL admin renamed", updated.Name)
	require.Equal(t, "updated content", updated.Content)
	require.Equal(t, cu.ID, updated.UserID, "owner must be preserved on update")

	// delete via _method=DELETE
	resp, err = client.PostForm(baseURL+"/care_templates/"+row.ID.String(), url.Values{
		"_method":            {"DELETE"},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	cnt, err := models.DB.Where("id = ?", row.ID).Count(&models.CareTemplate{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "template must be deleted")
}

// TestCareTemplatesAdminOnly158 (bugs.md bug 6): non-admin users cannot manage
// templates (every management verb redirects home, data untouched) but the
// care-form suggestion endpoint serves them the full shared pool.
func TestCareTemplatesAdminOnly158(t *testing.T) {
	tx := searchTestDB(t)

	// a template owned by another user (the shared pool)
	owner := &models.User{ID: uuid.Must(uuid.NewV4()), Login: "TSCtOwner-" + uuid.Must(uuid.NewV4()).String()[:8], PasswordHash: "x"}
	require.NoError(t, tx.Create(owner))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM users WHERE id = ?", owner.ID).Exec() })

	tpl := &models.CareTemplate{ID: uuid.Must(uuid.NewV4()), UserID: owner.ID, Name: "SharedTpl-" + owner.Login, Content: "x"}
	require.NoError(t, tx.Create(tpl))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM care_templates WHERE id = ?", tpl.ID).Exec() })

	login, password := feedingGuideUser(t, false)
	client, baseURL := feedingGuideLogin(t, login, password)

	// remove stale rows from earlier failed runs so the create-assert is exact
	require.NoError(t, tx.RawQuery("DELETE FROM care_templates WHERE name = ?", "Hax").Exec())

	// non-admin has no care_templates page (redirects home); take the CSRF
	// token from the landing page of the same session instead.
	token := careTemplatesToken(t, client, baseURL, "/")

	// GET list → redirect home
	resp, err := client.Get(baseURL + "/care_templates")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)

	// POST create → redirect, nothing stored
	resp, err = client.PostForm(baseURL+"/care_templates", url.Values{
		"Name":               {"Hax"},
		"Content":            {"Hax"},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	cnt, err := tx.Where("name = ?", "Hax").Count(&models.CareTemplate{})
	require.NoError(t, err)
	require.Equal(t, 0, cnt, "non-admin must not create templates")

	// DELETE → redirect, template untouched
	resp, err = client.PostForm(baseURL+"/care_templates/"+tpl.ID.String(), url.Values{
		"_method":            {"DELETE"},
		"authenticity_token": {token},
	})
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	cnt, err = tx.Where("id = ?", tpl.ID).Count(&models.CareTemplate{})
	require.NoError(t, err)
	require.Equal(t, 1, cnt, "non-admin must not delete templates")

	// the care form suggestion endpoint exposes the shared pool
	resp, err = client.Get(baseURL + "/suggestions/care_templates")
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	var got []models.CareTemplate
	require.NoError(t, json.Unmarshal(body, &got), "body: %s", body)
	found := false
	for _, g := range got {
		if g.ID == tpl.ID {
			found = true
		}
	}
	require.True(t, found, "shared pool must contain other users' templates, body: %s", body)
}
