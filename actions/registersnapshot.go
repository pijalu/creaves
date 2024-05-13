package actions

import (
	"creaves/localrender"
	"creaves/models"
	"fmt"
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

const REGISTER_SNAP_SQL = `
		SELECT DISTINCT a.*
		FROM animals a
		LEFT JOIN intakes i ON a.intake_id = i.id
		LEFT JOIN outtakes o ON a.outtake_id = o.id
		WHERE (i.id IS NOT NULL AND date(i.date) <= date(?))
		AND (a.outtake_id IS NULL or date(o.date) >= date(?))
		ORDER BY a.id DESC
		LIMIT 2000
`

// RegistertableIndex default implementation.
func RegistersnapshotIndexCSV(c buffalo.Context) error {
	snapshotDate := time.Now().Format("2006/01/02")
	y := c.Param("snapshotDate")
	if y != "" {
		snapshotDate = y
	}
	c.Set("snapshotDate", snapshotDate)

	snapshotDateAsDate, err := time.Parse("2006/01/02", snapshotDate)
	if err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := &models.Animals{}
	// Retrieve all animals
	if err := tx.RawQuery(REGISTER_SNAP_SQL, snapshotDateAsDate, snapshotDateAsDate).All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimals(animals, c); err != nil {
		return err
	}

	c.Set("animals", animals)
	return c.Render(http.StatusOK, localrender.Csv(r, "registersnapshot/registersnapshot.plush.csv"))
}

// RegistersnapshotIndex default implementation.
func RegistersnapshotIndex(c buffalo.Context) error {

	snapshotDate := time.Now().Format("2006/01/02")
	y := c.Param("snapshotDate")
	if y != "" {
		snapshotDate = y
	}
	c.Set("snapshotDate", snapshotDate)

	snapshotDateAsDate, err := time.Parse("2006/01/02", snapshotDate)
	if err != nil {
		return err
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := &models.Animals{}
	// Retrieve all animals
	if err := tx.RawQuery(REGISTER_SNAP_SQL, snapshotDateAsDate, snapshotDateAsDate).All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimals(animals, c); err != nil {
		return err
	}

	return responder.Wants("html", func(c buffalo.Context) error {
		c.Set("animals", animals)
		return c.Render(http.StatusOK, r.HTML("registersnapshot/registersnapshot.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animals))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animals))
	}).Respond(c)
}
