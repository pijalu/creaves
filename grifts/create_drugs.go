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
		unit       string
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
				{animalType: "Raptor", dosage: "1.4", unit: "mg"},
				{animalType: "Bird", dosage: "1.4", unit: "mg"},
				{animalType: "Mammals", dosage: "0.28", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.14", unit: "mg"},
			},
		},

		{
			name:        "Tolfine",
			description: "inject / 48h",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.5", unit: "mg"},
				{animalType: "Bird", dosage: "0.5", unit: "mg"},
				{animalType: "Mammals", dosage: "0.1", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.05", unit: "mg"},
			},
		},

		{
			name:        "Ketodolor",
			description: "iv, im",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.3", unit: "mg"},
				{animalType: "Bird", dosage: "0.3", unit: "mg"},
				{animalType: "Mammals", dosage: "0.06", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.03", unit: "mg"},
			},
		},

		{
			name:        "Keytil",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.33", unit: "mg"},
				{animalType: "Bird", dosage: "0.33", unit: "mg"},
				{animalType: "Mammals", dosage: "0.066", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.033", unit: "mg"},
			},
		},

		{
			name:        "Moderin La 40 mg/ml",
			description: "im",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.05", unit: "mg"},
				{animalType: "Bird", dosage: "0.05", unit: "mg"},
				{animalType: "Mammals", dosage: "0.05", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.05", unit: "mg"},
			},
		},

		{
			name:        "Dexa-Ject 2 mg/ml",
			description: "iv, iartic",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.05", unit: "mg"},
				{animalType: "Bird", dosage: "0.05", unit: "mg"},
				{animalType: "Mammals", dosage: "0.05", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.05", unit: "mg"},
			},
		},

		{
			name:        "Peni-Kel 300",
			description: "Im 5 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.07", unit: "mg"},
				{animalType: "Bird", dosage: "0.07", unit: "mg"},
				{animalType: "Mammals", dosage: "0.07", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.07", unit: "mg"},
			},
		},

		{
			name:        "Amoxy-Kel 15 %",
			description: "Im 3 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.1", unit: "mg"},
				{animalType: "Bird", dosage: "0.1", unit: "mg"},
				{animalType: "Mammals", dosage: "0.1", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.1", unit: "mg"},
			},
		},

		{
			name:        "Eradia 125 mg/ml",
			description: " 2xdays  5 to 7 - 10 jours",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.2", unit: "mg"},
				{animalType: "Bird", dosage: "0.2", unit: "mg"},
				{animalType: "Mammals", dosage: "0.2", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.2", unit: "mg"},
			},
		},

		{
			name:        "Baytril 100 mg/ml",
			description: "3 to 5 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "", unit: "mg"},
				{animalType: "Bird", dosage: "0.1", unit: "mg"},
				{animalType: "Mammals", dosage: "0.1", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.1", unit: "mg"},
			},
		},

		{
			name:        "Baycox 2,5 %",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "", unit: "mg"},
				{animalType: "Bird", dosage: "0.28", unit: "mg"},
				{animalType: "Mammals", dosage: "", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.28", unit: "mg"},
			},
		},

		{
			name:        "Ivomec",
			description: "Repeat after 7 days",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "0.2", unit: "mg"},
				{animalType: "Bird", dosage: "0.2", unit: "mg"},
				{animalType: "Mammals", dosage: "0.2", unit: "mg"},
				{animalType: "Hedgehog", dosage: "0.2", unit: "mg"},
			},
		},

		{
			name:        "Capizol",
			description: "",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "", unit: "mg"},
				{animalType: "Bird", dosage: "1.33", unit: "mg"},
				{animalType: "Mammals", dosage: "", unit: "mg"},
				{animalType: "Hedgehog", dosage: "1.33", unit: "mg"},
			},
		},

		{
			name:        "Catosal 10 %",
			description: "diluate iv, im, sc",
			dosages: []dosage{
				{animalType: "Raptor", dosage: "2.5", unit: "mg"},
				{animalType: "Bird", dosage: "2.5", unit: "mg"},
				{animalType: "Mammals", dosage: "2.5", unit: "mg"},
				{animalType: "Hedgehog", dosage: "2.5", unit: "mg"},
			},
		},
	}

	cnt, err := models.DB.Q().Count(&models.Drug{})
	if err != nil {
		return err
	}
	if cnt > 0 {
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
				dsf /= 1000.0
				at, present := atm[ds.animalType]
				if !present {
					fmt.Printf("Animal type %s not found - skipping\n", ds.animalType)
					continue
					//return fmt.Errorf("Could not find animal type %s", ds.animalType)
				}
				_ = at
				dosage := models.Dosage{
					Drug:               d,
					AnimaltypeID:       at.ID,
					Animaltype:         &at,
					Enabled:            true,
					DosagePerGrams:     nulls.NewFloat64(dsf),
					DosagePerGramsUnit: nulls.NewString(ds.unit),
					Description:        nulls.NewString(t.description),
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
