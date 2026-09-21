package actions

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Issue #199-14 — single "Account role" selector, privileged-flag restore for
// non-admin actors and users listing role/status filters.
// ---------------------------------------------------------------------------

// TestApplyAccountRoleSelection verifies the selector value → flags mapping.
func TestApplyAccountRoleSelection(t *testing.T) {
	tests := []struct {
		sel                       string
		admin, maintainer, shared bool
		role                      string
	}{
		{"", false, false, false, models.UserRoleRegular},
		{"user", false, false, false, models.UserRoleRegular},
		{"bogus", false, false, false, models.UserRoleRegular},
		{"admin", true, false, false, models.UserRoleRegular},
		{"maintainer", true, true, false, models.UserRoleRegular},
		{"shared", false, false, true, models.UserRoleRegular},
		{models.UserRoleReader, false, false, false, models.UserRoleReader},
		{models.UserRoleScientist, false, false, false, models.UserRoleScientist},
		{models.UserRoleSPW, false, false, false, models.UserRoleSPW},
	}
	for _, tc := range tests {
		t.Run("sel="+tc.sel, func(t *testing.T) {
			// start from a fully-flagged user to prove the mapping resets
			u := &models.User{Admin: true, Maintainer: true, Shared: true, Role: models.UserRoleSPW}
			applyAccountRoleSelection(u, tc.sel)
			require.Equal(t, tc.admin, u.Admin, "Admin")
			require.Equal(t, tc.maintainer, u.Maintainer, "Maintainer")
			require.Equal(t, tc.shared, u.Shared, "Shared")
			require.Equal(t, tc.role, u.Role, "Role")
		})
	}
}

// TestUserAccountRoleHelper verifies the inverse mapping used to preselect
// the selector on the edit form.
func TestUserAccountRoleHelper(t *testing.T) {
	require.Equal(t, "", userAccountRole(nil))
	require.Equal(t, "", userAccountRole(&models.User{}))
	require.Equal(t, "", userAccountRole(models.User{}))
	require.Equal(t, "admin", userAccountRole(&models.User{Admin: true}))
	require.Equal(t, "maintainer", userAccountRole(&models.User{Maintainer: true, Admin: true}))
	require.Equal(t, "shared", userAccountRole(&models.User{Shared: true}))
	require.Equal(t, "lecteur", userAccountRole(&models.User{Role: models.UserRoleReader}))
	require.Equal(t, "scientifique", userAccountRole(models.User{Role: models.UserRoleScientist}))
	require.Equal(t, "spw", userAccountRole(&models.User{Role: models.UserRoleSPW}))
	// flags take precedence over a leftover role string
	require.Equal(t, "admin", userAccountRole(&models.User{Admin: true, Role: models.UserRoleSPW}))
}

// TestUsersUpdateAccountRoleSelector verifies an admin folds the account
// flags via the single selector.
func TestUsersUpdateAccountRoleSelector(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	u := roleTestUser(t, models.UserRoleRegular, false)

	client, baseURL := roleTestLogin(t, admin)
	put := func(vals url.Values) {
		vals.Set("_method", "PUT")
		vals.Set("Login", u.Login)
		resp := roleTestPostForm(t, client, baseURL,
			"/users/"+u.ID.String()+"/edit",
			"/users/"+u.ID.String(),
			vals)
		resp.Body.Close()
		require.Equal(t, http.StatusSeeOther, resp.StatusCode, "admin update should redirect")
	}
	reload := func() models.User {
		var got models.User
		require.NoError(t, models.DB.Find(&got, u.ID.String()))
		return got
	}

	// selector: shared → Shared flag only
	put(url.Values{"AccountRole": {"shared"}})
	got := reload()
	require.True(t, got.Shared, "shared selector sets Shared")
	require.False(t, got.Admin, "shared selector clears Admin")
	require.Equal(t, models.UserRoleRegular, got.Role, "shared selector clears Role")

	// selector: lecteur → restricted role only
	put(url.Values{"AccountRole": {models.UserRoleReader}})
	got = reload()
	require.False(t, got.Shared, "lecteur selector clears Shared")
	require.Equal(t, models.UserRoleReader, got.Role, "lecteur selector sets Role")

	// selector: maintainer from a non-maintainer admin → maintainer flag is
	// restored (actor lacks the right), admin still applied.
	put(url.Values{"AccountRole": {"maintainer"}})
	got = reload()
	require.False(t, got.Maintainer, "non-maintainer actor cannot grant maintainer")
	require.True(t, got.Admin, "maintainer implies admin")

	// selector: empty → back to a regular user
	put(url.Values{"AccountRole": {""}})
	got = reload()
	require.False(t, got.Admin, "empty selector clears Admin")
	require.False(t, got.Maintainer)
	require.False(t, got.Shared)
	require.Equal(t, models.UserRoleRegular, got.Role)
}

