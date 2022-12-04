package actions

import (
	"database/sql"
	"net/http"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// AuthLanding shows a landing page to login
func AuthLanding(c buffalo.Context) error {
	return c.Render(200, r.HTML("auth/landing.plush.html"))
}

// AuthNew loads the signin page
func AuthNew(c buffalo.Context) error {
	c.Set("user", models.User{})
	return c.Render(200, r.HTML("auth/new.plush.html"))
}

// AuthCreate attempts to log the user in with an existing account.
func AuthCreate(c buffalo.Context) error {
	u := &models.User{}
	if err := c.Bind(u); err != nil {
		return errors.WithStack(err)
	}

	tx := c.Value("tx").(*pop.Connection)

	// find a user with the login
	err := tx.Where("login = ?", strings.ToLower(strings.TrimSpace(u.Login))).First(u)

	// helper function to handle bad attempts
	bad := func(er ...string) error {
		verrs := validate.NewErrors()
		if er == nil {
			verrs.Add("login", T.Translate(c, "users.invalid"))
		} else {
			for _, e := range er {
				verrs.Add("login", e)
			}
		}
		c.Set("errors", verrs)
		c.Set("user", u)

		return c.Render(http.StatusUnauthorized, r.HTML("auth/new.plush.html"))
	}

	if err != nil {
		if errors.Cause(err) == sql.ErrNoRows {
			// couldn't find an user with the supplied login address.
			return bad()
		}
		return errors.WithStack(err)
	}

	// confirm that the given password matches the hashed password from the db
	err = bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(u.Password))
	if err != nil {
		return bad()
	}

	c.Logger().Debugf("User: %v", u)

	// check if enabled
	if !u.Approved {
		return bad("Not approved account")
	}

	c.Session().Set("current_user_id", u.ID)
	c.Flash().Add("success", T.Translate(c, "welcome_greeting"))

	redirectURL := "/"
	if redir, ok := c.Session().Get("redirectURL").(string); ok && redir != "" {
		redirectURL = redir
	}

	return c.Redirect(302, redirectURL)
}

// AuthDestroy clears the session and logs a user out
func AuthDestroy(c buffalo.Context) error {
	c.Session().Clear()
	c.Flash().Add("success", T.Translate(c, "users.logout"))
	return c.Redirect(302, "/auth/new")
}
