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
// Model: Singular (Outtaketype)
// DB Table: Plural (outtaketypes)
// Resource: Plural (Outtaketypes)
// Path: Plural (/outtaketypes)
// View Template Folder: Plural (/templates/outtaketypes/)

// OuttaketypesResource is the resource for the Outtaketype model
type OuttaketypesResource struct {
	buffalo.Resource
}

// List gets all Outtaketypes. This function is mapped to the path
// GET /outtaketypes
func (v OuttaketypesResource) List(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	outtaketypes := &models.Outtaketypes{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Outtaketypes from the DB
	if err := q.All(outtaketypes); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("outtaketypes", outtaketypes)
		return c.Render(http.StatusOK, r.HTML("/outtaketypes/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(outtaketypes))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(outtaketypes))
	}).Respond(c)
}

// Show gets the data for one Outtaketype. This function is mapped to
// the path GET /outtaketypes/{outtaketype_id}
func (v OuttaketypesResource) Show(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Outtaketype
	outtaketype := &models.Outtaketype{}

	// To find the Outtaketype the parameter outtaketype_id is used.
	if err := tx.Find(outtaketype, c.Param("outtaketype_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("outtaketype", outtaketype)

		return c.Render(http.StatusOK, r.HTML("/outtaketypes/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(outtaketype))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(outtaketype))
	}).Respond(c)
}

// New renders the form for creating a new Outtaketype.
// This function is mapped to the path GET /outtaketypes/new
func (v OuttaketypesResource) New(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	c.Set("outtaketype", &models.Outtaketype{})

	return c.Render(http.StatusOK, r.HTML("/outtaketypes/new.plush.html"))
}

// Create adds a Outtaketype to the DB. This function is mapped to the
// path POST /outtaketypes
func (v OuttaketypesResource) Create(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Allocate an empty Outtaketype
	outtaketype := &models.Outtaketype{}

	// Bind outtaketype to the html form elements
	if err := c.Bind(outtaketype); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(outtaketype)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("outtaketype", outtaketype)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/outtaketypes/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "outtaketype.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/outtaketypes/%v", outtaketype.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(outtaketype))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(outtaketype))
	}).Respond(c)
}

// Edit renders a edit form for a Outtaketype. This function is
// mapped to the path GET /outtaketypes/{outtaketype_id}/edit
func (v OuttaketypesResource) Edit(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Outtaketype
	outtaketype := &models.Outtaketype{}

	if err := tx.Find(outtaketype, c.Param("outtaketype_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("outtaketype", outtaketype)
	return c.Render(http.StatusOK, r.HTML("/outtaketypes/edit.plush.html"))
}

// Update changes a Outtaketype in the DB. This function is mapped to
// the path PUT /outtaketypes/{outtaketype_id}
func (v OuttaketypesResource) Update(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Outtaketype
	outtaketype := &models.Outtaketype{}

	if err := tx.Find(outtaketype, c.Param("outtaketype_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Reset statuses
	outtaketype.Default = false

	// Bind Outtaketype to the html form elements
	if err := c.Bind(outtaketype); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(outtaketype)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("outtaketype", outtaketype)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/outtaketypes/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "outtaketype.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/outtaketypes/%v", outtaketype.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(outtaketype))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(outtaketype))
	}).Respond(c)
}

// Destroy deletes a Outtaketype from the DB. This function is mapped
// to the path DELETE /outtaketypes/{outtaketype_id}
func (v OuttaketypesResource) Destroy(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Outtaketype
	outtaketype := &models.Outtaketype{}

	// To find the Outtaketype the parameter outtaketype_id is used.
	if err := tx.Find(outtaketype, c.Param("outtaketype_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(outtaketype); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "outtaketype.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/outtaketypes")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(outtaketype))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(outtaketype))
	}).Respond(c)
}
