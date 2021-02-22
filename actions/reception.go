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

	a := &models.Animal{}
	a.Discovery.Date = n
	a.Intake.Date = n
	c.Set("animal", a)

	return c.Render(http.StatusOK, r.HTML("reception/new.plush.html"))
}
