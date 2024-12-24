package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
)

func createRequiredOuttaketype(c *Context) error {
	ts := []struct {
		Name    string
		Default bool
		Dead    bool
		Error   bool
		Rating  int
	}{
		{Name: "Doublon", Default: false, Dead: false, Error: true, Rating: -1},
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.Name).Exists(&models.Outtaketype{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating required outtake type group %v\n", t)
			if err := models.DB.Create(&models.Outtaketype{
				Name:    t.Name,
				Default: t.Default,
				Dead:    t.Dead,
				Error:   t.Error,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("required outtake type %s already exists\n", t.Name)
		}
	}
	return nil
}
