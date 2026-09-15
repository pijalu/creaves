package grifts

import (
	"database/sql"
	"errors"
	"fmt"

	"creaves/models"

	grift "github.com/gobuffalo/grift/grift"
	"github.com/gofrs/uuid"
)

// outtakeLocationOptionSeeds: stable UUIDs keep the reference translations
// keyed by record ID across reseeds. Base (canonical) names stay French.
var outtakeLocationOptionSeeds = []struct {
	id   string
	name string
}{
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b01", "Creaves"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b02", "Refuge"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b03", "VOC"},
	{"7e9b6f4e-7a4c-4f6b-9a1d-0c1e2f3a4b04", "Zoo"},
}

// createOuttakeLocationOptions seeds the outtake_location_options reference
// list (used by outcome types with location_mode = "list"). Idempotent.
func createOuttakeLocationOptions(c *grift.Context) error {
	for _, s := range outtakeLocationOptionSeeds {
		row := &models.OuttakeLocationOption{}
		err := models.DB.Q().Where("name = ?", s.name).First(row)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil {
			fmt.Printf("outtake location option %s already exists\n", s.name)
			continue
		}
		fmt.Printf("Creating outtake location option %s\n", s.name)
		if err := models.DB.Create(&models.OuttakeLocationOption{
			ID:   uuid.Must(uuid.FromString(s.id)),
			Name: s.name,
		}); err != nil {
			return err
		}
	}
	return nil
}
