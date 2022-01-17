package actions

import (
	"creaves/localrender"
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
	"github.com/gobuffalo/x/responder"
)

//SQL_GET_REGISTER_YEARS returns all the avl years for the register
const SQL_GET_REGISTER_YEARS = `
	select distinct a.year as 'Year'
	from animals a 
	order by 1 asc;
`

type registerYear struct {
	Year     string `db:"Year"`
	Selected bool
}

func listRegisterYears(c buffalo.Context) ([]registerYear, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ct := []registerYear{}
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(SQL_GET_REGISTER_YEARS).All(&ct); err != nil {
		return nil, err
	}

	return ct, nil
}

// RegistertableIndex default implementation.
func RegistertableIndexCSV(c buffalo.Context) error {
	return c.Render(http.StatusOK, localrender.Csv(r, "registertable/registertable.plush.csv"))
}

// RegistertableIndex default implementation.
func RegistertableIndex(c buffalo.Context) error {
	years, err := listRegisterYears(c)
	if err != nil {
		return err
	}
	selectedYear := ""
	y := c.Param("year")
	if y == "" {
		years[len(years)-1].Selected = true
		selectedYear = years[len(years)-1].Year
	} else {
		for i := 0; i < len(years); i++ {
			years[i].Selected = years[i].Year == y
			if years[i].Selected {
				selectedYear = years[i].Year
			}
		}
	}
	c.Set("years", years)

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := &models.Animals{}

	q := tx.PaginateFromParams(c.Params()).Where("year = ?", selectedYear)

	// Retrieve all Animals from the DB
	if err := q.Order("yearNumber desc").All(animals); err != nil {
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
		return c.Render(http.StatusOK, r.HTML("registertable/registertable.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animals))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animals))
	}).Respond(c)
}
