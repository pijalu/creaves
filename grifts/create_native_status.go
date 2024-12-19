package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
)

func createNativeStatus(c *Context) error {
	ts := []struct {
		ID         string
		Status     string
		Indication string
		Freeable   bool
		Precision  nulls.String
	}{
		{ID: "NS1", Status: "Indigène", Indication: "L'animal doit être relâché dans son milieu", Freeable: true, Precision: nulls.String{}},
		{ID: "NS2", Status: "Exotique", Indication: "L'animal est transféré en refuge, Parcs ou centres faune sauvage.", Freeable: false, Precision: nulls.NewString("Doit être transferé dans un parc ou refuge pour les individus nés en captivité et pour les individus nés à l'état sauvage: transfert dans un centre de leurs aires d'indigénat (Art. 5/1 e)")},
		{ID: "NS3", Status: "Exotique préoccupant", Indication: "L'animal doit être euthanasié.", Freeable: false, Precision: nulls.NewString("Conformément à la législation européenne")},
		{ID: "NS4", Status: "Domestique", Indication: "L'animal est transféré en refuge, Parcs et particuliers.", Freeable: false, Precision: nulls.NewString("(Art. 5/1 e)")},
	}

	cnt, err := models.DB.Q().Count(&models.NativeStatus{})
	if err != nil {
		return err
	}
	if cnt > 0 {
		fmt.Printf("Already %d records in native status - skipping\n", cnt)
		return nil
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("ID = ?", t.ID).Exists(&models.NativeStatus{}); err != nil {
			return err
		} else if !exists {
			fmt.Printf("Creating travel type %v\n", t)
			if err := models.DB.Create(&models.NativeStatus{
				ID:         t.ID,
				Status:     t.Status,
				Indication: t.Indication,
				Freeable:   t.Freeable,
				Precision:  t.Precision,
			}); err != nil {
				return err
			}
		} else {
			fmt.Printf("Native_status %s already exists\n", t.ID)
		}
	}
	return nil
}
