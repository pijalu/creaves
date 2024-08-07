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
// Model: Singular (Species)
// DB Table: Plural (species)
// Resource: Plural (Species)
// Path: Plural (/species)
// View Template Folder: Plural (/templates/species/)

// SpeciesResource is the resource for the Species model
type SpeciesResource struct {
	buffalo.Resource
}

// List gets all Species. This function is mapped to the path
// GET /species
func (v SpeciesResource) List(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	species := &[]models.Species{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params()).Order("class ASC, `order` ASC, family ASC, creaves_species ASC")

	// Retrieve all Species from the DB
	if err := q.All(species); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("species", species)
		return c.Render(http.StatusOK, r.HTML("species/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(species))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(species))
	}).Respond(c)
}

// Show gets the data for one Species. This function is mapped to
// the path GET /species/{species_id}
func (v SpeciesResource) Show(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Species
	species := &models.Species{}

	// To find the Species the parameter species_id is used.
	if err := tx.Find(species, c.Param("species_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("species", species)

		return c.Render(http.StatusOK, r.HTML("species/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(species))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(species))
	}).Respond(c)
}

// New renders the form for creating a new Species.
// This function is mapped to the path GET /species/new
func (v SpeciesResource) New(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	c.Set("species", &models.Species{})

	return c.Render(http.StatusOK, r.HTML("species/new.plush.html"))
}

// Create adds a Species to the DB. This function is mapped to the
// path POST /species
func (v SpeciesResource) Create(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Allocate an empty Species
	species := &models.Species{}

	// Bind species to the html form elements
	if err := c.Bind(species); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(species)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("species", species)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("species/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "species.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/species/%v", species.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(species))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(species))
	}).Respond(c)
}

// Edit renders a edit form for a Species. This function is
// mapped to the path GET /species/{species_id}/edit
func (v SpeciesResource) Edit(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Species
	species := &models.Species{}

	if err := tx.Find(species, c.Param("species_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("species", species)
	return c.Render(http.StatusOK, r.HTML("species/edit.plush.html"))
}

// Update changes a Species in the DB. This function is mapped to
// the path PUT /species/{species_id}
func (v SpeciesResource) Update(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Species
	species := &models.Species{}

	if err := tx.Find(species, c.Param("species_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// reset flags
	species.Game = false

	// Bind Species to the html form elements
	if err := c.Bind(species); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(species)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("species", species)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("species/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "species.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/species/%v", species.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(species))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(species))
	}).Respond(c)
}

// Destroy deletes a Species from the DB. This function is mapped
// to the path DELETE /species/{species_id}
func (v SpeciesResource) Destroy(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Species
	species := &models.Species{}

	// To find the Species the parameter species_id is used.
	if err := tx.Find(species, c.Param("species_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(species); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "species.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/species")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(species))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(species))
	}).Respond(c)
}
