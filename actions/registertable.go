package actions

import (
	"creaves/models"
	"fmt"
	"net/http"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/x/responder"
)

// SQL_GET_REGISTER_YEARS returns all the avl years for the register
const SQL_GET_REGISTER_YEARS = `
	select distinct a.year as 'Year'
	from animals a 
	order by 1 asc;
`

type registerYear struct {
	Year     string `db:"Year"`
	Selected bool
}

// speciesClassMap returns creaves_species → class (Mammalia, Aves, …) for
// the given distinct species names (#197 sub-item 5: register Type→Class).
// Missing species (free text) map to nothing.
func speciesClassMap(tx *pop.Connection, names []string) map[string]string {
	out := map[string]string{}
	if len(names) == 0 {
		return out
	}
	var rows []models.Species
	if err := tx.Where("creaves_species IN (?)", names).All(&rows); err != nil {
		return out
	}
	for _, s := range rows {
		out[s.CreavesSpecies] = s.Class
	}
	return out
}

// entryCauseDetailMap returns entry_causes.id → detail for the given ids
// (#197 sub-item 5: register entry-cause id + detail columns).
func entryCauseDetailMap(tx *pop.Connection, ids []string) map[string]string {
	out := map[string]string{}
	if len(ids) == 0 {
		return out
	}
	var rows []models.EntryCause
	if err := tx.Where("id IN (?)", ids).All(&rows); err != nil {
		return out
	}
	for _, r := range rows {
		out[r.ID] = r.Detail
	}
	return out
}

// registerLookupMaps builds the class/detail lookup maps used by the register
// page and its CSV export.
func registerLookupMaps(tx *pop.Connection, animals *models.Animals) (map[string]string, map[string]string) {
	seenSpecies := map[string]struct{}{}
	speciesNames := []string{}
	seenCauses := map[string]struct{}{}
	causeIDs := []string{}
	for _, a := range *animals {
		if a.Species != "" {
			if _, dup := seenSpecies[a.Species]; !dup {
				seenSpecies[a.Species] = struct{}{}
				speciesNames = append(speciesNames, a.Species)
			}
		}
		if a.Discovery.EntryCauseID != "" {
			if _, dup := seenCauses[a.Discovery.EntryCauseID]; !dup {
				seenCauses[a.Discovery.EntryCauseID] = struct{}{}
				causeIDs = append(causeIDs, a.Discovery.EntryCauseID)
			}
		}
	}
	return speciesClassMap(tx, speciesNames), entryCauseDetailMap(tx, causeIDs)
}

func listRegisterYears(c buffalo.Context) ([]registerYear, error) {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil, fmt.Errorf("no transaction found")
	}

	ct := []registerYear{}
	// Retrieve all Cares from the DB
	if err := tx.Eager().RawQuery(SQL_GET_REGISTER_YEARS).All(&ct); err != nil {
		return nil, err
	}

	return ct, nil
}

// RegistertableIndex default implementation.
func RegistertableIndexCSV(c buffalo.Context) error {
	y := c.Param("year")
	if y == "" {
		return fmt.Errorf("year not provided found")
	}

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := &models.Animals{}

	// Retrieve all Animals from the DB
	if err := tx.Where("Year = ?", y).Order("yearNumber desc").All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimalsOptimizedNoTreatments(animals, c); err != nil {
		return err
	}

	header := []string{
		"Numero", "Class", "Species", "identification", "Entry Date",
		"Discovery Location", "Postal code", "City",
		"Age", "Reason", "Entry cause", "Cause detail",
		"Outtake date", "Outtake Reason", "Location", "Precise location",
	}
	rows := make([][]string, 0, len(*animals))
	classBySpecies, causeDetail := registerLookupMaps(tx, animals)
	for _, a := range *animals {
		outtakeDate := ""
		outtakeType := ""
		outtakeLocation := ""
		outtakePreciseLocation := ""
		if a.Outtake != nil {
			outtakeDate = a.Outtake.DateFormated()
			outtakeType = a.Outtake.Type.Name
			outtakeLocation = a.Outtake.Location.String
			outtakePreciseLocation = a.Outtake.PreciseLocation.String
		}
		postalCode := a.Discovery.PostalCode.String
		city := a.Discovery.City.String
		entryCauseID := a.Discovery.EntryCauseID
		causeDetailValue := causeDetail[entryCauseID]
		rows = append(rows, []string{
			fmt.Sprintf("%d", a.YearNumber),
			classBySpecies[a.Species],
			a.Species,
			a.Ring.String,
			a.Intake.DateFormated(),
			a.Discovery.Location.String,
			postalCode,
			city,
			a.Animalage.Name,
			a.Discovery.Reason.String,
			entryCauseID,
			causeDetailValue,
			outtakeDate,
			outtakeType,
			outtakeLocation,
			outtakePreciseLocation,
		})
	}

	return writeCSV(c, fmt.Sprintf("registertable-%s.csv", y), header, rows)
}

// RegistertableIndex default implementation.
func RegistertableIndex(c buffalo.Context) error {
	years, err := listRegisterYears(c)
	if err != nil {
		return err
	}
	selectedYear := ""
	y := c.Param("year")
	if y == "" {
		years[len(years)-1].Selected = true
		selectedYear = years[len(years)-1].Year
	} else {
		for i := 0; i < len(years); i++ {
			years[i].Selected = years[i].Year == y
			if years[i].Selected {
				selectedYear = years[i].Year
			}
		}
	}
	c.Set("years", years)
	c.Set("selectedYear", selectedYear)

	// Get the DB connection from the context
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	animals := &models.Animals{}

	q := tx.PaginateFromParams(c.Params()).Where("year = ?", selectedYear)

	// Retrieve all Animals from the DB
	if err := q.Order("yearNumber desc").All(animals); err != nil {
		return err
	}

	// Preload required for "list"
	if _, err := EnrichAnimalsOptimizedNoTreatments(animals, c); err != nil {
		return err
	}

	classBySpecies, causeDetail := registerLookupMaps(tx, animals)
	c.Set("speciesClass", classBySpecies)
	c.Set("causeDetail", causeDetail)

	return responder.Wants("html", func(c buffalo.Context) error {
		// Add the paginator to the context so it can be used in the template.
		c.Set("pagination", q.Paginator)

		c.Set("animals", animals)
		return c.Render(http.StatusOK, r.HTML("registertable/registertable.plush.html"))
	}).Wants("json", func(c buffalo.Context) error {
		return c.Render(200, r.JSON(animals))
	}).Wants("xml", func(c buffalo.Context) error {
		return c.Render(200, r.XML(animals))
	}).Respond(c)
}
