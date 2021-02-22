package grifts

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
)

func createAnimaltypes(c *Context) error {
	ts := []struct {
		name        string
		description string
	}{
		{name: "Hedgehog", description: ""},
		{name: "Raptor", description: ""},
		{name: "Mammals", description: ""},
		{name: "Bird", description: ""},
		{name: "Other", description: ""},
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Animaltype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating animaltype %v\n", t)
			if err := models.DB.Create(&models.Animaltype{
				Name:        t.name,
				Description: nulls.NewString(t.description),
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("animaltype %s already exists\n", t.name)
		}
	}
	return nil
}
