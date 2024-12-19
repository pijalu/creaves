package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
)

func createSubsideGroup(c *Context) error {
	ts := []struct {
		ID     string
		Group  string
		Size   int
		Amount float64
	}{
		{ID: "SG1", Group: "Rapaces, oiseaux d'eau, échassiers ou limicoles", Size: 50, Amount: 1250},
		{ID: "SG2", Group: "Autres oiseaux et chauves-souris", Size: 100, Amount: 1250},
		{ID: "SG3", Group: "Mammifères non volants", Size: 100, Amount: 3000},
		{ID: "SG4", Group: "Non finançable", Size: 0, Amount: 0},
	}

	cnt, err := models.DB.Q().Count(&models.SubsideGroup{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in subside group - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("ID = ?", t.ID).Exists(&models.SubsideGroup{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating Subside group %v\n", t)
			if err := models.DB.Create(&models.SubsideGroup{
				ID:     t.ID,
				Group:  t.Group,
				Size:   t.Size,
				Amount: t.Amount,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Subside group %s already exists\n", t.ID)
		}
	}
	return nil
}
