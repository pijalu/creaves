package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Issue #107 — account roles (lecteur / scientifique / SPW), user columns and
// users listing search/sort.
// ---------------------------------------------------------------------------

// roleTestSuffix returns a short unique suffix for fixture logins.
func roleTestSuffix() string {
	return uuid.Must(uuid.NewV4()).String()[:8]
}

// roleTestUser creates (and registers cleanup for) a user fixture with the
// given account role. Requires GO_ENV=test so models.DB points at creaves_test.
func roleTestUser(t *testing.T, role string, admin bool) *models.User {
	t.Helper()
	requireMySQLTestDB(t)
	if models.DB == nil {
		t.Fatal("models.DB is nil — run with GO_ENV=test")
	}

	sfx := roleTestSuffix()
	login := "roletest_" + sfx
	u := &models.User{
		Login:     login,
		Admin:     admin,
		Approved:  true,
		Role:      role,
		FirstName: "First-" + sfx,
		LastName:  "Last-" + sfx,
		City:      "TestCity",
		Email:     "roletest-" + sfx + "@example.com",
		Remark:    nulls.NewString(""),
	}
	u.Password = "rolepass123"
	u.PasswordConfirmation = "rolepass123"
	verrs, err := u.Create(models.DB)
	require.False(t, verrs.HasAny(), "user fixture validation: %v", verrs)
	require.NoError(t, err)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})
	return u
}

// roleTestLogin performs the login dance for a role fixture user and returns
// an authenticated client + base URL of a fresh app server.
func roleTestLogin(t *testing.T, u *models.User) (*http.Client, string) {
	t.Helper()
	return feedingGuideLogin(t, u.Login, "rolepass123")
}

// roleTestGetBody fetches a URL and returns status code + body.
func roleTestGetBody(t *testing.T, client *http.Client, baseURL, path string) (int, string) {
	t.Helper()
	resp, err := client.Get(baseURL + path)
	require.NoError(t, err)
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	require.NoError(t, err)
	return resp.StatusCode, string(body)
}

// ---------------------------------------------------------------------------
// Pure-function unit tests
// ---------------------------------------------------------------------------

func TestUserSortClauses(t *testing.T) {
	// default (and unknown keys): volunteer name asc, login tiebreak
	require.Equal(t,
		[]string{"first_name asc, last_name asc, login asc"},
		userSortClauses("", ""))
	require.Equal(t,
		[]string{"first_name asc, last_name asc, login asc"},
		userSortClauses("bogus; DROP TABLE users", "desc"))
	require.Equal(t,
		[]string{"login asc", "login asc"},
		userSortClauses("login", "asc"))
	require.Equal(t,
		[]string{"city desc", "login asc"},
		userSortClauses("city", "DESC"))
	require.Equal(t,
		[]string{"email asc", "login asc"},
		userSortClauses("email", "sideways"))
}

func TestUserRoleNameHelper(t *testing.T) {
	require.Equal(t, "Utilisateur", userRoleName(models.UserRoleRegular))
	require.Equal(t, "Utilisateur", userRoleName("totally-unknown"))
	require.Equal(t, "Lecteur", userRoleName(models.UserRoleReader))
	require.Equal(t, "Scientifique (U Liège, DEMNA)", userRoleName(models.UserRoleScientist))
	require.Equal(t, "SPW", userRoleName(models.UserRoleSPW))
}

func TestUserDisplayName(t *testing.T) {
	u := &models.User{Login: "bob"}
	require.Equal(t, "bob", u.DisplayName())
	u.FirstName = "Bob"
	u.LastName = "Marley"
	require.Equal(t, "Bob Marley", u.DisplayName())
}

// roleTestCtx is a minimal buffalo.Context stub: roleAllows only reads the
// current user from the context values.
type roleTestCtx struct {
	buffalo.Context
	user *models.User
}

func (c roleTestCtx) Value(key interface{}) interface{} {
	if key == "current_user" {
		return c.user
	}
	return nil
}

