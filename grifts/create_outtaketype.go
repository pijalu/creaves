package grifts

import (
	"database/sql"
	"errors"
	"fmt"

	"creaves/models"
	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

var canonicalOuttakeTypes = []struct {
	code, name, description string
	def, dead, err          bool
	rating                  int
}{
	{"OT1", "DCD", "Animal died", true, true, false, -1},
	{"OT2", "Relacher", "Released to the wild", false, false, false, 1},
	{"OT3", "Transferer", "Transferred to another centre", false, false, false, 1},
	{"OT4", "Euthanasier", "Euthanized", false, true, false, -1},
	{"OT5", "Lost", "Lost", false, false, true, -1},
	{"OT6", "Stolen", "Stolen", false, false, true, -1},
	{"OT7", "Other outcome", "Other outcome", false, false, false, 0},
}

// createOuttaketype assigns stable OT codes to existing semantic rows and
// creates only missing named types; it never replaces legacy names with codes.
func createOuttaketype(c *grift.Context) error {
	if err := models.DB.RawQuery("UPDATE outtaketypes SET code = NULL WHERE code IN ('OT1','OT2','OT3','OT4','OT5','OT6','OT7') AND name NOT IN ('DCD','Relacher','Transferer','Euthanasier','Lost','Stolen','Other outcome')").Exec(); err != nil {
		return err
	}
	for _, t := range canonicalOuttakeTypes {
		row := &models.Outtaketype{}
		err := models.DB.Q().Where("name = ?", t.name).First(row)
		found := err == nil
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if !found {
			row = &models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: t.name}
		}
		row.Code = nulls.NewString(t.code)
		row.Description = nulls.NewString(t.description)
		row.Default, row.Dead, row.Error, row.Rating = t.def, t.dead, t.err, t.rating
		if !found {
			if err := models.DB.Create(row); err != nil {
				return err
			}
		} else {
			verrs, err := models.DB.ValidateAndUpdate(row)
			if err != nil {
				return err
			}
			if verrs.HasAny() {
				return fmt.Errorf("outtake type %s invalid: %v", t.code, verrs)
			}
		}
		fmt.Printf("Upserted outtake type %s (%s)\n", t.code, t.name)
	}
	return nil
}
