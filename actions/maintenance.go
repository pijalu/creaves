package actions

import (
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v5"
)

// MaintenanceInde default implementation.
func MaintenanceIndex(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required for this action"))
	}
	return c.Render(http.StatusOK, r.HTML("maintenance/index.plush.html"))
}

// MaintenanceRenumber default implementation.
func MaintenanceRenumber(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("Admin rights required for this action"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	if err := tx.RawQuery("update animals set year = null, yearNumber = null").Exec(); err != nil {
		return err
	}
	if err := tx.RawQuery("update animals set year = YEAR(IntakeDate);").Exec(); err != nil {
		return err
	}

	years := []int{}
	if err := tx.RawQuery("SELECT DISTINCT year FROM animals ORDER BY year asc").All(&years); err != nil {
		return err
	}

	for _, year := range years {
		if err := tx.RawQuery("SELECT @i:=0").Exec(); err != nil {
			return err
		}
		if err := tx.RawQuery(`
			UPDATE animals a
			set a.yearNumber = @i:=@i+1 
			where a.year=? 
			order by a.intakeDate asc;`, year).Exec(); err != nil {
			return err
		}

	}

	return c.Render(http.StatusOK, r.HTML("maintenance/renumber.plush.html"))
}
