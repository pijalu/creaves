package actions

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

const HINT_NATIVESPECIES = `SELECT distinct ns.*
FROM native_statuses ns
JOIN species s ON s.native_status=ns.ID
where s.creaves_species = "%s"`

// HintNativeStatusBySpecies default implementation.
func HintNativeStatusBySpecies(c buffalo.Context) error {
	s := []models.NativeStatus{}

	q := c.Param("q")
	if len(q) == 0 {
		return c.Render(404, r.JSON(s))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query = tx.RawQuery(fmt.Sprintf(HINT_NATIVESPECIES, q))
	if err := query.All(&s); err != nil {
		return err
	}
	return c.Render(200, r.JSON(s))
}