// TestRoleAllowsMatrix verifies the whitelist of the RoleGuard middleware per
// role. `self` cases need the request context to carry the user.
func TestRoleAllowsMatrix(t *testing.T) {
	selfID := uuid.Must(uuid.NewV4())
	otherID := uuid.Must(uuid.NewV4())

	tests := []struct {
		name   string
		role   string
		method string
		path   string
		self   bool
		want   bool
	}{
		// regular users pass everywhere
		{"regular get root", models.UserRoleRegular, "GET", "/", false, true},
		{"regular post animals", models.UserRoleRegular, "POST", "/animals", false, true},
		{"regular delete animal", models.UserRoleRegular, "DELETE", "/animals/1", false, true},

		// lecteur: GET everywhere + own account writes
		{"lecteur get root", models.UserRoleReader, "GET", "/", false, true},
		{"lecteur get animal", models.UserRoleReader, "GET", "/animals/42", false, true},
		{"lecteur get users", models.UserRoleReader, "GET", "/users", false, true},
		{"lecteur post animal", models.UserRoleReader, "POST", "/animals", false, false},
		{"lecteur put care", models.UserRoleReader, "PUT", "/cares/1", false, false},
		{"lecteur delete own", models.UserRoleReader, "DELETE", "/users/" + selfID.String(), true, false},
		{"lecteur put own", models.UserRoleReader, "PUT", "/users/" + selfID.String(), true, true},
		{"lecteur put other", models.UserRoleReader, "PUT", "/users/" + otherID.String(), false, false},
		{"lecteur post vetvisit", models.UserRoleReader, "POST", "/veterinaryvisits", false, false},

		// scientifique: GET everywhere + POST /veterinaryvisits + own account
		{"scientist get animal", models.UserRoleScientist, "GET", "/animals/42", false, true},
		{"scientist post vetvisit", models.UserRoleScientist, "POST", "/veterinaryvisits", false, true},
		{"scientist post vetvisit slash", models.UserRoleScientist, "POST", "/veterinaryvisits/", false, true},
		{"scientist put vetvisit", models.UserRoleScientist, "PUT", "/veterinaryvisits/1", false, false},
		{"scientist delete vetvisit", models.UserRoleScientist, "DELETE", "/veterinaryvisits/1", false, false},
		{"scientist put own", models.UserRoleScientist, "PUT", "/users/" + selfID.String(), true, true},
		{"scientist post users", models.UserRoleScientist, "POST", "/users", false, false},
		{"scientist delete animal", models.UserRoleScientist, "DELETE", "/animals/1", false, false},

		// SPW: GET on a fixed page set + own account writes
		{"spw get root", models.UserRoleSPW, "GET", "/", false, true},
		{"spw get dashboard", models.UserRoleSPW, "GET", "/dashboard", false, true},
		{"spw get animals", models.UserRoleSPW, "GET", "/animals", false, true},
		{"spw get animal", models.UserRoleSPW, "GET", "/animals/42", false, true},
		{"spw get reports", models.UserRoleSPW, "GET", "/reports", false, true},
		{"spw get reports sub", models.UserRoleSPW, "GET", "/reports/deaths", false, true},
		{"spw get users", models.UserRoleSPW, "GET", "/users", false, true},
		{"spw get user other", models.UserRoleSPW, "GET", "/users/" + otherID.String(), false, true},
		{"spw get attachment", models.UserRoleSPW, "GET", "/attachments/" + otherID.String(), false, true},
		{"spw get todos", models.UserRoleSPW, "GET", "/todos", false, true},
		{"spw post attachment delete", models.UserRoleSPW, "POST", "/attachments/" + otherID.String() + "/delete", false, false},
		{"spw post todos", models.UserRoleSPW, "POST", "/todos", false, false},
		{"spw get vetvisits", models.UserRoleSPW, "GET", "/veterinaryvisits", false, false},
		{"spw get cares", models.UserRoleSPW, "GET", "/cares", false, false},
		{"spw get reception", models.UserRoleSPW, "GET", "/reception/new", false, false},
		{"spw post users", models.UserRoleSPW, "POST", "/users", false, false},
		{"spw post vetvisit", models.UserRoleSPW, "POST", "/veterinaryvisits", false, false},
		{"spw put other", models.UserRoleSPW, "PUT", "/users/" + otherID.String(), false, false},
		{"spw put own", models.UserRoleSPW, "PUT", "/users/" + selfID.String(), true, true},
		{"spw get own edit", models.UserRoleSPW, "GET", "/users/" + selfID.String() + "/edit", true, true},
		{"spw delete own", models.UserRoleSPW, "DELETE", "/users/" + selfID.String(), true, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u := &models.User{ID: selfID, Login: "u", Role: tc.role}
			var ctx buffalo.Context = roleTestCtx{}
			if tc.self {
				ctx = roleTestCtx{user: u}
			}
			got := roleAllows(ctx, u, tc.method, tc.path)
			require.Equal(t, tc.want, got, "%s %s (self=%v)", tc.method, tc.path, tc.self)
		})
	}
}

