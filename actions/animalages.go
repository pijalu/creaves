package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/x/responder"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Animalage)
// DB Table: Plural (animalages)
// Resource: Plural (Animalages)
// Path: Plural (/animalages)
// View Template Folder: Plural (/templates/animalages/)

// AnimalagesResource is the resource for the Animalage model
type AnimalagesResource struct {
	buffalo.Resource
}

// List gets all Animalages. This function is mapped to the path
// GET /animalages
func (v AnimalagesResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animalages := &models.Animalages{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Animalages from the DB
	if err := q.All(animalages); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("animalages", animalages)
		return c.Render(http.StatusOK, r.HTML("/animalages/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animalages))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animalages))
	}).Respond(c)
}

// Show gets the data for one Animalage. This function is mapped to
// the path GET /animalages/{animalage_id}
func (v AnimalagesResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animalage
	animalage := &models.Animalage{}

	// To find the Animalage the parameter animalage_id is used.
	if err := tx.Find(animalage, c.Param("animalage_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("animalage", animalage)

		return c.Render(http.StatusOK, r.HTML("/animalages/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animalage))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animalage))
	}).Respond(c)
}

// New renders the form for creating a new Animalage.
// This function is mapped to the path GET /animalages/new
func (v AnimalagesResource) New(c buffalo.Context) error {
	c.Set("animalage", &models.Animalage{})

	return c.Render(http.StatusOK, r.HTML("/animalages/new.plush.html"))
}

// Create adds a Animalage to the DB. This function is mapped to the
// path POST /animalages
func (v AnimalagesResource) Create(c buffalo.Context) error {
	// Allocate an empty Animalage
	animalage := &models.Animalage{}

	// Bind animalage to the html form elements
	if err := c.Bind(animalage); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(animalage)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("animalage", animalage)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/animalages/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "animalage.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/animalages/%v", animalage.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(animalage))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(animalage))
	}).Respond(c)
}

// Edit renders a edit form for a Animalage. This function is
// mapped to the path GET /animalages/{animalage_id}/edit
func (v AnimalagesResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animalage
	animalage := &models.Animalage{}

	if err := tx.Find(animalage, c.Param("animalage_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("animalage", animalage)
	return c.Render(http.StatusOK, r.HTML("/animalages/edit.plush.html"))
}

// Update changes a Animalage in the DB. This function is mapped to
// the path PUT /animalages/{animalage_id}
func (v AnimalagesResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animalage
	animalage := &models.Animalage{}

	if err := tx.Find(animalage, c.Param("animalage_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Animalage to the html form elements
	if err := c.Bind(animalage); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(animalage)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("animalage", animalage)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/animalages/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "animalage.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/animalages/%v", animalage.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(animalage))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(animalage))
	}).Respond(c)
}

// Destroy deletes a Animalage from the DB. This function is mapped
// to the path DELETE /animalages/{animalage_id}
func (v AnimalagesResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animalage
	animalage := &models.Animalage{}

	// To find the Animalage the parameter animalage_id is used.
	if err := tx.Find(animalage, c.Param("animalage_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(animalage); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "animalage.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/animalages")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(animalage))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(animalage))
	}).Respond(c)
}
