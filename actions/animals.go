package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/x/responder"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Animal)
// DB Table: Plural (animals)
// Resource: Plural (Animals)
// Path: Plural (/animals)
// View Template Folder: Plural (/templates/animals/)

// AnimalsResource is the resource for the Animal model
type AnimalsResource struct {
	buffalo.Resource
}

// List gets all Animals. This function is mapped to the path
// GET /animals
func (v AnimalsResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animalID := c.Param("animal_id")
	if len(animalID) > 0 {
		exists, err := tx.Where("ID = ?", animalID).Exists(&models.Animal{})
		if err != nil {
			return err
		}
		if exists {
			return c.Redirect(http.StatusSeeOther, "/animals/%v", animalID)
		}
		// for message
		data := map[string]interface{}{
			"animalID": animalID,
		}
		c.Flash().Add("danger", T.Translate(c, "animal.not.found", data))
	}

	animals := &models.Animals{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Animals from the DB
	if err := q.Eager().Order("created_at desc").All(animals); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("animals", animals)
		return c.Render(http.StatusOK, r.HTML("/animals/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animals))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animals))
	}).Respond(c)
}

// Show gets the data for one Animal. This function is mapped to
// the path GET /animals/{animal_id}
func (v AnimalsResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animal
	animal := &models.Animal{}

	// To find the Animal the parameter animal_id is used.
	if err := tx.Eager().Find(animal, c.Param("animal_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	// force 2nd level
	if err := tx.Eager().Find(&animal.Discovery.Discoverer, animal.Discovery.DiscovererID); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if animal.Outtake != nil {
		c.Logger().Debugf("Loading outtake")
		if err := tx.Eager().Find(animal.Outtake, animal.OuttakeID); err != nil {
			return c.Error(http.StatusNotFound, err)
		}
	}

	c.Logger().Debugf("Loaded animal: %v", animal)

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("animal", animal)

		return c.Render(http.StatusOK, r.HTML("/animals/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animal))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animal))
	}).Respond(c)
}

// New renders the form for creating a new Animal.
// This function is mapped to the path GET /animals/new
func (v AnimalsResource) New(c buffalo.Context) error {
	c.Set("animal", &models.Animal{})

	if err := setupContext(c); err != nil {
		return err
	}

	return c.Render(http.StatusOK, r.HTML("/animals/new.plush.html"))
}

// Create adds a Animal to the DB. This function is mapped to the
// path POST /animals
func (v AnimalsResource) Create(c buffalo.Context) error {
	// Allocate an empty Animal
	animal := &models.Animal{}

	// Bind animal to the html form elements
	if err := c.Bind(animal); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Validate the data from the html form
	// 2 steps
	verrs, err := tx.Eager().ValidateAndCreate(&animal.Discovery)
	if err != nil {
		return err
	}
	if !verrs.HasAny() {
		c.Logger().Debugf("Animal: %v", animal)
		verrs, err = tx.Eager().ValidateAndCreate(animal)
		if err != nil {
			return err
		}
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("animal", animal)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/animals/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "animal.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/animals/%v", animal.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(animal))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(animal))
	}).Respond(c)
}

func setupContext(c buffalo.Context) error {
	at, err := animalTypes(c)
	if err != nil {
		return err
	}
	c.Set("selectAnimalTypes", animalTypesToSelectables(at))

	aa, err := animalages(c)
	if err != nil {
		return err
	}
	c.Set("selectAnimalages", animalagesToSelectables(aa))

	ot, err := outtakeTypes(c)
	if err != nil {
		return err
	}
	c.Set("selectOuttaketype", outtakeTypesToSelectables(ot))

	return nil
}

// Edit renders a edit form for a Animal. This function is
// mapped to the path GET /animals/{animal_id}/edit
func (v AnimalsResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	if err := setupContext(c); err != nil {
		return err
	}

	// Allocate an empty Animal
	animal := &models.Animal{}

	if err := tx.Eager().Find(animal, c.Param("animal_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	// force 2nd level
	if err := tx.Eager().Find(&animal.Discovery.Discoverer, animal.Discovery.DiscovererID); err != nil {
		return c.Error(http.StatusNotFound, err)
	}
	if animal.Outtake != nil {
		if err := tx.Eager().Find(animal.Outtake, animal.OuttakeID); err != nil {
			return c.Error(http.StatusNotFound, err)
		}
	}

	c.Set("animal", animal)
	return c.Render(http.StatusOK, r.HTML("/animals/edit.plush.html"))
}

// Update changes a Animal in the DB. This function is mapped to
// the path PUT /animals/{animal_id}
func (v AnimalsResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animal
	animal := &models.Animal{}

	if err := tx.Eager().Find(animal, c.Param("animal_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Animal to the html form elements
	if err := c.Bind(animal); err != nil {
		return err
	}

	c.Logger().Debugf("Updating animal: %v", animal)

	// Fix link
	animal.Discovery.Discoverer.ID = animal.Discovery.DiscovererID

	// Validate the data from the html form
	updateModels := []interface{}{
		&animal.Discovery.Discoverer,
		&animal.Discovery,
		&animal.Intake,
		animal.Outtake,
		animal,
	}

	var verrs *validate.Errors
	var err error
	for _, m := range updateModels {
		if m == nil {
			continue
		}
		verrs, err = tx.Eager().ValidateAndUpdate(m)
		if err != nil {
			return err
		}
		if verrs.HasAny() {
			break
		}
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("animal", animal)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/animals/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "animal.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/animals/%v", animal.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(animal))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(animal))
	}).Respond(c)
}

// Destroy deletes a Animal from the DB. This function is mapped
// to the path DELETE /animals/{animal_id}
func (v AnimalsResource) Destroy(c buffalo.Context) error {
	// Admin only
	if !GetCurrentUser(c).Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Animal
	animal := &models.Animal{}

	// To find the Animal the parameter animal_id is used.
	if err := tx.Eager().Find(animal, c.Param("animal_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Eager().Destroy(animal); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "animal.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/animals")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(animal))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(animal))
	}).Respond(c)
}
