package actions

import (
	"creaves/models"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
)

// ReceptionNew default implementation.
func ReceptionNew(c buffalo.Context) error {
	n := time.Now()

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

	z, err := zones(c)
	if err != nil {
		return err
	}
	c.Set("selectZone", zonesToSelectables(z))

	a := &models.Animal{}

	a.Discovery.Date = n
	a.Discovery.Discoverer.Country = nulls.NewString("Belgique")
	a.Intake.Date = n

	if z, err := defZone(c); err == nil {
		a.Zone = nulls.NewString(z.Zone)
		c.Logger().Debugf(" zone set to %v", a.Zone)
	} else {
		c.Logger().Debugf("Could not retrieve default zone: %v", err)
	}

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
