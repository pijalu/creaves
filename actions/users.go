package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// UsersNew renders the users form
func UsersNew(c buffalo.Context) error {
	u := models.User{}
	c.Set("user", u)
	return c.Render(200, r.HTML("users/new.plush.html"))
}

// UsersCreate registers a new user with the application.
func UsersCreate(c buffalo.Context) error {
	u := &models.User{}
	if err := c.Bind(u); err != nil {
		return errors.WithStack(err)
	}

	cu := GetCurrentUser(c)
	// Single "Account role" selector (issue #199-14): the admin creation
	// form posts to this handler too (new.plush.html → registrationPath),
	// so fold the selector value onto the legacy flags here as well.
	if cu != nil && cu.Admin {
		if sel, present := accountRoleParam(c); present {
			applyAccountRoleSelection(u, sel)
		}
	}

	// Prevent mass-assignment privilege escalation: privileged flags may only
	// be granted by an already-authenticated admin. A crafted registration
	// POST must never be able to set admin/approved/shared directly.
	if cu == nil || !cu.Admin {
		u.Admin = false
		u.Approved = false
		u.Shared = false
		u.Maintainer = false
		zeroAdminOnlyUserFields(u)
	}
	if cu == nil || !cu.Maintainer {
		u.Maintainer = false
	}

	tx := c.Value("tx").(*pop.Connection)
	verrs, err := u.Create(tx)
	if err != nil {
		return errors.WithStack(err)
	}

	if verrs.HasAny() {
		c.Set("user", u)
		c.Set("errors", verrs)
		return c.Render(200, r.HTML("users/new.plush.html"))
	}

	if cu == nil {
		c.Flash().Add("success", "You account is created and will need to be approved!")
		return c.Redirect(302, "/auth/new")
	}
	return c.Redirect(302, "/")
}

// SetCurrentUser attempts to find a user based on the current_user_id
// in the session. If one is found it is set on the context.
func SetCurrentUser(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		if uid := c.Session().Get("current_user_id"); uid != nil {
			uidStr, ok := uid.(string)
			if !ok {
				if u, ok2 := uid.(uuid.UUID); ok2 {
					uidStr = u.String()
				} else {
					uidStr = fmt.Sprintf("%v", uid)
				}
			}
			tx := c.Value("tx").(*pop.Connection)
			u, err := cachedUserByID(tx, uidStr)
			// user gone (or lookup failed)
			if err != nil || u == nil {
				c.Session().Delete("current_user_id")
				c.Session().Set("redirectURL", c.Request().URL.String())
				return next(c)
			}
			// check if still approved
			if !u.Approved {
				c.Session().Clear()
				c.Flash().Add("danger", "Your account is no longer approved")
				return c.Redirect(302, "/auth/new")
			}
			c.Set("current_user", u)
		}
		return next(c)
	}
}

// Authorize require a user be logged in before accessing a route
func Authorize(next buffalo.Handler) buffalo.Handler {
	return func(c buffalo.Context) error {
		if uid := c.Session().Get("current_user_id"); uid == nil {
			c.Session().Set("redirectURL", c.Request().URL.String())

			err := c.Session().Save()
			if err != nil {
				return errors.WithStack(err)
			}

			c.Flash().Add("danger", T.Translate(c, "users.unauthorized"))
			return c.Redirect(302, "/auth/new")
		}
		return next(c)
	}
}

// GetCurrentUser retrieve user from middleware
func GetCurrentUser(c buffalo.Context) *models.User {
	cu := c.Value("current_user")
	if cu == nil {
		return nil
	}
	return cu.(*models.User)
}

// User management

// UsersResource is the resource for the User model
type UsersResource struct {
	buffalo.Resource
}

// accountRoleParam returns the posted "Account role" selector value and
// whether the field was present at all (an empty string is a valid value —
// the regular user role). The form is already parsed by c.Bind.
func accountRoleParam(c buffalo.Context) (string, bool) {
	vals, present := c.Request().Form["AccountRole"]
	if !present || len(vals) == 0 {
		return "", present
	}
	return vals[0], true
}

// New renders the form for creating a new User.
// This function is mapped to the path GET /users/new
func (v UsersResource) New(c buffalo.Context) error {
	c.Set("user", &models.User{})

	return c.Render(http.StatusOK, r.HTML("/users/new.plush.html"))
}

// Create adds a User to the DB. This function is mapped to the
// path POST /users
func (v UsersResource) Create(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		c.Logger().Debugf("Create user rejected with user %v", cu)
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}
	// Allocate an empty User
	user := &models.User{}

	// Bind user to the html form elements
	if err := c.Bind(user); err != nil {
		return err
	}
	// Single "Account role" selector (issue #199-14): when posted, it folds
	// the Admin/Maintainer/Shared/Role fields into one exclusive choice.
	if sel, present := accountRoleParam(c); present {
		applyAccountRoleSelection(user, sel)
	}
	if !cu.Maintainer {
		user.Maintainer = false
	}
	if user.Maintainer {
		user.Admin = true
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// set password
	if err := user.SetPasswordHash(); err != nil {
		return err
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(user)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("user", user)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/users/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "user.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/users/%v", user.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(user))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(user))
	}).Respond(c)
}

