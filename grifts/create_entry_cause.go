package grifts

import (
	"creaves/models"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"strings"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

func createEntryCause(c *Context) error {
	ts := []models.EntryCause{}

	// Open the file
	csvfile, err := embedData.Open("create_entry_cause.csv")
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

		ts = append(ts, models.EntryCause{
			ID:         data["ID"],
			Cause:      data["cause"],
			Detail:     data["detail"],
			Nature:     data["nature"],
			Indication: data["indication"],
		})
	}

	cnt, err := models.DB.Q().Count(&models.EntryCause{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in entrycause - skipping\n", cnt)
		return nil
	} else {
		fmt.Printf("Already %d records in entrycause - expecting %d\n", cnt, len(ts))
	}

	return models.DB.Transaction(func(con *pop.Connection) error {
		for _, t := range ts {
			fmt.Printf("Working on %v\n", t)
			if len(t.ID) == 0 {
				continue
			}

			if exists, err := con.Where("ID = ?", t.ID).Exists(&models.EntryCause{}); err != nil {
				return err
			} else if !exists {
				if err := con.Create(&t); err != nil {
					return err
				}
			} else {
				fmt.Printf("EntryCause %s already exists - updating\n", t.ID)
				d_db := &models.EntryCause{}
				if err := con.Where("ID = ?", t.ID).First(d_db); err != nil {
					return err
				} else {
					// update record
					d_db.ID = t.ID
					d_db.Cause = t.Cause
					d_db.Detail = t.Detail
					d_db.Nature = t.Nature
					d_db.Indication = t.Indication

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
