package grifts

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
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