// ---------------------------------------------------------------------------
// Handler tests (GO_ENV=test, MySQL creaves_test)
// ---------------------------------------------------------------------------

// TestUsersListAccessMatrix verifies who may open the users listing/details.
func TestUsersListAccessMatrix(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	regular := roleTestUser(t, models.UserRoleRegular, false)
	reader := roleTestUser(t, models.UserRoleReader, false)
	spw := roleTestUser(t, models.UserRoleSPW, false)

	// admin: full access
	client, baseURL := roleTestLogin(t, admin)
	code, _ := roleTestGetBody(t, client, baseURL, "/users")
	require.Equal(t, http.StatusOK, code, "admin GET /users")

	// spw: read-only listing + details, no edit of others
	client, baseURL = roleTestLogin(t, spw)
	code, _ = roleTestGetBody(t, client, baseURL, "/users")
	require.Equal(t, http.StatusOK, code, "spw GET /users")
	code, _ = roleTestGetBody(t, client, baseURL, "/users/"+admin.ID.String())
	require.Equal(t, http.StatusOK, code, "spw GET /users/{admin}")
	code, _ = roleTestGetBody(t, client, baseURL, "/users/"+admin.ID.String()+"/edit")
	require.Equal(t, http.StatusForbidden, code, "spw GET /users/{admin}/edit")
	code, _ = roleTestGetBody(t, client, baseURL, "/users/"+spw.ID.String()+"/edit")
	require.Equal(t, http.StatusOK, code, "spw GET own /users/edit")

	// regular + lecteur: no users listing
	for _, u := range []*models.User{regular, reader} {
		client, baseURL = roleTestLogin(t, u)
		code, _ = roleTestGetBody(t, client, baseURL, "/users")
		require.Equal(t, http.StatusForbidden, code, "%s GET /users", u.Login)
	}
}

