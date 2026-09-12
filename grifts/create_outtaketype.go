package grifts

import (
	"database/sql"
	"fmt"

	"creaves/models"
	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

var canonicalOuttakeTypes = []struct {
	name, description string
	def, dead, err    bool
	rating            int
}{
	{"OT1", "Animal died", true, true, false, -1},
	{"OT2", "Released to the wild", false, false, false, 1},
	{"OT3", "Transferred to another centre", false, false, false, 1},
	{"OT4", "Euthanized", false, true, false, -1},
	{"OT5", "Lost", false, false, true, -1},
	{"OT6", "Stolen", false, false, true, -1},
	{"OT7", "Other outcome", false, false, false, 0},
}

// createOuttaketype installs the seven canonical outcome types. Upsert by name
// keeps existing ids (and dependent outtakes) stable while correcting fields.
func createOuttaketype(c *grift.Context) error {
	for _, t := range canonicalOuttakeTypes {
		row := &models.Outtaketype{}
		err := models.DB.Q().Where("name = ?", t.name).First(row)
		if err != nil {
			if err != sql.ErrNoRows {
				return err
			}
			row = &models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: t.name}
		}
		row.Name, row.Description = t.name, nulls.NewString(t.description)
		row.Default, row.Dead, row.Error, row.Rating = t.def, t.dead, t.err, t.rating
		if row.ID == uuid.Nil {
			if err := models.DB.Create(row); err != nil {
				return err
			}
		} else {
			verrs, err := models.DB.ValidateAndUpdate(row)
			if err != nil {
				return err
			}
			if verrs.HasAny() {
				return fmt.Errorf("outtake type %s invalid: %v", t.name, verrs)
			}
		}
		fmt.Printf("Upserted outtake type %s\n", t.name)
	}
	return nil
}
