package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
)

func createZones(c *Context) error {
	ts := []struct {
		zone      string
		zone_type string
		def       bool
	}{
		{zone: "Centre", zone_type: "zone interne au centre", def: true},
		{zone: "Cabinet vétérinaire", zone_type: "zone externe au centre", def: false},
		{zone: "Soft-release", zone_type: "zone externe au centre", def: false},
		{zone: "Famille d'accueil", zone_type: "zone externe au centre", def: false},
	}

	cnt, err := models.DB.Q().Count(&models.Zone{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in zones - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("zone = ?", t.zone).Exists(&models.Zone{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating travel type %v\n", t)
			if err := models.DB.Create(&models.Zone{
				Zone:    t.zone,
				Type:    t.zone_type,
				Default: t.def,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Travel type %s already exists\n", t.zone)
		}
	}
	return nil
}
