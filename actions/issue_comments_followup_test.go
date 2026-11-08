package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// Regression coverage for the follow-up review comments on issues #208 and
// #209. These tests use the real application routes, templates and MySQL test
// database so they exercise the paths on which the comments reported failures.

func issueCommentGetLang(t *testing.T, client *http.Client, baseURL, path, lang string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	require.NoError(t, err)
	if lang != "" {
		req.AddCookie(&http.Cookie{Name: "lang", Value: lang})
	}
	resp, err := client.Do(req)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

func requireIssueCommentAccountRole(t *testing.T, body, fieldLabel, expected string) {
	t.Helper()
	pattern := `(?s)<label[^>]*>\s*` + regexp.QuoteMeta(fieldLabel) + `\s*</label>\s*<p[^>]*>\s*` + regexp.QuoteMeta(expected) + `\s*</p>`
	require.Regexp(t, pattern, body, "account-role field must show the localized full account role")
}

func TestIssue209PlainAdminCanManageActiveInstanceIdentity(t *testing.T) {
	tx := searchTestDB(t)
	cfg := seedConfig(t, "issue209-admin", true)
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM config WHERE id = ?", cfg.ID).Exec()
	})
	saved := CurrentConfigGet()
	CurrentConfigSet(cfg)
	t.Cleanup(func() { CurrentConfigSet(saved) })

	client, baseURL := adminClientWithURL(t)

	// The System-menu shortcut and its redirect target must both be available
	// to a plain admin, not only to a maintainer.
	resp, err := client.Get(baseURL + "/instance")
	require.NoError(t, err)
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	location := resp.Header.Get("Location")
	require.Equal(t, "/config/"+cfg.ID.String()+"/edit", location)

	status, body := roleTestGetBody(t, client, baseURL, location)
	require.Equal(t, http.StatusOK, status, body)
	require.Contains(t, body, `name="form_scope" value="identity"`)

	status, body = roleTestGetBody(t, client, baseURL, "/")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, `href="/instance"`)
	require.NotContains(t, body, `href="/config"`)

	// Identity edits remain available, including the post-update destination.
	resp = roleTestPostForm(t, client, baseURL, location, "/config/"+cfg.ID.String(), url.Values{
		"_method":             {"PUT"},
		"form_scope":          {"identity"},
		"Name":                {cfg.Name},
		"Description":         {cfg.Description},
		"Active":              {"true"},
		"InstanceID":          {"identity-must-not-change"},
		"Settings.CenterName": {"Issue 209 admin center"},
	})
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode)
	require.Equal(t, "/config/"+cfg.ID.String()+"/edit", resp.Header.Get("Location"))

	reloaded := reloadConfig(t, cfg.ID)
	require.Equal(t, cfg.InstanceID, reloaded.InstanceID)
	settings, err := reloaded.GetSettings()
	require.NoError(t, err)
	require.Equal(t, "Issue 209 admin center", settings.CenterName)
	require.True(t, settings.EnableEventStream)

	// Granting identity access must not open the maintainer-only configuration
	// list or synchronization settings through a crafted form scope.
	resp = roleTestPostForm(t, client, baseURL, location, "/config/"+cfg.ID.String(), url.Values{
		"_method":                    {"PUT"},
		"form_scope":                 {"sync"},
		"InstanceID":                 {"sync-must-stay-maintainer-only"},
		"Settings.EnableEventStream": {"false"},
	})
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)

	status, _ = roleTestGetBody(t, client, baseURL, "/config")
	require.Equal(t, http.StatusForbidden, status)
	status, _ = roleTestGetBody(t, client, baseURL, "/sync_configuration")
	require.Equal(t, http.StatusForbidden, status)

	inactive := seedConfig(t, "issue209-inactive", false)
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM config WHERE id = ?", inactive.ID).Exec()
	})
	status, _ = roleTestGetBody(t, client, baseURL, "/config/"+inactive.ID.String()+"/edit")
	require.Equal(t, http.StatusForbidden, status)
	resp = roleTestPostForm(t, client, baseURL, location, "/config/"+inactive.ID.String(), url.Values{
		"_method":    {"PUT"},
		"form_scope": {"identity"},
		"Name":       {"must-not-update"},
	})
	resp.Body.Close()
	require.Equal(t, http.StatusForbidden, resp.StatusCode)
}

func TestIssue209NoTreatmentRolesPersistAndDisplay(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	client, baseURL := roleTestLogin(t, admin)

	for _, tc := range []struct {
		name     string
		selector string
		shared   bool
		label    string
	}{
		{name: "user", selector: models.UserRoleNoTreatments, label: "Utilisateur (sans traitements)"},
		{name: "shared", selector: "shared_sans_treatments", shared: true, label: "Compte partagé (sans traitements)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			target := roleTestUser(t, models.UserRoleRegular, false)
			resp := roleTestPostForm(t, client, baseURL,
				"/users/"+target.ID.String()+"/edit",
				"/users/"+target.ID.String(),
				url.Values{
					"_method":     {"PUT"},
					"Login":       {target.Login},
					"AccountRole": {tc.selector},
				})
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.Equal(t, http.StatusSeeOther, resp.StatusCode, string(body))

			var stored models.User
			require.NoError(t, models.DB.Find(&stored, target.ID))
			require.Equal(t, models.UserRoleNoTreatments, stored.Role)
			require.Equal(t, tc.shared, stored.Shared)
			require.False(t, stored.Admin)

			status, page := issueCommentGetLang(t, client, baseURL, "/users/"+target.ID.String(), "fr")
			require.Equal(t, http.StatusOK, status)
			requireIssueCommentAccountRole(t, page, "Rôle du compte", tc.label)
		})
	}
}

