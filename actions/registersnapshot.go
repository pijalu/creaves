package actions

import (
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
		WHERE (i.id IS NOT NULL AND i.date < ?)
		AND (a.outtake_id IS NULL OR o.date >= ?)
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
	// Intake: on or before snapshotDate (exclusive upper bound = day after snapshotDate)
	// Outtake: on or after snapshotDate
	snapshotEnd := snapshotDateAsDate.AddDate(0, 0, 1)
	if err := tx.RawQuery(REGISTER_SNAP_SQL, snapshotEnd, snapshotDateAsDate).All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimalsOptimizedNoTreatments(animals, c); err != nil {
		return err
	}

	header := []string{
		"Numero", "Type", "Species", "identification", "Zone", "Cage",
		"Entry Date", "Discovery Location", "Age", "Reason",
	}
	rows := make([][]string, 0, len(*animals))
	for _, a := range *animals {
		rows = append(rows, []string{
			fmt.Sprintf("%d", a.YearNumber),
			a.Animaltype.Name,
			a.Species,
			a.Ring.String,
			a.Zone.String,
			a.Cage.String,
			a.Intake.DateFormated(),
			a.Discovery.Location.String,
			a.Animalage.Name,
			a.Discovery.Reason.String,
		})
	}

	return writeCSV(c, fmt.Sprintf("registersnapshot-%s.csv", snapshotDate), header, rows)
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
	// Intake: on or before snapshotDate (exclusive upper bound = day after snapshotDate)
	// Outtake: on or after snapshotDate
	snapshotEnd := snapshotDateAsDate.AddDate(0, 0, 1)
	if err := tx.RawQuery(REGISTER_SNAP_SQL, snapshotEnd, snapshotDateAsDate).All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimalsOptimizedNoTreatments(animals, c); err != nil {
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
