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

// SQL_CARES_IN_WARNING lists all cares in warning
const SQL_CARES_IN_WARNING = `
SELECT
  c.*,
  a.year,
  a.yearNumber
FROM
  cares c
JOIN
  animals a ON c.animal_id = a.id
JOIN
  caretypes ct_warning ON c.type_id = ct_warning.id AND ct_warning.warning = TRUE
JOIN
  (
    SELECT
      animal_id,
      MAX(date) AS last_care_date
    FROM
      cares
    WHERE
      type_id IN (
        SELECT
          id
        FROM
          caretypes
        WHERE
          reset_warning = TRUE OR warning = TRUE
      )
    GROUP BY
      animal_id
  ) AS last_cares ON c.animal_id = last_cares.animal_id AND c.date = last_cares.last_care_date
WHERE
  a.outtake_id IS NULL
  AND c.type_id IN (SELECT id FROM caretypes WHERE warning = TRUE)
ORDER BY
  c.date DESC
`

// SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE returns the count of animal in care per type
const SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE = `
select at.name as 'Name', count(1) as 'Count'
from animaltypes at, animals a
 WHERE a.outtake_id is NULL
   and a.animaltype_id = at.id
GROUP BY at.name
ORDER by at.name
`

// SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE returns the count of animal with care per type
const SQL_ANIMAL_WITH_TODAY_TREATMENTS = `
SELECT a.* 
FROM animals a 
WHERE EXISTS(
	SELECT * 
	FROM treatments t 
	WHERE t.animal_id = a.id
	  AND t.timebitmap <> t.timedonebitmap
	  AND t.date >= ? 
	  AND t.date < ?) 
`

// SQL_ANIMAL_TOBE_FORCEFEED returns the animals than need force feeding
const SQL_ANIMAL_TOBE_FORCEFEED = `
SELECT a.* 
FROM animals a
WHERE outtake_id is null 
 AND force_feed is true
`

func listOpenCares(c buffalo.Context) ([]models.CareWithAnimalNumber, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	cares := []models.CareWithAnimalNumber{}
	// Retrieve all Cares from the DB
	if err := tx.RawQuery(SQL_CARES_IN_WARNING).All(&cares); err != nil {
		return nil, err
	}

	if _, err := EnrichCaresWithAnimalNumber(&cares, c); err != nil {
		return nil, err
	}

	return cares, nil
}

func listAnimalWithTodayTreatments(c buffalo.Context) (*models.Animals, error) {
	now := time.Now()
	nowDt := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	tmrDt := nowDt.AddDate(0, 0, 1)

	animals := &models.Animals{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	// Retrieve all animals with today treatments from the DB
	if err := tx.RawQuery(SQL_ANIMAL_WITH_TODAY_TREATMENTS, nowDt, tmrDt).All(animals); err != nil {
		return nil, err
	}
	return EnrichAnimals(animals, c)
}

func listAnimalWithForceFeed(c buffalo.Context) (*models.Animals, error) {
	animals := &models.Animals{}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	// Retrieve all animals with today treatments from the DB
	if err := tx.RawQuery(SQL_ANIMAL_TOBE_FORCEFEED).All(animals); err != nil {
		return nil, err
	}
	return EnrichAnimals(animals, c)
}

type listAnimalCountPerTypeReply struct {
	Name  string `db:"Name"`
	Count int    `db:"Count"`
}

func listAnimalCountPerType(c buffalo.Context) ([]listAnimalCountPerTypeReply, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ct := []listAnimalCountPerTypeReply{}
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE).All(&ct); err != nil {
		return nil, err
	}

	return ct, nil
}

func listLast24hLogEntries(c buffalo.Context) (models.Logentries, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	logentries := models.Logentries{}

	// Retrieve all Logentries from the DB
	yesterday := time.Now().AddDate(0, 0, -1)
	if err := tx.Eager().Where("updated_at >= ?", yesterday).All(&logentries); err != nil {
		return nil, err
	}

	return logentries, nil
}

// DashboardIndex default implementation.
func DashboardIndex(c buffalo.Context) error {
	wla, err := listAnimalWithWeightLoss(c)
	if err != nil {
		return err
	}
	c.Set("animalsWithWeightLoss", wla)

	oc, err := listOpenCares(c)
	if err != nil {
		return err
	}
	c.Set("openCares", oc)

	animals, err := listAnimalWithTodayTreatments(c)
	if err != nil {
		return err
	}
	c.Set("animalsToTreat", animals)

	animalsToForceFeed, err := listAnimalWithForceFeed(c)
	if err != nil {
		return err
	}
	c.Set("animalsToForceFeed", animalsToForceFeed)

	ct, err := listAnimalCountPerType(c)
	if err != nil {
		return err
	}
	c.Set("animalCountPerType", ct)
	// Add total
	totalAnimal := 0
	for _, cti := range ct {
		totalAnimal += cti.Count
	}
	c.Set("totalAnimalCount", totalAnimal)

	le, err := listLast24hLogEntries(c)
	if err != nil {
		return err
	}
	c.Set("lastLogentries", le)

	return responder.Wants("html", func(c buffalo.Context) error {
		return c.Render(http.StatusOK, r.HTML("dashboard/dashboard.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(oc))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(oc))
	}).Respond(c)
}