// TestUsersUpdateNonAdminCannotEscalate verifies a regular user editing their
// own account cannot craft privileged flags (with or without the selector).
func TestUsersUpdateNonAdminCannotEscalate(t *testing.T) {
	u := roleTestUser(t, models.UserRoleRegular, false)

	client, baseURL := roleTestLogin(t, u)
	resp := roleTestPostForm(t, client, baseURL,
		"/users/"+u.ID.String()+"/edit",
		"/users/"+u.ID.String(),
		url.Values{
			"_method":     {"PUT"},
			"Login":       {u.Login},
			"FirstName":   {"Escalation"},
			"Admin":       {"true"},
			"Shared":      {"true"},
			"Maintainer":  {"true"},
			"AccountRole": {"admin"},
			"Role":        {models.UserRoleSPW},
		})
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "self update should redirect")

	var got models.User
	require.NoError(t, models.DB.Find(&got, u.ID.String()))
	require.Equal(t, "Escalation", got.FirstName, "contact field updated")
	require.False(t, got.Admin, "crafted Admin ignored")
	require.False(t, got.Shared, "crafted Shared ignored")
	require.False(t, got.Maintainer, "crafted Maintainer ignored")
	require.Equal(t, models.UserRoleRegular, got.Role, "crafted Role/selector ignored")
}

// TestUsersCreateAccountRoleSelector verifies admin user creation honours the
// single selector.
func TestUsersCreateAccountRoleSelector(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	client, baseURL := roleTestLogin(t, admin)

	sfx := roleTestSuffix()
	login := "roletest_" + sfx
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})

	resp := roleTestPostForm(t, client, baseURL,
		"/users/new",
		"/users",
		url.Values{
			"Login":                {login},
			"Password":             {"rolepass123"},
			"PasswordConfirmation": {"rolepass123"},
			"FirstName":            {"Create"},
			"AccountRole":          {"shared"},
		})
	resp.Body.Close()
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "admin create should redirect")

	var got models.User
	require.NoError(t, models.DB.Where("login = ?", login).First(&got))
	require.True(t, got.Shared, "shared selector sets Shared")
	require.False(t, got.Admin, "shared selector clears Admin")
	require.False(t, got.Maintainer)
	require.Equal(t, models.UserRoleRegular, got.Role)
}

// TestRegistrationAccountRoleSelector verifies the admin creation form (which
// posts to the registration handler) honours the single "Account role"
// selector while anonymous registration still cannot set privileged flags.
func TestRegistrationAccountRoleSelector(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	client, baseURL := roleTestLogin(t, admin)

	sfx := roleTestSuffix()
	login := "roletest_" + sfx
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM users WHERE login = ?", login).Exec()
	})

	resp := roleTestPostForm(t, client, baseURL,
		"/registration/new",
		"/registration/",
		url.Values{
			"Login":                {login},
			"Password":             {"rolepass123"},
			"PasswordConfirmation": {"rolepass123"},
			"FirstName":            {"Reg"},
			"AccountRole":          {models.UserRoleSPW},
			"Approved":             {"true"},
		})
	resp.Body.Close()
	require.Equal(t, http.StatusFound, resp.StatusCode, "logged-in admin registration redirects (302)")

	var got models.User
	require.NoError(t, models.DB.Where("login = ?", login).First(&got))
	require.Equal(t, models.UserRoleSPW, got.Role, "selector sets Role via registration path")
	require.False(t, got.Admin, "selector clears Admin")
	require.True(t, got.Approved, "admin may approve")
}

// TestUsersListFilters verifies the role and status query filters.
func TestUsersListFilters(t *testing.T) {
	admin := roleTestUser(t, models.UserRoleRegular, true)
	client, baseURL := roleTestLogin(t, admin)

	sharedU := roleTestUser(t, models.UserRoleRegular, false)
	sharedU.Shared = true
	require.NoError(t, models.DB.Update(sharedU))
	readerU := roleTestUser(t, models.UserRoleReader, false)
	pendingU := roleTestUser(t, models.UserRoleRegular, false)
	pendingU.Approved = false
	require.NoError(t, models.DB.Update(pendingU))

	pos := func(body, login string) int {
		return strings.Index(body, login)
	}

	// role=shared → shared account listed, reader not
	code, body := roleTestGetBody(t, client, baseURL, "/users?per_page=100&role=shared")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, sharedU.Login), 0, "shared account listed")
	require.Equal(t, -1, pos(body, readerU.Login), "reader not in shared filter")
	require.Equal(t, -1, pos(body, pendingU.Login), "regular user not in shared filter")

	// role=lecteur → reader listed, shared not
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&role=lecteur")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, readerU.Login), 0, "reader listed")
	require.Equal(t, -1, pos(body, sharedU.Login), "shared not in lecteur filter")

	// role=user → regular accounts only (no admin/shared/restricted)
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&role=user")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, pendingU.Login), 0, "regular account listed")
	require.Equal(t, -1, pos(body, sharedU.Login), "shared not in user filter")
	require.Equal(t, -1, pos(body, readerU.Login), "reader not in user filter")
	// the admin's own login always appears in the navbar — check for a table
	// row instead of a bare occurrence
	require.Equal(t, -1, pos(body, `align-middle">`+admin.Login+"<"), "admin row not in user filter")

	// status=pending → unapproved listed, approved not
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&status=pending")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, pendingU.Login), 0, "pending account listed")
	require.Equal(t, -1, pos(body, readerU.Login), "approved reader not in pending filter")

	// combined: role=lecteur + status=active
	code, body = roleTestGetBody(t, client, baseURL, "/users?per_page=100&role=lecteur&status=active")
	require.Equal(t, http.StatusOK, code)
	require.GreaterOrEqual(t, pos(body, readerU.Login), 0, "active reader listed")
	require.Equal(t, -1, pos(body, pendingU.Login), "pending regular not listed")
}
