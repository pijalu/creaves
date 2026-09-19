package actions

import (
	"net/http"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/x/responder"
)

// ---------------------------------------------------------------------------
// Role guard for restricted accounts (issue #107): lecteur (view-only),
// scientifique (U Liège/DEMNA: view + vet visit creation) and SPW (view on a
// fixed page set + users listing). Regular users and admins pass through.
//
// Enforcement is whitelist based on method + path prefixes, layered UNDER the
// per-handler checks (admins-only pages etc. still apply afterwards).
// ---------------------------------------------------------------------------

// ownUserWriteAllowed verifies a GET/PUT/POST on /users/{id} targets the
// current user's own account (self-management for restricted roles).
func ownUserWriteAllowed(c buffalo.Context, method, path string) bool {
	if method != "GET" && method != "PUT" && method != "POST" {
		return false
	}
	cu := GetCurrentUser(c)
	if cu == nil {
		return false
	}
	// /users/{id}, /users/{id}/edit and the PUT /users/{id} update
	if path == "/users/"+cu.ID.String() ||
		path == "/users/"+cu.ID.String()+"/edit" ||
		strings.HasPrefix(path, "/users/"+cu.ID.String()+"/") {
		return true
	}
	return false
}

// RoleGuard blocks requests of restricted-role accounts outside their
// whitelist. Registered right after Authorize.
func RoleGuard(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		cu := GetCurrentUser(c)
		if cu != nil && cu.IsRestricted() && !roleAllows(c, cu, c.Request().Method, c.Request().URL.Path) {
			c.Logger().Debugf("RoleGuard: %s role %q blocked %s %s", cu.Login, cu.Role, c.Request().Method, c.Request().URL.Path)
			return responder.Wants("html", func(c buffalo.Context) error {
				c.Flash().Add("danger", T.Translate(c, "users.role.denied"))
				return c.Redirect(http.StatusSeeOther, "/")
			}).Wants("json", func(c buffalo.Context) error {
				return c.Error(http.StatusForbidden, errRoleDenied)
			}).Wants("xml", func(c buffalo.Context) error {
				return c.Error(http.StatusForbidden, errRoleDenied)
			}).Respond(c)
		}
		return next(c)
	}
}

var errRoleDenied = errForbidden("role does not allow this action")

// errForbidden builds a static forbidden error.
func errForbidden(msg string) error { return &roleDeniedError{msg} }

type roleDeniedError struct{ msg string }

func (e *roleDeniedError) Error() string { return e.msg }

// roleAllows decides whether a restricted account may perform this request.
func roleAllows(c buffalo.Context, u *models.User, method, path string) bool {
	path = strings.TrimSuffix(path, "/")
	if path == "" {
		path = "/"
	}

	// own-account management is always allowed (view + edit own profile)
	if ownUserWriteAllowed(c, method, path) {
		return true
	}

	switch {
	case u.IsReader():
		// view everywhere, change nothing
		return method == http.MethodGet
	case u.IsScientist():
		if method != http.MethodGet {
			// the single write: creating a veterinary visit (also on
			// outtaken animals — handler exemption below)
			return path == "/veterinaryvisits" && method == http.MethodPost
		}
		return true
	case u.IsSPW():
		if method != http.MethodGet {
			return false
		}
		switch {
		case path == "/" || path == "/dashboard",
			path == "/animals",
			strings.HasPrefix(path, "/animals/"),
			strings.HasPrefix(path, "/attachments/"), // media shown on animal pages
			path == "/todos",                         // read-only visibility, same as lecteur
			strings.HasPrefix(path, "/reports"),
			path == "/users", // users listing (view only)
			strings.HasPrefix(path, "/users/"):
			return true
		}
		return false
	}
	return true
}

// userRoleName maps an account role key to its display label (issue #107).
// Empty/unknown roles map to the regular user label.
func userRoleName(role string) string {
	if name, ok := models.UserRoleNames[role]; ok {
		return name
	}
	return models.UserRoleNames[models.UserRoleRegular]
}

// zeroAdminOnlyUserFields resets the admin-only columns of issue #107
// (account role, volunteer flags, remark) to their defaults. Used on public
// registration so a crafted POST can never set privileged markers.
func zeroAdminOnlyUserFields(u *models.User) {
	u.Role = models.UserRoleRegular
	u.FosterFamily = false
	u.Transporter = false
	u.BoardMember = false
	u.Committee = false
	u.Coordinator = false
	u.Veterinarian = false
	u.Referent = false
	u.TeamLeader = false
	u.Caregiver = false
	u.CareAssistant = false
	u.Helper = false
	u.Remark = nulls.String{}
}

// applyUserFieldPermissions enforces the field-level rights of issue #107 on
// a bound user update: `user` holds the submitted values, `was` the persisted
// row. Contact fields (names, address, contact info) may be changed by the
// account owner and admins; the volunteer flags, the remark and the account
// role are admin-only. Values are restored from `was` when the actor lacks
// the right to change them.
func applyUserFieldPermissions(c buffalo.Context, user, was *models.User) {
	if cu := GetCurrentUser(c); cu != nil && cu.Admin {
		return
	}
	user.FosterFamily = was.FosterFamily
	user.Transporter = was.Transporter
	user.BoardMember = was.BoardMember
	user.Committee = was.Committee
	user.Coordinator = was.Coordinator
	user.Veterinarian = was.Veterinarian
	user.Referent = was.Referent
	user.TeamLeader = was.TeamLeader
	user.Caregiver = was.Caregiver
	user.CareAssistant = was.CareAssistant
	user.Helper = was.Helper
	user.Remark = was.Remark
	user.Role = was.Role
}
