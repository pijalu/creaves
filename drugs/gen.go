package main

import (
	//"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"text/template"
)

const header = `
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
`

const footer = `
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
				dsf /= 1000.0
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
					DosagePerGramsUnit: nulls.NewString(ds.unit),
					Description: nulls.NewString(t.description),
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
`

const gencode = `
{
	name:        "{{.drug}}",
	description: "{{.note}}",
	dosages: []dosage{
		{animalType: "Raptor", dosage: "{{.Raptor}}", unit: "{{.unit}}"},
		{animalType: "Bird", dosage: "{{.Bird}}", unit: "{{.unit}}"},
		{animalType: "Mammals", dosage: "{{.Mammals}}", unit: "{{.unit}}"},
		{animalType: "Hedgehog", dosage: "{{.Hedgehog}}", unit: "{{.unit}}"},
	},
},
`

func main() {
	t := template.Must(template.New("gencode").Parse(gencode))

	// Open the file
	csvfile, err := os.Open("drugs.csv")
	if err != nil {
		log.Fatalln("Couldn't open the csv file", err)
	}
	defer csvfile.Close()

	fmt.Print(header)

	r := csv.NewReader(csvfile)
	header := []string{}
	firstLine := true
	for {
		record, err := r.Read()
		if firstLine {
			firstLine = false
			header = record
			continue
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		data := map[string]string{}
		for idx, name := range header {
			data[name] = record[idx]
		}
		if err := t.Execute(os.Stdout, data); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Print(footer)
}