// TestUsersListSearchSort verifies the `q` filter and `sort`/`dir` of the
// users listing (as admin).
func TestUsersListSearchSort(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	client, baseURL := roleTestLogin(t, admin)

	type fixture struct {
		login string
		first string
		city  string
	}
	mk := func(first, last, city string) fixture {
		u := roleTestUser(t, models.UserRoleRegular, false)
		u.FirstName = first
		u.LastName = last
		u.City = city
		require.NoError(t, models.DB.Update(u))
		return fixture{login: u.Login, first: first, city: city}
	}
	ua := mk("Alice", "Alpha", "Namur")
	ub := mk("Bob", "Beta", "Liège")
	uc := mk("Celine", "Gamma", "Namur")

	pos := func(body, login string) int {
		return strings.Index(body, login)
	}

	// default order: first_name asc → Alice, Bob, Celine
	// per_page keeps all fixture rows on page 1 regardless of how many
	// users accumulate in the shared test database.
	code, body := roleTestGetBody(t, client, baseURL, "/users?per_page=100")
	require.Equal(t, http.StatusOK, code)
	pa, pb, pc := pos(body, ua.login), pos(body, ub.login), pos(body, uc.login)
	require.GreaterOrEqual(t, pa, 0)
	require.Greater(t, pb, pa, "Bob after Alice (default name asc)")
	require.Greater(t, pc, pb, "Celine after Bob (default name asc)")

	// explicit desc reverses
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&sort=name&dir=desc")
	require.Equal(t, http.StatusOK, code)
	pa, pb, pc = pos(body, ua.login), pos(body, ub.login), pos(body, uc.login)
	require.Greater(t, pa, pb, "Alice after Bob (name desc)")
	require.Greater(t, pb, pc, "Bob after Celine (name desc)")

	// sort by city: Liège < Namur
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&sort=city")
	require.Equal(t, http.StatusOK, code)
	pb = pos(body, ub.login)
	pn := pos(body, ua.login)
	require.Greater(t, pn, pb, "Namur rows after Liège row (city asc)")

	// search by last name matches only Beta
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&q=Beta")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, ub.login), 0, "Beta found")
	require.Equal(t, -1, pos(body, ua.login), "Alpha not in search results")
	require.Equal(t, -1, pos(body, uc.login), "Gamma not in search results")

	// search by city matches both Namur rows
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&q=Namur")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, ua.login), 0)
	require.GreaterOrEqual(t, pos(body, uc.login), 0)
	require.Equal(t, -1, pos(body, ub.login))
}

// roleTestPostForm issues a form POST with the CSRF token scraped from a page.
func roleTestPostForm(t *testing.T, client *http.Client, baseURL, tokenPath, postPath string, vals url.Values) *http.Response {
	t.Helper()
	code, body := roleTestGetBody(t, client, baseURL, tokenPath)
	require.Equal(t, http.StatusOK, code, "GET %s for csrf", tokenPath)
	m := csrfTokenRe.FindSubmatch([]byte(body))
	require.NotNil(t, m, "no authenticity_token on %s", tokenPath)
	vals.Set("authenticity_token", string(m[1]))
	resp, err := client.PostForm(baseURL+postPath, vals)
	require.NoError(t, err)
	return resp
}

// TestUserFieldPermissionsSelfUpdate verifies a restricted (and a regular)
// user editing their own account: contact fields change, admin-only columns
// (role, volunteer flags, remark) are restored.
func TestUserFieldPermissionsSelfUpdate(t *testing.T) {
	u := roleTestUser(t, models.UserRoleReader, false)
	u.FosterFamily = true
	u.Remark = nulls.NewString("keepme")
	require.NoError(t, models.DB.Update(u))

	client, baseURL := roleTestLogin(t, u)
	resp := roleTestPostForm(t, client, baseURL,
		"/users/"+u.ID.String()+"/edit",
		"/users/"+u.ID.String(),
		url.Values{
			"_method":      {"PUT"},
			"Login":        {u.Login},
			"FirstName":    {"Self"},
			"LastName":     {"Edited"},
			"City":         {"SelfCity"},
			"Role":         {"spw"},
			"FosterFamily": {"true"},
			"Remark":       {"hacked"},
		})
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "self update should redirect")

	var got models.User
	require.NoError(t, models.DB.Find(&got, u.ID.String()))
	require.Equal(t, "Self", got.FirstName, "contact field updated")
	require.Equal(t, "Edited", got.LastName, "contact field updated")
	require.Equal(t, "SelfCity", got.City, "contact field updated")
	require.Equal(t, models.UserRoleReader, got.Role, "role restored (admin-only)")
	require.True(t, got.FosterFamily, "flag restored (admin-only)")
	require.Equal(t, "keepme", got.Remark.String, "remark restored (admin-only)")
}

