package actions

import (
	"creaves/models"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
)

// ReceptionNew default implementation.
func ReceptionNew(c buffalo.Context) error {
	n := time.Now()

	at, err := animalTypes(c)
	if err != nil {
		return err
	}
	c.Set("selectAnimalTypes", animalTypesToSelectables(at))

	a := &models.Animal{}

	a.Discovery.Date = n
	a.Intake.Date = n

	// Set default animal type
	for _, t := range *at {
		if t.Default {
			a.Animaltype = t
			a.AnimaltypeID = t.ID
			c.Logger().Debugf("Set default type to %v", t)
			break
		}
	}
	c.Logger().Debugf("Created blank animal: %v", a)
	c.Set("animal", a)

	return c.Render(http.StatusOK, r.HTML("reception/new.plush.html"))
}
