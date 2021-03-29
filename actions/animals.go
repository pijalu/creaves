package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"reflect"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
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

// EnrichAnimal load all deps of an animal record
func EnrichAnimal(a *models.Animal, c buffalo.Context) (*models.Animal, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	if err := tx.Find(&a.Animalage, a.AnimalageID); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	if err := tx.Find(&a.Animaltype, a.AnimaltypeID); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	if err := tx.Eager().Find(&a.Discovery, a.DiscoveryID); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	if err := tx.Eager().Find(&a.Intake, a.IntakeID); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}

	if err := tx.Eager().Where("animal_id = ?", a.ID).Order("date desc").All(&a.Cares); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	if err := tx.Eager().Where("animal_id = ?", a.ID).Order("date desc").All(&a.VetVisits); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	if err := tx.Eager().Where("animal_id = ?", a.ID).Order("date desc").All(&a.Treatments); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}

	if a.OuttakeID.Valid {
		c.Logger().Debugf("Loading outtake %v", a.OuttakeID)
		a.Outtake = &models.Outtake{}
		if err := tx.Eager().Find(a.Outtake, a.OuttakeID); err != nil {
			return nil, c.Error(http.StatusNotFound, err)
		}
	}
	return a, nil
}

func (v AnimalsResource) loadAnimal(animal_id string, c buffalo.Context) (*models.Animal, error) {
	a := &models.Animal{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}
	if err := tx.Find(a, animal_id); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	return EnrichAnimal(a, c)
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
	// Load animal
	animal, err := v.loadAnimal(c.Param("animal_id"), c)
	if err != nil {
		return err
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
	ac := struct {
		AnimalCount int
	}{}

	// Bind extra count param
	if err := c.Bind(&ac); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := models.Animals{}

	for i := 0; i < ac.AnimalCount; i++ {
		animal := &models.Animal{}
		// Bind animal to the html form elements
		if err := c.Bind(animal); err != nil {
			return err
		}

		// Add remark
		if ac.AnimalCount > 1 {
			animal.Intake.Remarks = nulls.NewString(
				animal.Intake.Remarks.String +
					fmt.Sprintf("( %d / %d )",
						i+1,
						ac.AnimalCount))
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

		animals = append(animals, *animal)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "animal.created.success"))
		if ac.AnimalCount == 1 {
			// and redirect to the show page
			return c.Redirect(http.StatusSeeOther, "/animals/%v", animals[ac.AnimalCount-1].ID)
		}
		return c.Redirect(http.StatusSeeOther, "/animals/")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(animals))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(animals))
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
	if err := setupContext(c); err != nil {
		return err
	}

	// Load animal
	animal, err := v.loadAnimal(c.Param("animal_id"), c)
	if err != nil {
		return err
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

	// Load animal
	animal, err := v.loadAnimal(c.Param("animal_id"), c)
	if err != nil {
		return err
	}

	// Bind Animal to the html form elements
	if err := c.Bind(animal); err != nil {
		return err
	}

	// Decode additonal form param
	backUrl := struct {
		BackUrl nulls.String
	}{}
	// load backUrl
	if err := c.Bind(&backUrl); err != nil {
		return err
	}
	c.Logger().Debugf("Back: %v", backUrl)

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
	for _, m := range updateModels {
		if reflect.ValueOf(m).IsNil() {
			continue
		}
		c.Logger().Debugf("Updating %v", m)
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
		if backUrl.BackUrl.Valid {
			return c.Redirect(http.StatusSeeOther, backUrl.BackUrl.String)
		}
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