// TestUserFieldPermissionsAdminUpdate verifies admins may change the
// admin-only columns of another user.
func TestUserFieldPermissionsAdminUpdate(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	u := roleTestUser(t, models.UserRoleRegular, false)

	client, baseURL := roleTestLogin(t, admin)
	resp := roleTestPostForm(t, client, baseURL,
		"/users/"+u.ID.String()+"/edit",
		"/users/"+u.ID.String(),
		url.Values{
			"_method":      {"PUT"},
			"Login":        {u.Login},
			"FirstName":    {"Managed"},
			"Role":         {"lecteur"},
			"FosterFamily": {"true"},
			"Remark":       {"volunteer"},
		})
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "admin update should redirect")

	var got models.User
	require.NoError(t, models.DB.Find(&got, u.ID.String()))
	require.Equal(t, "Managed", got.FirstName)
	require.Equal(t, models.UserRoleReader, got.Role, "admin may set role")
	require.True(t, got.FosterFamily, "admin may set flags")
	require.Equal(t, "volunteer", got.Remark.String, "admin may set remark")
}

// TestRegistrationMassAssignmentGuard verifies public registration cannot
// self-assign the account role, volunteer flags or remark.
func TestRegistrationMassAssignmentGuard(t *testing.T) {
	sfx := roleTestSuffix()
	login := "regtest_" + sfx
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})

	srv := httptest.NewServer(App())
	t.Cleanup(srv.Close)
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := srv.Client()
	client.Jar = jar
	// Keep redirects unfollowed so the 303 of the registration POST is the
	// observed response.
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}

	vals := url.Values{
		"Login":                {login},
		"Password":             {"regpass123"},
		"PasswordConfirmation": {"regpass123"},
		"FirstName":            {"Reg"},
		"City":                 {"RegCity"},
		"Role":                 {"spw"},
		"FosterFamily":         {"true"},
		"Admin":                {"true"},
		"Approved":             {"true"},
		"Remark":               {"self-granted"},
	}
	resp := roleTestPostForm(t, client, srv.URL, "/registration/new", "/registration/", vals)
	resp.Body.Close()
	// UsersCreate redirects anonymous registrations with a hard-coded 302.
	require.Equal(t, http.StatusFound, resp.StatusCode, "registration redirects")

	var got models.User
	require.NoError(t, models.DB.Where("login = ?", login).First(&got))
	require.Equal(t, models.UserRoleRegular, got.Role, "role not self-assignable")
	require.False(t, got.FosterFamily, "flags not self-assignable")
	require.False(t, got.Admin, "admin not self-assignable")
	require.False(t, got.Approved, "registration stays unapproved")
	require.False(t, got.Remark.Valid, "remark not self-assignable")
}

// TestScientistVetVisitFormOnOuttakenAnimal verifies the handler exemption:
// scientists may open the vet visit form for an outtaken animal (issue #107),
// everyone else keeps the refusal.
func TestScientistVetVisitFormOnOuttakenAnimal(t *testing.T) {
	tx := searchTestDB(t)
	f := createAnimalSearchFixtures(t, tx)

	var a models.Animal
	require.NoError(t, tx.Find(&a, f.animalA))
	require.True(t, a.OuttakeID.Valid, "fixture animalA is outtaken")
	yearNumber := fmt.Sprintf("%d/%02d", a.YearNumber, a.Year%100)

	scientist := roleTestUser(t, models.UserRoleScientist, false)
	client, baseURL := roleTestLogin(t, scientist)
	code, _ := roleTestGetBody(t, client, baseURL,
		"/veterinaryvisits/new?animal_year_number="+url.PathEscape(yearNumber))
	require.Equal(t, http.StatusOK, code, "scientist may open vet visit form on outtaken animal")

	regular := roleTestUser(t, models.UserRoleRegular, false)
	client, baseURL = roleTestLogin(t, regular)
	code, _ = roleTestGetBody(t, client, baseURL,
		"/veterinaryvisits/new?animal_year_number="+url.PathEscape(yearNumber))
	require.Equal(t, http.StatusConflict, code, "regular user keeps the outtake refusal")
}
