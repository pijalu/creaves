package grifts

import (
	"creaves/models"
	"fmt"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
)

func createCaretype(c *Context) error {
	ts := []struct {
		name         string
		description  string
		def          bool
		Warning      bool
		ResetWarning bool
	}{
		{name: "Care", description: "Animal had care given", def: true},
		{name: "Feeding", description: "Animal is fed"},
		{name: "Move", description: "Animal is moved"},
		{name: "Warning", description: "Attention needed", Warning: true},
		{name: "Warning Response", description: "Response to attention", ResetWarning: true},
	}

	cnt, err := models.DB.Q().Count(&models.Caretype{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in care types - skipping\n", cnt)
		return nil
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
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("care type %s already exists\n", t.name)
		}
	}
	return nil
}
