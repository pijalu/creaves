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
// Model: Singular (Discovery)
// DB Table: Plural (discoveries)
// Resource: Plural (Discoveries)
// Path: Plural (/discoveries)
// View Template Folder: Plural (/templates/discoveries/)

// DiscoveriesResource is the resource for the Discovery model
type DiscoveriesResource struct {
	buffalo.Resource
}

// List gets all Discoveries. This function is mapped to the path
// GET /discoveries
func (v DiscoveriesResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	discoveries := &models.Discoveries{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.Eager().PaginateFromParams(c.Params())

	// Retrieve all Discoveries from the DB
	if err := q.All(discoveries); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("discoveries", discoveries)
		return c.Render(http.StatusOK, r.HTML("/discoveries/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(discoveries))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(discoveries))
	}).Respond(c)
}

// Show gets the data for one Discovery. This function is mapped to
// the path GET /discoveries/{discovery_id}
func (v DiscoveriesResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Discovery
	discovery := &models.Discovery{}

	// To find the Discovery the parameter discovery_id is used.
	if err := tx.Eager().Find(discovery, c.Param("discovery_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("discovery", discovery)

		return c.Render(http.StatusOK, r.HTML("/discoveries/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(discovery))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(discovery))
	}).Respond(c)
}

// New renders the form for creating a new Discovery.
// This function is mapped to the path GET /discoveries/new
func (v DiscoveriesResource) New(c buffalo.Context) error {
	c.Set("discovery", &models.Discovery{})

	return c.Render(http.StatusOK, r.HTML("/discoveries/new.plush.html"))
}

// Create adds a Discovery to the DB. This function is mapped to the
// path POST /discoveries
func (v DiscoveriesResource) Create(c buffalo.Context) error {
	// Allocate an empty Discovery
	discovery := &models.Discovery{}

	// Bind discovery to the html form elements
	if err := c.Bind(discovery); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(discovery)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("discovery", discovery)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/discoveries/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "discovery.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/discoveries/%v", discovery.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(discovery))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(discovery))
	}).Respond(c)
}

// Edit renders a edit form for a Discovery. This function is
// mapped to the path GET /discoveries/{discovery_id}/edit
func (v DiscoveriesResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Discovery
	discovery := &models.Discovery{}

	if err := tx.Eager().Find(discovery, c.Param("discovery_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("discovery", discovery)
	return c.Render(http.StatusOK, r.HTML("/discoveries/edit.plush.html"))
}

// Update changes a Discovery in the DB. This function is mapped to
// the path PUT /discoveries/{discovery_id}
func (v DiscoveriesResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Discovery
	discovery := &models.Discovery{}

	if err := tx.Eager().Find(discovery, c.Param("discovery_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Discovery to the html form elements
	if err := c.Bind(discovery); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(discovery)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("discovery", discovery)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/discoveries/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "discovery.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/discoveries/%v", discovery.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(discovery))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(discovery))
	}).Respond(c)
}

// Destroy deletes a Discovery from the DB. This function is mapped
// to the path DELETE /discoveries/{discovery_id}
func (v DiscoveriesResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Discovery
	discovery := &models.Discovery{}

	// To find the Discovery the parameter discovery_id is used.
	if err := tx.Find(discovery, c.Param("discovery_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	/*
		d, err := DiscoverersResource{}.FindByID(c, discovery.DiscovererID.String())
		if err != nil {
			return err
		}

		if err := tx.Destroy(d); err != nil {
			return err
		}
	*/

	if err := tx.Destroy(discovery); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "discovery.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/discoveries")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(discovery))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(discovery))
	}).Respond(c)
}
