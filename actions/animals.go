package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/x/responder"
	"github.com/gofrs/uuid"
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

// EnrichAnimals load deps of an animal record required for general listings
func EnrichAnimals(a *models.Animals, c buffalo.Context) (*models.Animals, error) {
	// If nothing to enrich, don't preload
	if len(*a) == 0 {
		return a, nil
	}
	// preload types
	// Types
	atsMap := make(map[uuid.UUID]models.Animaltype)
	if ats, err := animalTypes(c); err != nil {
		return nil, err
	} else {
		for _, at := range *ats {
			atsMap[at.ID] = at
		}
	}
	// Ages
	agsMap := make(map[uuid.UUID]models.Animalage)
	if ags, err := animalages(c); err != nil {
		return nil, err
	} else {
		for _, a := range *ags {
			agsMap[a.ID] = a
		}
	}
	// Outtake Type
	otkMap := make(map[uuid.UUID]models.Outtaketype)
	if otk, err := outtakeTypes(c); err != nil {
		return nil, err
	} else {
		for _, ot := range *otk {
			otkMap[ot.ID] = ot
		}
	}

	// Preload all sub items
	intakeIDS := []uuid.UUID{}
	outtakeIDS := []uuid.UUID{}
	discoveryIDS := []uuid.UUID{}

	// Preload all outtakes

	animalsID := []string{}
	// 1st pass - populate IDS / base type
	for i := 0; i < len(*a); i++ {
		(*a)[i].Animalage = agsMap[(*a)[i].AnimalageID]
		(*a)[i].Animaltype = atsMap[(*a)[i].AnimaltypeID]
		intakeIDS = append(intakeIDS, (*a)[i].IntakeID)
		discoveryIDS = append(discoveryIDS, (*a)[i].DiscoveryID)
		if (*a)[i].OuttakeID.Valid {
			outtakeIDS = append(outtakeIDS, (*a)[i].OuttakeID.UUID)
		}
		animalsID = append(animalsID, fmt.Sprintf("%d", (*a)[i].ID))

		//c.Logger().Debugf("Animal %d: intake %v", (*a)[i].ID, (*a)[i].IntakeID)
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	now := time.Now()
	nowDt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tmrDt := nowDt.AddDate(0, 0, 1)

	// Preload all today treatments
	treaments := models.Treatments{}
	tmap := map[int]models.Treatments{}
	if err := tx.Where("date >= ?", nowDt).Where("date < ?", tmrDt).Where(
		fmt.Sprintf("animal_id IN (%s)", strings.Join(animalsID, ","))).Order("animal_id desc").All(&treaments); err != nil {
		return nil, c.Error(http.StatusNotFound, err)
	}
	// remap by animal id
	for _, t := range treaments {
		tmap[t.AnimalID] = append(tmap[t.AnimalID], t)
	}
	// populate animals
	for i := 0; i < len(*a); i++ {
		(*a)[i].Treatments = tmap[(*a)[i].ID]
	}

	dts := &models.Discoveries{}
	if len(discoveryIDS) > 0 {
		if err := tx.Where("ID in (?)", discoveryIDS).All(dts); err != nil {
			return nil, err
		}
	}
	// discovery map
	discoveryMap := make(map[uuid.UUID]models.Discovery)
	for _, i := range *dts {
		discoveryMap[i.ID] = i
	}

	its := &models.Intakes{}
	if err := tx.Where("ID in (?)", intakeIDS).All(its); err != nil {
		return nil, err
	}
	// Intake map
	intakeMap := make(map[uuid.UUID]models.Intake)
	for _, i := range *its {
		intakeMap[i.ID] = i
	}

	ots := &models.Outtakes{}
	if len(outtakeIDS) > 0 {
		if err := tx.Where("ID in (?)", outtakeIDS).All(ots); err != nil {
			return nil, err
		}
	}
	// outtake map
	outtakeMap := make(map[uuid.UUID]models.Outtake)
	for _, i := range *ots {
		outtakeMap[i.ID] = i
	}

	// 2nd pass: Populate intakes
	// 1st pass - populate IDS
	for i := 0; i < len(*a); i++ {
		(*a)[i].Intake = intakeMap[(*a)[i].IntakeID]
		(*a)[i].Discovery = discoveryMap[(*a)[i].DiscoveryID]
		if (*a)[i].OuttakeID.Valid {
			outtake := outtakeMap[(*a)[i].OuttakeID.UUID]
			outtake.Type = otkMap[outtake.TypeID]
			(*a)[i].Outtake = &outtake
		}
	}

	return a, nil
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

	animalYearNumber := c.Param("animal_year_number")
	if len(animalYearNumber) > 0 {
		c.Logger().Debug("animalYearNumber:", animalYearNumber)
		matches := AnimalYearNumberRegEx.FindStringSubmatch(animalYearNumber)
		if matches == nil {
			return fmt.Errorf("invalid year number: %s", animalYearNumber)
		}
		c.Logger().Debug("animalYearNumber regex matches:", matches)
		q := tx.Where("YearNumber = ?", matches[1])
		if len(matches) == 4 && len(matches[3]) == 2 {
			q = q.Where("Year = ?", fmt.Sprintf("20%s", matches[3]))
		}

		animal := models.Animal{}
		err := q.Order("ID desc").First(&animal)
		if err == nil {
			return c.Redirect(http.StatusSeeOther, "/animals/%v", animal.ID)
		}
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
	if err := q.Order("ID desc").All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimals(animals, c); err != nil {
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

	//c.Logger().Debugf("Loaded animal: %v", animal)

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
	a := &models.Animal{}

	c.Set("animal", a)

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

	type registerYearNumber struct {
		Year       nulls.Int `db:"Year"`
		YearNumber nulls.Int `db:"YearNumber"`
	}

	ryn := registerYearNumber{}
	if err := tx.Eager().RawQuery("SELECT MAX(Year) as 'Year', MAX(yearNumber)+1 as 'YearNumber' from animals where year = ?", time.Now().Year()).First(&ryn); err != nil {
		return err
	}

	if !ryn.Year.Valid {
		ryn.Year.Int = time.Now().Year()
	}
	if !ryn.YearNumber.Valid {
		ryn.YearNumber.Int = 1
	}

	for i := 0; i < ac.AnimalCount; i++ {
		animal := &models.Animal{}
		// Bind animal to the html form elements
		if err := c.Bind(animal); err != nil {
			return err
		}
		// Set discovery date
		animal.IntakeDate = animal.Intake.Date

		// Set year+Number
		animal.Year = ryn.Year.Int
		animal.YearNumber = ryn.YearNumber.Int
		ryn.YearNumber.Int++

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

	c.Set("selectFeedingPeriod", selectFeedingPeriod())

	z, err := zones(c)
	if err != nil {
		return err
	}
	c.Set("selectZone", zonesToSelectables(z))

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

	// save original cage
	originalCage := animal.Cage
	originalFeeding := animal.Feeding

	// Bind Animal to the html form elements
	if err := c.Bind(animal); err != nil {
		return err
	}

	// Decode Feeding times
	feedingTimes := struct {
		AnimalFeedingStartTime string
		AnimalFeedingEndTime   string
	}{}
	if err := c.Bind(&feedingTimes); err != nil {
		return err
	}
	c.Logger().Debugf("feedingTimes: %v", feedingTimes)
	animal.FeedingStart = timeToNullTime(feedingTimes.AnimalFeedingStartTime)
	animal.FeedingEnd = timeToNullTime(feedingTimes.AnimalFeedingEndTime)

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

	c.Logger().Debugf("originalCage: %v - cage: %v", originalCage, animal.Cage)

	// Load care types
	careTypes, err := caretypes(c)
	if err != nil {
		return err
	}

	// Create new defailt care if animal was moved
	if !verrs.HasAny() && originalCage.String != "" && animal.Cage.String != originalCage.String {
		c.Logger().Debugf("Creating new care for cage move for animalID %d", animal.ID)
		care := &models.Care{}
		care.Animal = *animal
		care.AnimalID = animal.ID
		care.Date = models.NowOffset()
		care.Note = nulls.NewString(fmt.Sprintf("Cage %s => %s", originalCage.String, animal.Cage.String))

		// Set care to default
		for _, careType := range *careTypes {
			ctn := strings.ToLower(careType.Name)
			// try to find move
			if strings.Contains(ctn, "move") || strings.Contains(ctn, "déplacement") {
				care.Type = careType
				care.TypeID = careType.ID
				break
			}
			// fall back to default
			if careType.Def {
				care.Type = careType
				care.TypeID = careType.ID
			}
		}

		c.Logger().Debugf("Creating new care for cage move for animalID %d: %v", animal.ID, care)
		verrs, err = tx.Eager().ValidateAndCreate(care)
		if err != nil {
			return err
		}
	}

	c.Logger().Debugf("originalCage: %v - cage: %v", originalCage, animal.Cage)
	// Create new defailt care if animal feeding is updated
	if !verrs.HasAny() && animal.Feeding.String != originalFeeding.String && animal.Feeding.String != "" {
		c.Logger().Debugf("Creating new care for updated feeding animalID %d", animal.ID)
		care := &models.Care{}
		care.Animal = *animal
		care.AnimalID = animal.ID
		care.Date = models.NowOffset()
		care.Note = animal.Feeding

		// Set care to default
		for _, careType := range *careTypes {
			ctn := strings.ToLower(careType.Name)
			// try to find move
			if strings.Contains(ctn, "feedings") || strings.Contains(ctn, "alimentation") {
				care.Type = careType
				care.TypeID = careType.ID
				break
			}
			// fall back to default
			if careType.Def {
				care.Type = careType
				care.TypeID = careType.ID
			}
		}

		c.Logger().Debugf("Creating new care for cage move for animalID %d: %v", animal.ID, care)
		verrs, err = tx.Eager().ValidateAndCreate(care)
		if err != nil {
			return err
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
