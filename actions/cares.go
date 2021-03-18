package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/x/responder"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Care)
// DB Table: Plural (cares)
// Resource: Plural (Cares)
// Path: Plural (/cares)
// View Template Folder: Plural (/templates/cares/)

// CaresResource is the resource for the Care model
type CaresResource struct {
	buffalo.Resource
}

// List gets all Cares. This function is mapped to the path
// GET /cares
func (v CaresResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	cares := &models.Cares{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Cares from the DB
	if err := q.Eager().All(cares); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("cares", cares)
		return c.Render(http.StatusOK, r.HTML("/cares/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(cares))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(cares))
	}).Respond(c)
}

// Show gets the data for one Care. This function is mapped to
// the path GET /cares/{care_id}
func (v CaresResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Care
	care := &models.Care{}

	// To find the Care the parameter care_id is used.
	if err := tx.Eager().Find(care, c.Param("care_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("care", care)

		return c.Render(http.StatusOK, r.HTML("/cares/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(care))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(care))
	}).Respond(c)
}

// New renders the form for creating a new Care.
// This function is mapped to the path GET /cares/new
func (v CaresResource) New(c buffalo.Context) error {
	care := &models.Care{
		Date: time.Now(),
	}
	c.Set("care", care)

	animalID := c.Param("animal_id")
	if len(animalID) > 0 {
		// Get the DB connection from the context
		tx, ok := c.Value("tx").(*pop.Connection)
		if !ok {
			return fmt.Errorf("no transaction found")
		}

		animal := &models.Animal{}
		errCode := http.StatusOK

		// for message
		data := map[string]interface{}{
			"animalID": animalID,
		}

		if err := tx.Find(animal, animalID); err != nil {
			c.Flash().Add("danger", T.Translate(c, "outtake.animal.not.found", data))
			errCode = http.StatusNotFound
		}

		c.Logger().Debugf("Loaded animal %v", animal)

		if animal.Outtake != nil {
			c.Flash().Add("danger", T.Translate(c, "outtake.animal.outtake.already.exist", data))
			errCode = http.StatusConflict
		}
		if errCode != http.StatusOK {
			return c.Render(errCode, r.HTML("/cares/new.plush.html"))
		}

		care.Animal = *animal
		care.AnimalID = animal.ID

		// Set care type
		ct, err := caretypes(c)
		if err != nil {
			return err
		}
		c.Set("selectCaretype", caretypesToSelectables(ct))
	}
	return c.Render(http.StatusOK, r.HTML("/cares/new.plush.html"))
}

// Create adds a Care to the DB. This function is mapped to the
// path POST /cares
func (v CaresResource) Create(c buffalo.Context) error {
	// Allocate an empty Care
	care := &models.Care{}

	// Bind care to the html form elements
	if err := c.Bind(care); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(care)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("care", care)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/cares/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "care.created.success"))
		if len(c.Param("back")) > 0 {
			return c.Redirect(http.StatusSeeOther, c.Param("back"))
		}
		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/cares/%v", care.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(care))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(care))
	}).Respond(c)
}

// Edit renders a edit form for a Care. This function is
// mapped to the path GET /cares/{care_id}/edit
func (v CaresResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Set care type
	ct, err := caretypes(c)
	if err != nil {
		return err
	}
	c.Set("selectCaretype", caretypesToSelectables(ct))

	// Allocate an empty Care
	care := &models.Care{}

	if err := tx.Find(care, c.Param("care_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("care", care)

	return c.Render(http.StatusOK, r.HTML("/cares/edit.plush.html"))
}

// Update changes a Care in the DB. This function is mapped to
// the path PUT /cares/{care_id}
func (v CaresResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Set care type
	ct, err := caretypes(c)
	if err != nil {
		return err
	}
	c.Set("selectCaretype", caretypesToSelectables(ct))

	// Allocate an empty Care
	care := &models.Care{}

	if err := tx.Find(care, c.Param("care_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Care to the html form elements
	if err := c.Bind(care); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(care)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("care", care)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/cares/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "care.updated.success"))
		if len(c.Param("back")) > 0 {
			return c.Redirect(http.StatusSeeOther, c.Param("back"))
		}
		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/cares/%v", care.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(care))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(care))
	}).Respond(c)
}

// Destroy deletes a Care from the DB. This function is mapped
// to the path DELETE /cares/{care_id}
func (v CaresResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Care
	care := &models.Care{}

	// To find the Care the parameter care_id is used.
	if err := tx.Find(care, c.Param("care_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(care); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "care.destroyed.success"))
		if len(c.Param("back")) > 0 {
			return c.Redirect(http.StatusSeeOther, c.Param(("back")))
		}
		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/cares")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(care))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(care))
	}).Respond(c)
}
