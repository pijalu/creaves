package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/x/responder"
)

// SQL_CARES_IN_WARNING lists all cares in warning
const SQL_CARES_IN_WARNING = `
select c.*
from cares c
where  c.type_id in (
	  select id from caretypes where warning = true 
 )
 and c.date > (
     select max(c2.date)
     from cares c2
     where c2.animal_id = c.animal_id
       and c2.type_id in (
            select id from caretypes where reset_warning is true 
       )
    )
 and c.animal_id in (
	 select id from animals where outtake_id is null
 )
`

//SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE returns the count of animal in care per type
const SQL_ANIMAL_COUNT_IN_CARE_PER_TYPE = `
select at.name as 'Name', count(1) as 'Count'
from animaltypes at, animals a
 WHERE a.outtake_id is NULL
   and a.animaltype_id = at.id
GROUP BY at.name
ORDER by at.name
`

func listOpenCares(c buffalo.Context) (models.Cares, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	cares := models.Cares{}
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(SQL_CARES_IN_WARNING).All(&cares); err != nil {
		return nil, err
	}

	return cares, nil
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
	oc, err := listOpenCares(c)
	if err != nil {
		return err
	}
	c.Set("openCares", oc)

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