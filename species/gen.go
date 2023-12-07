package main

import (
	//"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"text/template"
)

const header = `
package grifts

import (
	"creaves/models"
	"fmt"
	"strconv"

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
)

func createSpecies(c *Context) error {
	ts := []struct {
		Species        string        
		Group          string        
		Family         string       
		CreavesSpecies string        
		CreavesGroup   string        
		Subside        string
	}{
`

const footer = `
	}
	
	cnt, err := models.DB.Q().Count(&models.Species{})
	if err != nil {
		return err
	}
	if cnt >= len(ts) {
		fmt.Printf("Already %d records in species - skipping\n", cnt)
		return nil
	} else {
		fmt.Printf("Already %d records in species - expecting %d\n", cnt, len(ts))
	}


	return models.DB.Transaction(func(con *pop.Connection) error {
		for _, t := range ts {
			d := &models.Species{
				Species:        t.Species,
				Group:          t.Group,
				Family:         t.Family,
				CreavesSpecies: t.CreavesSpecies,
				CreavesGroup:   t.CreavesGroup,
			}
			if len(t.Subside) > 0 && t.Subside != "?" {
				dsf, err := strconv.ParseFloat(t.Subside, 64)
				if err == nil {
					d.Subside = nulls.NewFloat64(dsf)
				} else {
					fmt.Printf("Error parsing subside %s: %v", t.Subside, err)
				}
			}

			if exists, err := con.Where("Species = ?", t.Species).Exists(&models.Species{}); err != nil {
				return err
			} else if !exists {
				if err := con.Create(d); err != nil {
					return err
				}
			} else {
				fmt.Printf("Species %s already exists - updating\n", t.Species)
				d_db := &models.Species{}
				if err := con.Where("Species = ?", t.Species).First(d_db); err != nil {
					fmt.Printf("Failure to load: %s - record corrupted... Removing", t.Species)
					if err := con.RawQuery("delete from species where species = ?", t.Species).Exec(); err != nil {
						return err
					}
					fmt.Printf("Recreating %s", t.Species)
					if err := con.Create(d); err != nil {
						return err
					}
				} else {
					// update record
					d_db.Group = d.Group
					d_db.Family = d.Family
					d_db.CreavesSpecies = d.CreavesSpecies
					d_db.CreavesGroup = d.CreavesGroup
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
`

const gencode = `
{
	Species        : "{{ .Species }}",
	Group          : "{{ .Group }}", 
	Family         : "{{ .Family }}",
	CreavesSpecies : "{{ .CreavesSpecies }}",
	CreavesGroup   : "{{ .CreavesGroup }}",        
	Subside        : "{{ .Subside }}",
},
`

func main() {
	t := template.Must(template.New("gencode").Parse(gencode))

	// Open the file
	csvfile, err := os.Open("species.csv")
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
			data[name] = strings.TrimSpace(record[idx])
		}
		if err := t.Execute(os.Stdout, data); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Print(footer)
}
