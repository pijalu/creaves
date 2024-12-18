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

	. "github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
)

func createSpecies(c *Context) error {
	ts := []struct {
		ID			   string
		Species        string   
		CreavesSpecies string      
		Class          string 
		Order          string        
		Family         string
		NativeStatus   string
		AgwGroup       string
		SubsideGroup   string 
		Game	       bool
		Huntable       bool
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
			if len(t.Species) == 0 {
				continue
			}

			d := &models.Species{
				ID: 			t.ID,
				Species:        t.Species,
				CreavesSpecies: t.CreavesSpecies,
				Class:          t.Class,
				Order:          t.Order,
				Family:         t.Family,
				NativeStatus:   t.NativeStatus,
				AgwGroup:   	t.AgwGroup,
				SubsideGroup:   t.SubsideGroup,
				Game:	        t.Game,
				Huntable:	    t.Huntable,
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
					d_db.ID = t.ID
					d_db.CreavesSpecies = t.CreavesSpecies
					d_db.Class = t.Class
					d_db.Order = t.Order
					d_db.Family = t.Family
					d_db.NativeStatus = t.NativeStatus
					d_db.AgwGroup = t.AgwGroup
					d_db.SubsideGroup = t.SubsideGroup
					d_db.Game = t.Game
					d_db.Huntable = t.Huntable
					
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
	ID			   : "{{ .ID }}",
	Species        : "{{ .Species }}",
	CreavesSpecies : "{{ .CreavesSpecies }}",
	Class          : "{{ .Class }}", 
	Order          : "{{ .Order }}", 
	Family         : "{{ .Family }}",
	NativeStatus   : "{{ .NativeStatus }}",
	AgwGroup   	   : "{{ .AgwGroup }}",
	SubsideGroup   : "{{ .SubsideGroup }}",
	Game           : 1 == {{ .Game }},
	Huntable       : 1 == {{ .Huntable }},
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
		if len(strings.TrimSpace(record[0])) == 0 {
			continue
		}
		if err := t.Execute(os.Stdout, data); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Print(footer)
}
