package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

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
// Model: Singular (Treatment)
// DB Table: Plural (treatments)
// Resource: Plural (Treatments)
// Path: Plural (/treatments)
// View Template Folder: Plural (/templates/treatments/)

// TreatmentsResource is the resource for the Treatment model
type TreatmentsResource struct {
	buffalo.Resource
}

// List gets all Treatments. This function is mapped to the path
// GET /treatments
func (v TreatmentsResource) List(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	treatments := &models.Treatments{}

	// Paginate results. Params "page" and "per_page" control pagination.
	// Default values are "page=1" and "per_page=20".
	q := tx.PaginateFromParams(c.Params())

	// Retrieve all Treatments from the DB
	if err := q.Eager().Order("date desc").All(treatments); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("treatments", treatments)
		return c.Render(http.StatusOK, r.HTML("/treatments/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(treatments))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(treatments))
	}).Respond(c)
}

// Show gets the data for one Treatment. This function is mapped to
// the path GET /treatments/{treatment_id}
func (v TreatmentsResource) Show(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Treatment
	treatment := &models.Treatment{}

	// To find the Treatment the parameter treatment_id is used.
	if err := tx.Eager().Find(treatment, c.Param("treatment_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("treatment", treatment)

		return c.Render(http.StatusOK, r.HTML("/treatments/show.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(treatment))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(treatment))
	}).Respond(c)
}

// New renders the form for creating a new Treatment.
// This function is mapped to the path GET /treatments/new
func (v TreatmentsResource) New(c buffalo.Context) error {
	tc := &models.TreatmentTemplate{
		DateFrom: time.Now(),
		DateTo:   time.Now(),
		Morning:  true,
		Noon:     true,
		Evening:  true,
	}
	c.Set("treatmentTemplate", tc)

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
			c.Flash().Add("danger", T.Translate(c, "treatment.animal.not.found", data))
			errCode = http.StatusNotFound
		}

		c.Logger().Debugf("Loaded animal %v", animal)

		if errCode != http.StatusOK {
			return c.Render(errCode, r.HTML("/treatments/new.plush.html"))
		}

		tc.Animal = animal
		tc.AnimalID = animal.ID
	}

	return c.Render(http.StatusOK, r.HTML("/treatments/new.plush.html"))
}

// Create adds a Treatment to the DB. This function is mapped to the
// path POST /treatments
func (v TreatmentsResource) Create(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Treatment
	treatments := []*models.Treatment{}
	treatmentTemplate := &models.TreatmentTemplate{}

	// Bind treatment to the html form elements
	if err := c.Bind(treatmentTemplate); err != nil {
		return err
	}
	treatmentTemplate.Animal = &models.Animal{}
	if err := tx.Find(treatmentTemplate.Animal, treatmentTemplate.AnimalID); err != nil {
		c.Logger().Errorf("Animal id %d not found for %v", treatmentTemplate.AnimalID, treatmentTemplate)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("/treatments/new.plush.html"))
	}
	c.Logger().Debugf("Binded treatmentTemplate with %v", treatmentTemplate)

	// Bad dates
	if treatmentTemplate.DateFrom.After(treatmentTemplate.DateTo) {
		c.Flash().Add("danger", T.Translate(c, "treatment.animal.invalid.date"))
		c.Set("treatmentTemplate", treatmentTemplate)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("/treatments/new.plush.html"))
	}

	bitmap := models.TreatmentBoolToBitmap(
		treatmentTemplate.Morning,
		treatmentTemplate.Noon,
		treatmentTemplate.Evening,
	)

	startDate := treatmentTemplate.DateFrom
	for !startDate.After(treatmentTemplate.DateTo) {
		t := &models.Treatment{
			Date:           startDate,
			AnimalID:       treatmentTemplate.AnimalID,
			Drug:           treatmentTemplate.Drug,
			Dosage:         treatmentTemplate.Dosage,
			Remarks:        treatmentTemplate.Remarks,
			Timebitmap:     bitmap,
			Timedonebitmap: 0,
		}
		treatments = append(treatments, t)
		startDate = startDate.AddDate(0, 0, 1)
	}

	c.Logger().Debugf("Treatments: %v", treatments)

	// Save all
	var verrs *validate.Errors
	var err error
	for _, treatment := range treatments {
		// Validate the data from the html form
		verrs, err = tx.ValidateAndCreate(treatment)
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
			c.Logger().Debug("Errors: %v", verrs)
			c.Set("errors", verrs)

			// Render again the new.html template that the user can
			// correct the input.
			c.Set("treatmentTemplate", treatmentTemplate)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/treatments/new.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "treatment.created.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/treatments/")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.JSON(treatments))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusCreated, r.XML(treatments))
	}).Respond(c)
}

// Edit renders a edit form for a Treatment. This function is
// mapped to the path GET /treatments/{treatment_id}/edit
func (v TreatmentsResource) Edit(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Treatment
	treatment := &models.Treatment{}

	if err := tx.Eager().Find(treatment, c.Param("treatment_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("treatment", treatment)
	return c.Render(http.StatusOK, r.HTML("/treatments/edit.plush.html"))
}

// Update changes a Treatment in the DB. This function is mapped to
// the path PUT /treatments/{treatment_id}
func (v TreatmentsResource) Update(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Treatment
	treatment := &models.Treatment{}

	if err := tx.Find(treatment, c.Param("treatment_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	// Bind Treatment to the html form elements
	if err := c.Bind(treatment); err != nil {
		return err
	}

	verrs, err := tx.ValidateAndUpdate(treatment)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		return responder.Wants("html", func(c buffalo.Context) error {
			// Make the errors available inside the html template
			c.Set("errors", verrs)

			// Render again the edit.html template that the user can
			// correct the input.
			c.Set("treatment", treatment)

			return c.Render(http.StatusUnprocessableEntity, r.HTML("/treatments/edit.plush.html"))
		}).Wants("json", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.JSON(verrs))
		}).Wants("xml", func(c buffalo.Context) error {
			return c.Render(http.StatusUnprocessableEntity, r.XML(verrs))
		}).Respond(c)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a success message
		c.Flash().Add("success", T.Translate(c, "treatment.updated.success"))

		// and redirect to the show page
		return c.Redirect(http.StatusSeeOther, "/treatments/%v", treatment.ID)
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(treatment))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(treatment))
	}).Respond(c)
}

// Destroy deletes a Treatment from the DB. This function is mapped
// to the path DELETE /treatments/{treatment_id}
func (v TreatmentsResource) Destroy(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	// Allocate an empty Treatment
	treatment := &models.Treatment{}

	// To find the Treatment the parameter treatment_id is used.
	if err := tx.Find(treatment, c.Param("treatment_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(treatment); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// If there are no errors set a flash message
		c.Flash().Add("success", T.Translate(c, "treatment.destroyed.success"))

		// Redirect to the index page
		return c.Redirect(http.StatusSeeOther, "/treatments")
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.JSON(treatment))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.XML(treatment))
	}).Respond(c)
}
