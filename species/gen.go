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
	}


	return models.DB.Transaction(func(con *pop.Connection) error {
		for _, t := range ts {
			if exists, err := con.Where("Species = ?", t.Species).Exists(&models.Species{}); err != nil {
				return err
			} else if !exists {
				d := &models.Species{
					Species:        t.Species,
					Group:          t.Group,
					Family:         t.Family,
					CreavesSpecies: t.CreavesSpecies,
					CreavesGroup:   t.CreavesGroup,
				}
				if len(t.Subside) > 0 {
					dsf, err := strconv.ParseFloat(t.Subside, 64)
					if err == nil {
						d.Subside = nulls.NewFloat64(dsf)
					} else {
						fmt.Printf("Error parsing subside %s: %v", t.Subside, err)
					}
				}
				if err := con.Create(d); err != nil {
					return err
				}
			} else {
				fmt.Printf("Species %s already exists\n", t.Species)
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
