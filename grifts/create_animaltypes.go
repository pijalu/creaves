package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
)

func createAnimaltypes(c *Context) error {
	ts := []struct {
		name        string
		description string
		def         bool
		HasRing     bool
	}{
		{name: "Hedgehog", description: "", HasRing: false},
		{name: "Raptor", description: "", HasRing: true},
		{name: "Mammals", description: "", HasRing: false},
		{name: "Bird", description: "", HasRing: true},
		{name: "Other", description: "", def: true},
	}

	cnt, err := models.DB.Q().Count(&models.Animaltype{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in animal types - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Animaltype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating animaltype %v\n", t)
			if err := models.DB.Create(&models.Animaltype{
				Name:        t.name,
				Description: nulls.NewString(t.description),
				Default:     t.def,
				HasRing:     t.HasRing,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("animaltype %s already exists\n", t.name)
		}
	}
	return nil
}
