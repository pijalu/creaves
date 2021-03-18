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
// Model: Singular (Veterinaryvisit)
// DB Table: Plural (veterinaryvisits)
// Resource: Plural (Veterinaryvisits)
// Path: Plural (/veterinaryvisits)
// View Template Folder: Plural (/templates/veterinaryvisits/)

// VeterinaryvisitsResource is the resource for the Veterinaryvisit model
type VeterinaryvisitsResource struct {
	buffalo.Resource
}

func (v VeterinaryvisitsResource) setContext(c buffalo.Context) error {
	// Set care type
	u, err := users(c)
	if err != nil {
		return err
	}
	c.Set("selectUsers", usersToSelectables(u))

	return nil
}

// List gets all Veterinaryvisits. This function is mapped to the path
// GET /veterinaryvisits
func (v VeterinaryvisitsResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	veterinaryvisits := &models.Veterinaryvisits{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Veterinaryvisits from the DB
	if err := q.Eager().Order("date desc").All(veterinaryvisits); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("veterinaryvisits", veterinaryvisits)
		return c.Render(http.StatusOK, r.HTML("/veterinaryvisits/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(veterinaryvisits))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(veterinaryvisits))
	}).Respond(c)
}

// Show gets the data for one Veterinaryvisit. This function is mapped to
// the path GET /veterinaryvisits/{veterinaryvisit_id}
func (v VeterinaryvisitsResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Veterinaryvisit
	veterinaryvisit := &models.Veterinaryvisit{}

	// To find the Veterinaryvisit the parameter veterinaryvisit_id is used.
	if err := tx.Eager().Find(veterinaryvisit, c.Param("veterinaryvisit_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("veterinaryvisit", veterinaryvisit)

		return c.Render(http.StatusOK, r.HTML("/veterinaryvisits/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(veterinaryvisit))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(veterinaryvisit))
	}).Respond(c)
}

// New renders the form for creating a new Veterinaryvisit.
// This function is mapped to the path GET /veterinaryvisits/new
func (v VeterinaryvisitsResource) New(c buffalo.Context) error {
	if err := v.setContext(c); err != nil {
		return err
	}

	vv := &models.Veterinaryvisit{
		Date:   time.Now(),
		UserID: GetCurrentUser(c).ID,
	}
	c.Set("veterinaryvisit", vv)

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
			c.Flash().Add("danger", T.Translate(c, "veterinaryvisit.animal.not.found", data))
			errCode = http.StatusNotFound
		}

		if errCode != http.StatusOK {
			c.Logger().Debugf("Not ok: %v", errCode)
			return c.Render(errCode, r.HTML("/veterinaryvisits/new.plush.html"))
		}

		vv.Animal = *animal
		vv.AnimalID = animal.ID

		// Set users
		u, err := users(c)
		if err != nil {
			return err
		}
		c.Set("selectCaretype", usersToSelectables(u))
	}

	c.Set("veterinaryvisit", vv)
	return c.Render(http.StatusOK, r.HTML("/veterinaryvisits/new.plush.html"))
}

// Create adds a Veterinaryvisit to the DB. This function is mapped to the
// path POST /veterinaryvisits
func (v VeterinaryvisitsResource) Create(c buffalo.Context) error {
	// Allocate an empty Veterinaryvisit
	veterinaryvisit := &models.Veterinaryvisit{}

	// Bind veterinaryvisit to the html form elements
	if err := c.Bind(veterinaryvisit); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(veterinaryvisit)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("veterinaryvisit", veterinaryvisit)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/veterinaryvisits/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "veterinaryvisit.created.success"))
		if len(c.Param("back")) > 0 {
			// and redirect to the show page
			return c.Redirect(http.StatusSeeOther, c.Param("back"))
		}
		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/veterinaryvisits/%v", veterinaryvisit.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(veterinaryvisit))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(veterinaryvisit))
	}).Respond(c)
}

// Edit renders a edit form for a Veterinaryvisit. This function is
// mapped to the path GET /veterinaryvisits/{veterinaryvisit_id}/edit
func (v VeterinaryvisitsResource) Edit(c buffalo.Context) error {
	if err := v.setContext(c); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Veterinaryvisit
	veterinaryvisit := &models.Veterinaryvisit{}

	if err := tx.Eager().Find(veterinaryvisit, c.Param("veterinaryvisit_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("veterinaryvisit", veterinaryvisit)
	return c.Render(http.StatusOK, r.HTML("/veterinaryvisits/edit.plush.html"))
}

// Update changes a Veterinaryvisit in the DB. This function is mapped to
// the path PUT /veterinaryvisits/{veterinaryvisit_id}
func (v VeterinaryvisitsResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Veterinaryvisit
	veterinaryvisit := &models.Veterinaryvisit{}

	if err := tx.Find(veterinaryvisit, c.Param("veterinaryvisit_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Veterinaryvisit to the html form elements
	if err := c.Bind(veterinaryvisit); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(veterinaryvisit)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("veterinaryvisit", veterinaryvisit)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/veterinaryvisits/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "veterinaryvisit.updated.success"))
		if len(c.Param("back")) > 0 {
			// and redirect to the show page
			return c.Redirect(http.StatusSeeOther, c.Param("back"))
		}
		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/veterinaryvisits/%v", veterinaryvisit.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(veterinaryvisit))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(veterinaryvisit))
	}).Respond(c)
}

// Destroy deletes a Veterinaryvisit from the DB. This function is mapped
// to the path DELETE /veterinaryvisits/{veterinaryvisit_id}
func (v VeterinaryvisitsResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Veterinaryvisit
	veterinaryvisit := &models.Veterinaryvisit{}

	// To find the Veterinaryvisit the parameter veterinaryvisit_id is used.
	if err := tx.Find(veterinaryvisit, c.Param("veterinaryvisit_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(veterinaryvisit); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "veterinaryvisit.destroyed.success"))
		if len(c.Param("back")) > 0 {
			// and redirect to the show page
			return c.Redirect(http.StatusSeeOther, c.Param("back"))
		}
		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/veterinaryvisits")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(veterinaryvisit))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(veterinaryvisit))
	}).Respond(c)
}
