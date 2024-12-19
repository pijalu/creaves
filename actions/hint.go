package actions

import (
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

const HINT_SPECIES_DETAILS = `SELECT distinct ns.status, ns.indication, ns.freeable, s.game, s.huntable
FROM native_statuses ns
JOIN species s ON s.native_status=ns.ID
where s.creaves_species = "%s"`

// HintSpeciesDetails default implementation.
func HintSpeciesDetails(c buffalo.Context) error {
	s := []struct {
		Status     string `json:"status" db:"status"`
		Freeable   bool   `json:"freeable" db:"freeable"`
		Indication string `json:"indication" db:"indication"`
		Game       bool   `json:"game" db:"game"`
		Huntable   bool   `json:"huntable" db:"huntable"`
	}{}

	q := c.Param("q")
	if len(q) == 0 {
		return c.Render(404, r.JSON(s))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query = tx.RawQuery(fmt.Sprintf(HINT_SPECIES_DETAILS, q))
	if err := query.All(&s); err != nil {
		return err
	}
	return c.Render(200, r.JSON(s))
}
