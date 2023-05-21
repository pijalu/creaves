package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

type listAnimalWithCleanCageReply struct {
	Id int `db:"ID"`
}

const SQL_ANIMAL_WITH_CLEAN_CAGE = `
	SELECT DISTINCT c.animal_id as 'ID'
	FROM cares c
	WHERE c.clean = 1
		AND c.date >= DATE_ADD(CURDATE(), INTERVAL 2 HOUR)
`

// List all animals id with a clean cage within the last 24h
func listAnimalWithCleanCage(c buffalo.Context) (map[int]bool, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	var a []listAnimalWithCleanCageReply
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(SQL_ANIMAL_WITH_CLEAN_CAGE).All(&a); err != nil {
		return nil, err
	}

	//remap
	res := map[int]bool{}

	for _, item := range a {
		res[item.Id] = true
	}

	return res, nil
}

// LandingIndex is the default landing view
func LandingIndex(c buffalo.Context) error {
	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := models.Animals{}
	if err := tx.Where("outtake_id is null").Order("ID desc").All(&animals); err != nil {
		return c.Error(http.StatusNoContent, err)
	}

	animalsByType := models.AnimalsByTypeMap{}
	if _, err := EnrichAnimals(&animals, c); err != nil {
		return err
	}

	for _, animal := range animals {
		animalsByType[animal.Animaltype] = append(animalsByType[animal.Animaltype], animal)
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add clean cage flag
		animalWithCleanCage, err := listAnimalWithCleanCage(c)
		if err != nil {
			return err
		}
		c.Set("animalsWithCleanCage", animalWithCleanCage)
		c.Set("animalsByType", animalsByType)
		return c.Render(http.StatusOK, r.HTML("landing/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animalsByType))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animalsByType))
	}).Respond(c)
}
