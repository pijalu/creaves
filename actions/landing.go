package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/x/responder"
)

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
		c.Set("animalsByType", animalsByType)
		return c.Render(http.StatusOK, r.HTML("landing/index.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animalsByType))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animalsByType))
	}).Respond(c)
}
