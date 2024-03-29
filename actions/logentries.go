package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Logentry)
// DB Table: Plural (logentries)
// Resource: Plural (Logentries)
// Path: Plural (/logentries)
// View Template Folder: Plural (/templates/logentries/)

// LogentriesResource is the resource for the Logentry model
type LogentriesResource struct {
	buffalo.Resource
}

// List gets all Logentries. This function is mapped to the path
// GET /logentries
func (v LogentriesResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	logentries := &models.Logentries{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.Eager().PaginateFromParams(c.Params()).Order("updated_at desc, created_at desc")

	// Retrieve all Logentries from the DB
	if err := q.All(logentries); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("logentries", logentries)
		return c.Render(http.StatusOK, r.HTML("/logentries/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(logentries))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(logentries))
	}).Respond(c)
}

// Show gets the data for one Logentry. This function is mapped to
// the path GET /logentries/{logentry_id}
func (v LogentriesResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Logentry
	logentry := &models.Logentry{}

	// To find the Logentry the parameter logentry_id is used.
	if err := tx.Eager().Find(logentry, c.Param("logentry_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("logentry", logentry)

		return c.Render(http.StatusOK, r.HTML("/logentries/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(logentry))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(logentry))
	}).Respond(c)
}

// New renders the form for creating a new Logentry.
// This function is mapped to the path GET /logentries/new
func (v LogentriesResource) New(c buffalo.Context) error {
	c.Set("logentry", &models.Logentry{})

	return c.Render(http.StatusOK, r.HTML("/logentries/new.plush.html"))
}

// Create adds a Logentry to the DB. This function is mapped to the
// path POST /logentries
func (v LogentriesResource) Create(c buffalo.Context) error {
	// Allocate an empty Logentry
	logentry := &models.Logentry{}

	// Bind logentry to the html form elements
	if err := c.Bind(logentry); err != nil {
		return err
	}

	logentry.User = *GetCurrentUser(c)

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(logentry)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("logentry", logentry)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/logentries/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "logentry.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/logentries/%v", logentry.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(logentry))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(logentry))
	}).Respond(c)
}

// Edit renders a edit form for a Logentry. This function is
// mapped to the path GET /logentries/{logentry_id}/edit
func (v LogentriesResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Logentry
	logentry := &models.Logentry{}

	if err := tx.Eager().Find(logentry, c.Param("logentry_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	cu := GetCurrentUser(c)
	if !cu.Admin && cu.ID != logentry.UserID {
		return c.Error(http.StatusForbidden, fmt.Errorf("Cannot edit other user entries"))
	}

	c.Set("logentry", logentry)
	return c.Render(http.StatusOK, r.HTML("/logentries/edit.plush.html"))
}

// Update changes a Logentry in the DB. This function is mapped to
// the path PUT /logentries/{logentry_id}
func (v LogentriesResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Logentry
	logentry := &models.Logentry{}

	if err := tx.Eager().Find(logentry, c.Param("logentry_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Logentry to the html form elements
	if err := c.Bind(logentry); err != nil {
		return err
	}

	cu := GetCurrentUser(c)
	if !cu.Admin && cu.ID != logentry.UserID {
		return c.Error(http.StatusForbidden, fmt.Errorf("Cannot edit other user entries"))
	}

	verrs, err := tx.ValidateAndUpdate(logentry)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("logentry", logentry)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/logentries/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "logentry.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/logentries/%v", logentry.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(logentry))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(logentry))
	}).Respond(c)
}

// Destroy deletes a Logentry from the DB. This function is mapped
// to the path DELETE /logentries/{logentry_id}
func (v LogentriesResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Logentry
	logentry := &models.Logentry{}

	// To find the Logentry the parameter logentry_id is used.
	if err := tx.Eager().Find(logentry, c.Param("logentry_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	cu := GetCurrentUser(c)
	if !cu.Admin && cu.ID != logentry.UserID {
		return c.Error(http.StatusForbidden, fmt.Errorf("Cannot remove other user entries"))
	}

	if err := tx.Destroy(logentry); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "logentry.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/logentries")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(logentry))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(logentry))
	}).Respond(c)
}
