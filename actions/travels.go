package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// This file is generated by Buffalo. It offers a basic structure for
// adding, editing and deleting a page. If your model is more
// complex or you need more than the basic implementation you need to
// edit this file.

// Following naming logic is implemented in Buffalo:
// Model: Singular (Travel)
// DB Table: Plural (travels)
// Resource: Plural (Travels)
// Path: Plural (/travels)
// View Template Folder: Plural (/templates/travels/)

// TravelsResource is the resource for the Travel model
type TravelsResource struct {
	buffalo.Resource
}

// List gets all Travels. This function is mapped to the path
// GET /travels
func (v TravelsResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	travels := &models.Travels{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params()).Eager()

	// Retrieve all Travels from the DB for given user (or all if admin)
	u := GetCurrentUser(c)
	if !u.Admin {
		q = q.Where("user_id = ?", u.ID)
	}
	if err := q.Eager().All(travels); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("travels", travels)
		return c.Render(http.StatusOK, r.HTML("/travels/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(travels))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(travels))
	}).Respond(c)
}

// Show gets the data for one Travel. This function is mapped to
// the path GET /travels/{travel_id}
func (v TravelsResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Travel
	travel := &models.Travel{}

	// To find the Travel the parameter travel_id is used.
	if err := tx.Eager().Find(travel, c.Param("travel_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	u := GetCurrentUser(c)
	if travel.UserID != u.ID && !u.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("travel", travel)

		return c.Render(http.StatusOK, r.HTML("/travels/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(travel))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(travel))
	}).Respond(c)
}

// New renders the form for creating a new Travel.
// This function is mapped to the path GET /travels/new
func (v TravelsResource) New(c buffalo.Context) error {
	travel := &models.Travel{
		Date:   time.Now(),
		UserID: GetCurrentUser(c).ID,
		User:   GetCurrentUser(c),
	}

	// Set users
	u, err := users(c)
	if err != nil {
		return err
	}
	c.Set("selectUsers", usersToSelectables(u))

	// Set travel type
	tt, err := traveltypes(c)
	if err != nil {
		return err
	}
	c.Set("selectTraveltype", traveltypesToSelectables(tt))

	// set default travel type
	for _, t := range *tt {
		if t.Def {
			travel.Traveltype = &t
			travel.TraveltypeID = t.ID
		}
	}

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
			c.Flash().Add("danger", T.Translate(c, "travels.animal.not.found", data))
			errCode = http.StatusNotFound
		}

		if errCode != http.StatusOK {
			return c.Render(errCode, r.HTML("/outtakes/new.plush.html"))
		}
		travel.Animal = animal
		travel.AnimalID = animal.ID
		c.Set("animal", animal)
	}

	c.Set("travel", travel)
	return c.Render(http.StatusOK, r.HTML("/travels/new.plush.html"))
}

// Create adds a Travel to the DB. This function is mapped to the
// path POST /travels
func (v TravelsResource) Create(c buffalo.Context) error {
	// Allocate an empty Travel
	travel := &models.Travel{}

	// Bind travel to the html form elements
	if err := c.Bind(travel); err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	u := GetCurrentUser(c)
	if travel.UserID != u.ID && !u.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	// Validate the data from the html form
	verrs, err := tx.ValidateAndCreate(travel)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("travel", travel)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/travels/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "travel.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/travels/%v", travel.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(travel))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(travel))
	}).Respond(c)
}

// Edit renders a edit form for a Travel. This function is
// mapped to the path GET /travels/{travel_id}/edit
func (v TravelsResource) Edit(c buffalo.Context) error {
	// Set users
	us, err := users(c)
	if err != nil {
		return err
	}
	c.Set("selectUsers", usersToSelectables(us))

	// Set travel type
	tt, err := traveltypes(c)
	if err != nil {
		return err
	}
	c.Set("selectTraveltype", traveltypesToSelectables(tt))

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Travel
	travel := &models.Travel{}

	if err := tx.Eager().Find(travel, c.Param("travel_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	u := GetCurrentUser(c)
	if travel.UserID != u.ID && !u.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	c.Set("travel", travel)
	return c.Render(http.StatusOK, r.HTML("/travels/edit.plush.html"))
}

// Update changes a Travel in the DB. This function is mapped to
// the path PUT /travels/{travel_id}
func (v TravelsResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Travel
	travel := &models.Travel{}

	if err := tx.Eager().Find(travel, c.Param("travel_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Travel to the html form elements
	if err := c.Bind(travel); err != nil {
		return err
	}

	u := GetCurrentUser(c)
	if travel.UserID != u.ID && !u.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	verrs, err := tx.ValidateAndUpdate(travel)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("travel", travel)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/travels/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "travel.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/travels/%v", travel.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(travel))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(travel))
	}).Respond(c)
}

// Destroy deletes a Travel from the DB. This function is mapped
// to the path DELETE /travels/{travel_id}
func (v TravelsResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Travel
	travel := &models.Travel{}

	// To find the Travel the parameter travel_id is used.
	if err := tx.Find(travel, c.Param("travel_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	u := GetCurrentUser(c)
	if travel.UserID != u.ID && !u.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("restricted"))
	}

	if err := tx.Destroy(travel); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "travel.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/travels")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(travel))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(travel))
	}).Respond(c)
}
