package grifts

import (
	"creaves/models"
	"fmt"
	"strconv"

	"github.com/gobuffalo/nulls"
	. "github.com/markbates/grift/grift"
)

func animalTypeID(c *Context) (map[string]models.Animaltype, error) {
	m := map[string]models.Animaltype{}
	animalTypes := []models.Animaltype{}
	if err := models.DB.Q().All(&animalTypes); err != nil {
		return nil, err
	}
	for _, at := range animalTypes {
		m[at.Name] = at
	}
	return m, nil
}

func createDrugs(c *Context) error {
	type dosage struct {
		animalType string
		dosage     string
	}

	ts := []struct {
		name        string
		dosages     []dosage
		description string
	}{

		{
			name:        "Meloxoral 1,5 mg/ml",
			description: "Oral",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "1.4"},
				{animalType: "Bird", dosage: "1.4"},
				{animalType: "Mammals", dosage: "0.28"},
				{animalType: "Hedgehog", dosage: "0.14"},
			},
		},

		{
			name:        "Tolfine",
			description: "inject / 48h",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.5"},
				{animalType: "Bird", dosage: "0.5"},
				{animalType: "Mammals", dosage: "0.1"},
				{animalType: "Hedgehog", dosage: "0.05"},
			},
		},

		{
			name:        "Ketodolor",
			description: "iv, im",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.3"},
				{animalType: "Bird", dosage: "0.3"},
				{animalType: "Mammals", dosage: "0.06"},
				{animalType: "Hedgehog", dosage: "0.03"},
			},
		},

		{
			name:        "Keytil",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.33"},
				{animalType: "Bird", dosage: "0.33"},
				{animalType: "Mammals", dosage: "0.066"},
				{animalType: "Hedgehog", dosage: "0.033"},
			},
		},

		{
			name:        "Moderin La 40 mg/ml",
			description: "im",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.05"},
				{animalType: "Bird", dosage: "0.05"},
				{animalType: "Mammals", dosage: "0.05"},
				{animalType: "Hedgehog", dosage: "0.05"},
			},
		},

		{
			name:        "Dexa-Ject 2 mg/ml",
			description: "iv, iartic",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.05"},
				{animalType: "Bird", dosage: "0.05"},
				{animalType: "Mammals", dosage: "0.05"},
				{animalType: "Hedgehog", dosage: "0.05"},
			},
		},

		{
			name:        "Peni-Kel 300",
			description: "Im 5 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.07"},
				{animalType: "Bird", dosage: "0.07"},
				{animalType: "Mammals", dosage: "0.07"},
				{animalType: "Hedgehog", dosage: "0.07"},
			},
		},

		{
			name:        "Amoxy-Kel 15 %",
			description: "Im 3 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.1"},
				{animalType: "Bird", dosage: "0.1"},
				{animalType: "Mammals", dosage: "0.1"},
				{animalType: "Hedgehog", dosage: "0.1"},
			},
		},

		{
			name:        "Eradia 125 mg/ml",
			description: " 2xdays  5 to 7 - 10 jours",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.2"},
				{animalType: "Bird", dosage: "0.2"},
				{animalType: "Mammals", dosage: "0.2"},
				{animalType: "Hedgehog", dosage: "0.2"},
			},
		},

		{
			name:        "Baytril 100 mg/ml",
			description: "3 to 5 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: ""},
				{animalType: "Bird", dosage: "0.1"},
				{animalType: "Mammals", dosage: "0.1"},
				{animalType: "Hedgehog", dosage: "0.1"},
			},
		},

		{
			name:        "Baycox 2,5 %",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: ""},
				{animalType: "Bird", dosage: "0.28"},
				{animalType: "Mammals", dosage: ""},
				{animalType: "Hedgehog", dosage: "0.28"},
			},
		},

		{
			name:        "Ivomec",
			description: "Repeat after 7 jours ",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.2"},
				{animalType: "Bird", dosage: "0.2"},
				{animalType: "Mammals", dosage: "0.2"},
				{animalType: "Hedgehog", dosage: "0.2"},
			},
		},

		{
			name:        "Capizol",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: ""},
				{animalType: "Bird", dosage: "1.33"},
				{animalType: "Mammals", dosage: ""},
				{animalType: "Hedgehog", dosage: "1.33"},
			},
		},

		{
			name:        "Catosal 10 %",
			description: "diluate iv, im, sc",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "2.5"},
				{animalType: "Bird", dosage: "2.5"},
				{animalType: "Mammals", dosage: "2.5"},
				{animalType: "Hedgehog", dosage: "2.5"},
			},
		},
	}

	cnt, err := models.DB.Q().Count(&models.Drug{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in drugs - skipping\n", cnt)
		return nil
	}

	atm, err := animalTypeID(c)
	if err != nil {
		return err
	}

	for _, t := range ts {
		if exists, err := models.DB.Q().Where("name = ?", t.name).Exists(&models.Drug{}); err != nil {
			return err
		} else if !exists {
			d := &models.Drug{
				Name:        t.name,
				Description: nulls.NewString(t.description),
				Dosages:     []models.Dosage{},
			}

			for _, ds := range t.dosages {
				if len(ds.dosage) == 0 {
					continue
				}
				dsf, err := strconv.ParseFloat(ds.dosage, 64)
				if err != nil {
					return err
				}
				at, present := atm[ds.animalType]
				if !present {
					return fmt.Errorf("Could not find animal type %s", ds.animalType)
				}
				_ = at
				dosage := models.Dosage{
					Drug:           d,
					AnimaltypeID:   at.ID,
					Animaltype:     &at,
					Enabled:        true,
					DosagePerGrams: nulls.NewFloat64(dsf),
				}
				d.Dosages = append(d.Dosages, dosage)
			}
			fmt.Printf("Creating drug %v\n", d)
			if err := models.DB.Eager().Create(d); err != nil {
				return err
			}
		} else {
			fmt.Printf("Drug %s already exists\n", t.name)
		}
	}
	return nil
}
