package actions

import (
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
)

func suggest(c buffalo.Context, table string, field string) error {
	s := []string{}

	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := "SELECT DISTINCT " + field + " FROM " + table

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" WHERE "+field+" like ?", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsAnimalSpecies default implementation.
func SuggestionsAnimalSpecies(c buffalo.Context) error {
	return suggest(c, "animals", "species")
}

// SuggestionsDiscoveryLocation default implementation.
func SuggestionsDiscoveryLocation(c buffalo.Context) error {
	return suggest(c, "discoveries", "location")
}

// SuggestionsOuttakeLocation default implementation.
func SuggestionsOuttakeLocation(c buffalo.Context) error {
	return suggest(c, "outtakes", "location")
}

// SuggestionsDiscovererCity default implementation.
func SuggestionsDiscovererCity(c buffalo.Context) error {
	return suggest(c, "discoverers", "city")
}

// SuggestionsDiscovererCountry default implementation.
func SuggestionsDiscovererCountry(c buffalo.Context) error {
	return suggest(c, "discoverers", "country")
}
