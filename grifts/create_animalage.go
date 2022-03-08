package grifts

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
)

func createAnimalage(c *Context) error {
	ts := []struct {
		name        string
		description string
		def         bool
	}{
		{name: "baby", description: ""},
		{name: "juvenile", description: ""},
		{name: "adult", description: "", def: true},
	}

	cnt, err := models.DB.Q().Count(&models.Animalage{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in animal ages - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Animalage{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating animal age %v\n", t)
			if err := models.DB.Create(&models.Animalage{
				Name:        t.name,
				Description: nulls.NewString(t.description),
				Default:     t.def,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("animalage %s already exists\n", t.name)
		}
	}
	return nil
}
