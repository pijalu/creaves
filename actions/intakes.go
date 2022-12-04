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
// Model: Singular (Intake)
// DB Table: Plural (intakes)
// Resource: Plural (Intakes)
// Path: Plural (/intakes)
// View Template Folder: Plural (/templates/intakes/)

// IntakesResource is the resource for the Intake model
type IntakesResource struct {
	buffalo.Resource
}

// List gets all Intakes. This function is mapped to the path
// GET /intakes
func (v IntakesResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	intakes := &models.Intakes{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Intakes from the DB
	if err := q.All(intakes); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("intakes", intakes)
		return c.Render(http.StatusOK, r.HTML("/intakes/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(intakes))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(intakes))
	}).Respond(c)
}

// Show gets the data for one Intake. This function is mapped to
// the path GET /intakes/{intake_id}
func (v IntakesResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Intake
	intake := &models.Intake{}

	// To find the Intake the parameter intake_id is used.
	if err := tx.Find(intake, c.Param("intake_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("intake", intake)

		return c.Render(http.StatusOK, r.HTML("/intakes/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(intake))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(intake))
	}).Respond(c)
}

// New renders the form for creating a new Intake.
// This function is mapped to the path GET /intakes/new
func (v IntakesResource) New(c buffalo.Context) error {
	c.Set("intake", &models.Intake{})

	return c.Render(http.StatusOK, r.HTML("/intakes/new.plush.html"))
}

// Create adds a Intake to the DB. This function is mapped to the
// path POST /intakes
func (v IntakesResource) Create(c buffalo.Context) error {
	// Allocate an empty Intake
	intake := &models.Intake{}

	// Bind intake to the html form elements
	if err := c.Bind(intake); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(intake)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("intake", intake)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/intakes/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "intake.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/intakes/%v", intake.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(intake))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(intake))
	}).Respond(c)
}

// Edit renders a edit form for a Intake. This function is
// mapped to the path GET /intakes/{intake_id}/edit
func (v IntakesResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Intake
	intake := &models.Intake{}

	if err := tx.Find(intake, c.Param("intake_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("intake", intake)
	return c.Render(http.StatusOK, r.HTML("/intakes/edit.plush.html"))
}

// Update changes a Intake in the DB. This function is mapped to
// the path PUT /intakes/{intake_id}
func (v IntakesResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Intake
	intake := &models.Intake{}

	if err := tx.Find(intake, c.Param("intake_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Intake to the html form elements
	if err := c.Bind(intake); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(intake)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("intake", intake)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/intakes/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "intake.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/intakes/%v", intake.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(intake))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(intake))
	}).Respond(c)
}

// Destroy deletes a Intake from the DB. This function is mapped
// to the path DELETE /intakes/{intake_id}
func (v IntakesResource) Destroy(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Intake
	intake := &models.Intake{}

	// To find the Intake the parameter intake_id is used.
	if err := tx.Find(intake, c.Param("intake_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(intake); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "intake.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/intakes")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(intake))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(intake))
	}).Respond(c)
}
