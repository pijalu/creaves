package grifts

import (
	"creaves/models"
	"embed"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strings"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"

	_ "embed"
)

//go:embed create_locality.csv
var localityData embed.FS

func createLocality(c *Context) error {
	ts := []models.Locality{}

	// Open the file
	csvfile, err := localityData.Open("create_locality.csv")
	if err != nil {
		log.Fatalln("Couldn't open the csv file", err)
	}
	defer csvfile.Close()

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
			data[name] = strings.TrimSpace(record[idx])
		}
		if len(strings.TrimSpace(record[0])) == 0 {
			continue
		}

		ts = append(ts, models.Locality{
			ID:              data["ID"],
			Country:         data["Country"],
			Region:          data["Region"],
			Province:        data["Province"],
			Municipality:    data["Municipality"],
			SubMunicipality: data["SubMunicipality"] == "1",
			PostalCode:      data["PostalCode"],
			Locality:        data["Locality"],
			Zoning:          data["Zoning"],
			Direction:       data["Direction"],
		})
	}

	cnt, err := models.DB.Q().Count(&models.Locality{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in localities - skipping\n", cnt)
		return nil
	} else {
		fmt.Printf("Already %d records in localities - expecting %d\n", cnt, len(ts))
	}

	return models.DB.Transaction(func(con *pop.Connection) error {
		for _, t := range ts {
			fmt.Printf("Working on %v\n", t)
			if len(t.Locality) == 0 {
				continue
			}

			if exists, err := con.Where("ID = ?", t.ID).Exists(&models.Locality{}); err != nil {
				return err
			} else if !exists {
				if err := con.Create(&t); err != nil {
					return err
				}
			} else {
				fmt.Printf("Locality %s already exists - updating\n", t.ID)
				d_db := &models.Locality{}
				if err := con.Where("ID = ?", t.ID).First(d_db); err != nil {
					return err
				} else {
					// update record
					d_db.ID = t.ID
					d_db.Country = t.Country
					d_db.Region = t.Region
					d_db.Province = t.Province
					d_db.Municipality = t.Municipality
					d_db.SubMunicipality = t.SubMunicipality
					d_db.PostalCode = t.PostalCode
					d_db.Locality = t.Locality
					d_db.Zoning = t.Zoning
					d_db.Direction = t.Direction

					if err := con.Update(d_db); err != nil {
						fmt.Printf("Failure to save: %v", d_db)
						return err
					}
				}
			}
		}
		return nil
	})
}
