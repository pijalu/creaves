package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
)

func extendCaretype(c *Context) error {
	ts := []struct {
		name         string
		description  string
		def          bool
		Warning      bool
		ResetWarning bool
		Type         int
	}{
		{name: "Repas", description: "Animal got food", Type: 1},
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Caretype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating care type %v\n", t)
			if err := models.DB.Create(&models.Caretype{
				Name:         t.name,
				Description:  nulls.NewString(t.description),
				Def:          t.def,
				Warning:      t.Warning,
				ResetWarning: t.ResetWarning,
				Type:         t.Type,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("care type %s already exists\n", t.name)
		}
	}
	return nil
}