// List gets all Users. This function is mapped to the path
// GET /users
func (v UsersResource) List(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	// Admins manage users; SPW accounts have read-only access to the
	// listing (issue #107).
	if !cu.Admin && !cu.IsSPW() {
		c.Logger().Debugf("List user rejected with user %v", cu)
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	users := &models.Users{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Search + sort (issue #107): `q` filters login/name/email/city,
	// `sort`/`dir` pick a whitelisted ORDER BY (default: volunteer name).
	// `role`/`status` filter the account type and approval (issue #199-14).
	q = usersListQuery(q, c.Param("q"), c.Param("role"), c.Param("status"), c.Param("sort"), c.Param("dir"))

	// Retrieve all Users from the DB
	if err := q.All(users); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("users", users)
		c.Set("searchQ", c.Param("q"))
		c.Set("curSort", c.Param("sort"))
		c.Set("curDir", c.Param("dir"))
		c.Set("filterRole", c.Param("role"))
		c.Set("filterStatus", c.Param("status"))
		c.Set("filtersActive", c.Param("q") != "" || c.Param("role") != "" || c.Param("status") != "")
		return c.Render(http.StatusOK, r.HTML("/users/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(users))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(users))
	}).Respond(c)
}

// Show gets the data for one User. This function is mapped to
// the path GET /users/{user_id}
func (v UsersResource) Show(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	// SPW accounts may view user details read-only (issue #107).
	if !cu.Admin && !cu.IsSPW() && cu.ID.String() != c.Param("user_id") {
		c.Logger().Debugf("Show user failed with %v to show user_id %s", cu, c.Param("user_id"))
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty User
	user := &models.User{}

	// To find the User the parameter user_id is used.
	if err := tx.Find(user, c.Param("user_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("user", user)

		return c.Render(http.StatusOK, r.HTML("/users/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(user))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(user))
	}).Respond(c)
}

// Edit renders a edit form for a User. This function is
// mapped to the path GET /users/{user_id}/edit
func (v UsersResource) Edit(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if cu.Shared || (!cu.Admin && cu.ID.String() != c.Param("user_id")) {
		c.Logger().Debugf("Edit user failed with %v to show user_id %s", cu, c.Param("user_id"))
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty User
	user := &models.User{}

	if err := tx.Find(user, c.Param("user_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("user", user)
	return c.Render(http.StatusOK, r.HTML("/users/edit.plush.html"))
}

// Update changes a User in the DB. This function is mapped to
// the path PUT /users/{user_id}
func (v UsersResource) Update(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if cu.Shared || (!cu.Admin && cu.ID.String() != c.Param("user_id")) {
		c.Logger().Debugf("Update user failed with %v to show user_id %s", cu, c.Param("user_id"))
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty User
	user := &models.User{}

	if err := tx.Find(user, c.Param("user_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Snapshot of the persisted row for field-level permission restores
	// (issue #107) — taken before the privileged flags are reset so crafted
	// form fields can be rolled back (issue #199-14).
	was := *user

	// Unchecked checkboxes bind nothing: reset the privileged flags before
	// the bind so an absent field means false.
	wasMaintainer := user.Maintainer
	user.Admin = false
	user.Shared = false
	user.Maintainer = false

	// Bind User to the html form elements
	if err := c.Bind(user); err != nil {
		return err
	}
	// Single "Account role" selector (issue #199-14): when posted by an
	// admin it folds the Admin/Maintainer/Shared/Role fields into one
	// exclusive choice. Non-admin actors may never change the privileged
	// flags — restore the persisted values (crafted fields are ignored).
	if sel, present := accountRoleParam(c); present && cu.Admin {
		applyAccountRoleSelection(user, sel)
	} else if !cu.Admin {
		user.Admin = was.Admin
		user.Shared = was.Shared
		user.Maintainer = was.Maintainer
	}
	if cu.ID.String() == user.ID.String() && cu.Maintainer {
		user.Admin = true
		user.Maintainer = true
	}
	if !cu.Maintainer {
		user.Maintainer = wasMaintainer
	}
	if user.Maintainer {
		user.Admin = true
	}
	// Keep default approval if not admin
	if !cu.Admin {
		user.Approved = cu.Approved
	}

	// Field-level rights for the volunteer columns (issue #107): contact
	// fields are editable by self + admin; flags, remark and the account
	// role are admin-only. Restore persisted values when the actor lacks
	// the right to change them.
	applyUserFieldPermissions(c, user, &was)

	// change password
	if len(user.Password) > 0 {
		if err := user.SetPasswordHash(); err != nil {
			return err
		}
	}

	verrs, err := tx.ValidateAndUpdate(user)
	if err != nil {
		return err
	}
	queuePostCommitInvalidation(c, func() { InvalidateUserCache(user.ID.String()) })

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			c.Set("errors", verrs)
			c.Set("user", user)
			return c.Render(http.StatusUnprocessableEntity, r.HTML("/users/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Flash().Add("success", T.Translate(c, "user.updated.success"))
		return c.Redirect(http.StatusSeeOther, "/users/%v", user.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(user))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(user))
	}).Respond(c)
}

// Destroy deletes a User from the DB. This function is mapped
// to the path DELETE /users/{user_id}
func (v UsersResource) Destroy(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if cu.Shared || (!cu.Admin && cu.ID.String() != c.Param("user_id")) {
		c.Logger().Debugf("Destroy user failed with %v to show user_id %s", cu, c.Param("user_id"))
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty User
	user := &models.User{}

	// To find the User the parameter user_id is used.
	if err := tx.Find(user, c.Param("user_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if user.Login == "admin" {
		return c.Error(http.StatusBadRequest, fmt.Errorf("admin cannot be removed"))
	}

	if err := tx.Destroy(user); err != nil {
		return err
	}
	queuePostCommitInvalidation(c, func() { InvalidateUserCache(user.ID.String()) })

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "user.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/users")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(user))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(user))
	}).Respond(c)
}