func TestIssue209UserShowDisplaysLocalizedFlagRoles(t *testing.T) {
	actor := roleTestUser(t, models.UserRoleRegular, true)
	adminTarget := roleTestUser(t, models.UserRoleRegular, true)
	maintainerTarget := roleTestUser(t, models.UserRoleRegular, true)
	maintainerTarget.Maintainer = true
	require.NoError(t, models.DB.Update(maintainerTarget))
	sharedTarget := roleTestUser(t, models.UserRoleRegular, false)
	sharedTarget.Shared = true
	require.NoError(t, models.DB.Update(sharedTarget))

	client, baseURL := roleTestLogin(t, actor)
	for _, tc := range []struct {
		lang              string
		fieldLabel        string
		admin, maintainer string
		shared            string
	}{
		{lang: "en-US", fieldLabel: "Account role", admin: "Admin", maintainer: "Maintainer", shared: "Shared account"},
		{lang: "fr", fieldLabel: "Rôle du compte", admin: "Administrateur", maintainer: "Mainteneur", shared: "Compte partagé"},
		{lang: "de", fieldLabel: "Kontorolle", admin: "Administrator", maintainer: "Betreuer", shared: "Gemeinsames Konto"},
		{lang: "nl", fieldLabel: "Accountrol", admin: "Beheerder", maintainer: "Onderhouder", shared: "Gedeeld account"},
	} {
		t.Run(tc.lang, func(t *testing.T) {
			for _, roleCase := range []struct {
				name  string
				user  *models.User
				label string
			}{
				{name: "admin", user: adminTarget, label: tc.admin},
				{name: "maintainer", user: maintainerTarget, label: tc.maintainer},
				{name: "shared", user: sharedTarget, label: tc.shared},
			} {
				t.Run(roleCase.name, func(t *testing.T) {
					status, body := issueCommentGetLang(t, client, baseURL, "/users/"+roleCase.user.ID.String(), tc.lang)
					require.Equal(t, http.StatusOK, status)
					requireIssueCommentAccountRole(t, body, tc.fieldLabel, roleCase.label)
				})
			}
		})
	}
}

func TestIssue208ExportChooserHasExcelColumnNextToCSV(t *testing.T) {
	client, baseURL := adminClientWithURL(t)
	status, body := roleTestGetBody(t, client, baseURL, "/export/view")
	require.Equal(t, http.StatusOK, status)

	csvHeading := strings.Index(body, ">CSV</th>")
	excelHeading := strings.Index(body, ">Excel</th>")
	require.GreaterOrEqual(t, csvHeading, 0, "CSV column missing")
	require.Greater(t, excelHeading, csvHeading, "Excel column must follow CSV")
	require.Contains(t, body, `class="btn btn-secondary btn-sm export-link" data-base="/export/excel?query=register"`)
}

func TestIssue208PhonePlaceholdersMatchInEveryLocale(t *testing.T) {
	client, baseURL := adminClientWithURL(t)
	for _, lang := range []string{"en-US", "fr", "de", "nl"} {
		t.Run(lang, func(t *testing.T) {
			status, guest := issueCommentGetLang(t, client, baseURL, "/guest/?lang="+url.QueryEscape(lang), "")
			require.Equal(t, http.StatusOK, status)
			require.Contains(t, guest, `placeholder="0470112233"`)
			require.NotContains(t, guest, `placeholder="06 12 34 56 78"`)

			status, reception := issueCommentGetLang(t, client, baseURL, "/reception/new", lang)
			require.Equal(t, http.StatusOK, status)
			require.Contains(t, reception, `placeholder="0470112233"`)
			require.NotContains(t, reception, `placeholder="+32470112233"`)
		})
	}
}

func TestIssue208NewCarePrefillsInitialDefaultNote(t *testing.T) {
	tx := searchTestDB(t)
	fx := createAnimalSearchFixtures(t, tx)
	animal := &models.Animal{}
	require.NoError(t, tx.Find(animal, fx.animalC))

	login, password := testUserFixture(t, true)
	owner := &models.User{}
	require.NoError(t, tx.Where("login = ?", login).First(owner))

	marker := uuid.Must(uuid.NewV4()).String()[:8]
	careType := &models.Caretype{ID: uuid.Must(uuid.NewV4()), Name: "TS-initial-default-" + marker, Type: 97}
	require.NoError(t, tx.Create(careType))
	defaultNote := "Default initial note " + marker
	tpl := &models.CareTemplate{
		ID:         uuid.Must(uuid.NewV4()),
		UserID:     owner.ID,
		Name:       "TS-initial-default-" + marker,
		Content:    defaultNote,
		CaretypeID: nulls.NewUUID(careType.ID),
		IsDefault:  true,
	}
	require.NoError(t, tx.Create(tpl))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM care_templates WHERE id = ?", tpl.ID).Exec()
		tx.RawQuery("DELETE FROM caretypes WHERE id = ?", careType.ID).Exec()
	})

	client, baseURL := testUserLogin(t, login, password)
	path := fmt.Sprintf("/cares/new?animal_year_number=%s&careType=97", url.QueryEscape(animal.YearNumberFormatted()))
	status, body := roleTestGetBody(t, client, baseURL, path)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, careType.ID.String())
	require.Contains(t, body, defaultNote, "the initially selected care type must prefill its default note")

	// An explicit note supplied by the calling page still wins over the default.
	status, body = roleTestGetBody(t, client, baseURL, path+"&note=Explicit+note")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "Explicit note")
	require.NotContains(t, body, defaultNote)
}
