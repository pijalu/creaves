package grifts

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
)

func createTraveltype(c *Context) error {
	ts := []struct {
		name        string
		description string
		def         bool
	}{
		{name: "center <-> vetenary", description: "travel from center to vet", def: true},
		{name: "sub <-> center", description: "travel from a subdiary to center"},
		{name: "discovery site <-> center", description: "travel from discovery site to center"},
		{name: "discovery site <-> vetenary", description: "traval from discovery site to vetenary"},
		{name: "Other", description: "Other"},
	}

	cnt, err := models.DB.Q().Count(&models.Traveltype{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in travel types - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Traveltype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating travel type %v\n", t)
			if err := models.DB.Create(&models.Traveltype{
				Name:        t.name,
				Description: nulls.NewString(t.description),
				Def:         t.def,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Travel type %s already exists\n", t.name)
		}
	}
	return nil
}
