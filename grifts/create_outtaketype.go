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
	{"OT1", "Relacher", "Animal réhabilité et remis en liberté dans son milieu naturel.", false, false, false, 1},
	{"OT2", "DCD", "Animal décédé naturellement durant la prise en charge.", true, true, false, -1},
	{"OT3", "Euthanasier", "Animal euthanasié en raison de lésions ou d’un état incompatible avec une remise en liberté.", false, true, false, -1},
	{"OT4", "Transferer", "Transfert de l'animal vers un: refuge, CREAVES, VOC, ZOO, ...", false, false, false, 1},
	{"OT5", "Mort à l'arrivée avant l'encodage", "Animal arrivé décédé avant l'encodage ou la prise en charge.", false, true, false, -1},
	{"OT6", "Adoption", "Animal placé en captivité autorisée car espèce non indigène.", false, false, false, 0},
	{"OT7", "Doublon", "Fiche créée en double pour le même animal.", false, false, true, -1},
}

// createOuttaketype assigns stable OT codes to existing semantic rows and
// creates only missing named types; it never replaces legacy names with codes.
func createOuttaketype(c *grift.Context) error {
	if err := models.DB.RawQuery("UPDATE outtaketypes SET code = NULL WHERE code IN ('OT1','OT2','OT3','OT4','OT5','OT6','OT7') AND name NOT IN ('Relacher','DCD','Euthanasier','Transferer','Mort à l''arrivée avant l''encodage','Adoption','Doublon')").Exec(); err != nil {
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
