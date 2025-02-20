package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"

	"creaves/models"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (EntryCause)
// DB Table: Plural (entry_causes)
// Resource: Plural (EntryCauses)
// Path: Plural (/entry_causes)
// View Template Folder: Plural (/templates/entry_causes/)

// EntryCausesResource is the resource for the EntryCause model
type EntryCausesResource struct {
	buffalo.Resource
}

// List gets all EntryCauses. This function is mapped to the path
// GET /entry_causes
func (v EntryCausesResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	entryCauses := &models.EntryCauses{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all EntryCauses from the DB
	if err := q.All(entryCauses); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("entryCauses", entryCauses)
		return c.Render(http.StatusOK, r.HTML("entry_causes/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(entryCauses))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(entryCauses))
	}).Respond(c)
}

// Show gets the data for one EntryCause. This function is mapped to
// the path GET /entry_causes/{entry_cause_id}
func (v EntryCausesResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty EntryCause
	entryCause := &models.EntryCause{}

	// To find the EntryCause the parameter entry_cause_id is used.
	if err := tx.Find(entryCause, c.Param("entry_cause_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("entryCause", entryCause)

		return c.Render(http.StatusOK, r.HTML("entry_causes/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(entryCause))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(entryCause))
	}).Respond(c)
}

// New renders the form for creating a new EntryCause.
// This function is mapped to the path GET /entry_causes/new
func (v EntryCausesResource) New(c buffalo.Context) error {
	c.Set("entryCause", &models.EntryCause{})

	return c.Render(http.StatusOK, r.HTML("entry_causes/new.plush.html"))
}

// Create adds a EntryCause to the DB. This function is mapped to the
// path POST /entry_causes
func (v EntryCausesResource) Create(c buffalo.Context) error {
	// Allocate an empty EntryCause
	entryCause := &models.EntryCause{}

	// Bind entryCause to the html form elements
	if err := c.Bind(entryCause); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(entryCause)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("entryCause", entryCause)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("entry_causes/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "entryCause.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/entry_causes/%v", entryCause.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(entryCause))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(entryCause))
	}).Respond(c)
}

// Edit renders a edit form for a EntryCause. This function is
// mapped to the path GET /entry_causes/{entry_cause_id}/edit
func (v EntryCausesResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty EntryCause
	entryCause := &models.EntryCause{}

	if err := tx.Find(entryCause, c.Param("entry_cause_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("entryCause", entryCause)
	return c.Render(http.StatusOK, r.HTML("entry_causes/edit.plush.html"))
}

// Update changes a EntryCause in the DB. This function is mapped to
// the path PUT /entry_causes/{entry_cause_id}
func (v EntryCausesResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty EntryCause
	entryCause := &models.EntryCause{}

	if err := tx.Find(entryCause, c.Param("entry_cause_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind EntryCause to the html form elements
	if err := c.Bind(entryCause); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(entryCause)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("entryCause", entryCause)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("entry_causes/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "entryCause.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/entry_causes/%v", entryCause.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(entryCause))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(entryCause))
	}).Respond(c)
}

// Destroy deletes a EntryCause from the DB. This function is mapped
// to the path DELETE /entry_causes/{entry_cause_id}
func (v EntryCausesResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty EntryCause
	entryCause := &models.EntryCause{}

	// To find the EntryCause the parameter entry_cause_id is used.
	if err := tx.Find(entryCause, c.Param("entry_cause_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(entryCause); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "entryCause.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/entry_causes")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(entryCause))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(entryCause))
	}).Respond(c)
}
