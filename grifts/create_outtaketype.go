package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
)

func createOuttaketype(c *Context) error {
	ts := []struct {
		name        string
		description string
		def         bool
	}{
		{name: "Dead", description: "animal died", def: true},
		{name: "Transfer", description: "animal is transfered"},
		{name: "Freed", description: "animal is freed"},
	}

	cnt, err := models.DB.Q().Count(&models.Outtaketype{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in outtake types - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Outtaketype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating outtake type %v\n", t)
			if err := models.DB.Create(&models.Outtaketype{
				Name:        t.name,
				Description: nulls.NewString(t.description),
				Default:     t.def,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Outtake type %s already exists\n", t.name)
		}
	}
	return nil
}
