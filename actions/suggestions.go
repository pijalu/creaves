package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

func suggest(c buffalo.Context, table string, field string) error {
	s := []string{}

	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := "SELECT DISTINCT " + field + " FROM " + table

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" WHERE "+field+" like ?", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsAnimalSpecies default implementation.
func SuggestionsAnimalSpecies(c buffalo.Context) error {
	return suggest(c, "species", "creaves_species")
}

// SuggestionsDiscoveryLocation default implementation.
func SuggestionsDiscoveryLocation(c buffalo.Context) error {
	return suggest(c, "discoveries", "location")
}

// SuggestionsOuttakeLocation default implementation.
func SuggestionsOuttakeLocation(c buffalo.Context) error {
	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := `SELECT CONCAT(postal_code,"_",locality) FROM localities`

	if len(q) > 0 {
		query = tx.RawQuery(qroot+` WHERE CONCAT(postal_code,"_",locality) like ?`, "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsDiscovererCity default implementation.
func SuggestionsDiscovererCity(c buffalo.Context) error {
	return suggest(c, "discoverers", "city")
}

// SuggestionsDiscovererCountry default implementation.
func SuggestionsDiscovererCountry(c buffalo.Context) error {
	return suggest(c, "discoverers", "country")
}

// SuggestionsDiscovererCountry default implementation.
func SuggestionsPostalCode(c buffalo.Context) error {
	return suggest(c, "localities", "postal_code")
}

// SuggestionsOuttakeLocation default implementation.
func SuggestionsLocality(c buffalo.Context) error {
	z := c.Param("z") // zip
	l := c.Param("l") // locality

	ret := c.Param("r") // return

	var field string
	switch ret {
	case "postal_code":
		field = "postal_code"
	case "locality":
		field = "locality"
	default:
		return fmt.Errorf("unexpected request for %s", ret)
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := fmt.Sprintf(`SELECT distinct %s FROM localities WHERE 1=1`, field)
	args := []interface{}{}

	// Add postal code
	if len(z) > 0 {
		qroot += ` AND postal_code LIKE ?`
		args = append(args, "%"+z+"%")
	}

	// Add zip code
	if len(l) > 0 {
		qroot += ` AND locality LIKE ?`
		args = append(args, "%"+l+"%")
	}
	qroot += " ORDER BY 1 LIMIT 10"
	c.Logger().Debugf("Query: %s - params: %v", qroot, args)
	query = tx.RawQuery(qroot, args...)

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// Suggest discovered
func SuggestionsDiscoverer(c buffalo.Context) error {
	f := c.Param("f") // first name
	l := c.Param("l") // last name
	a := c.Param("a") // address

	ret := c.Param("r") // return

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	var field string

	switch ret {
	case "firstname":
		field = "firstname"
	case "lastname":
		field = "lastname"
	case "address":
		field = "address"
	default:
		return fmt.Errorf("unexpected request for %s", ret)
	}

	qroot := fmt.Sprintf(`SELECT DISTINCT %s FROM discoverers WHERE 1=1 `, field)
	args := []interface{}{}

	// Add first name
	if len(f) > 0 {
		qroot += ` AND firstname like ?`
		args = append(args, "%"+f+"%")
	}

	// Add last name
	if len(l) > 0 {
		qroot += ` AND lastname LIKE ?`
		args = append(args, "%"+l+"%")
	}

	// add Address
	if len(a) > 0 {
		qroot += ` AND address LIKE ?`
		args = append(args, "%"+a+"%")
	}

	c.Logger().Debugf("Query: %s - params: %v", qroot, args)
	query = tx.RawQuery(qroot, args...)

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsAnimalTypeDefaultSpecies default implementation.
func SuggestionsAnimalTypeDefaultSpecies(c buffalo.Context) error {
	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := "SELECT distinct default_species FROM animaltypes WHERE default_species is NOT NULL "

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" and name like ?", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsTreatmentDrug default implementation.
func SuggestionsTreatmentDrug(c buffalo.Context) error {
	q := c.Param("q")
	at := c.Param("at")
	//w := c.Param("w")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := `
		SELECT d.name 
		FROM drugs d, 
		     dosages s 
		WHERE d.ID = s.drug_id 
		  AND s.animaltype_id = ?`

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND d.Name like ?", at, "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot, at)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsTreatmentDrug default implementation.
func SuggestionsTreatmentDrugDosage(c buffalo.Context) error {
	result := []string{}

	q := c.Param("q")
	at := c.Param("at")
	w, err := strconv.ParseFloat(c.Param("w"), 64)
	if err != nil {
		c.Logger().Debugf("Failed to convert weight to integer:%v", err)
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	c.Logger().Debugf("Dosage for: %v %v %v", q, at, w)

	if len(q) == 0 {
		c.Logger().Debugf("No medication name provided")
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	d := &models.Dosage{}
	var query *pop.Query
	qroot := `
		SELECT s.* 
		FROM drugs d, 
		     dosages s 
		WHERE d.Name = ?
		  AND d.ID = s.drug_id
		  AND s.animaltype_id = ?`

	query = tx.RawQuery(qroot, q, at)

	if err := query.First(d); err != nil {
		c.Logger().Debugf("Dosage lookup failed: %v", err)
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	//c.Logger().Debugf("Loaded mediaction/dosage: %v", d)

	if !d.DosagePerGrams.Valid {
		c.Logger().Debugf("No dosage for drug")
		return c.Render(http.StatusNotFound, r.JSON(result))
	}

	c.Logger().Debugf("Dosage: %v * %v", w, d.DosagePerGrams.Float64)
	ds := w * d.DosagePerGrams.Float64

	result = append(result, fmt.Sprintf("%.2f %s", ds, d.DosagePerGramsUnit.String))

	return c.Render(200, r.JSON(result))
}

// SuggestionsAnimalInCare - specific implementation to only account in care animal (no outtake).
func SuggestionsAnimalInCare(c buffalo.Context) error {
	results := []struct {
		Year       string `json:"Year" db:"Year"`
		YearNumber string `json:"YearNumber" db:"YearNumber"`
	}{}

	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := "SELECT Year, YearNumber FROM animals WHERE outtake_id IS null "

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND YearNumber like ?", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	if err := query.All(&results); err != nil {
		return err
	}

	// return a series of strings
	s := []string{}
	for _, result := range results {
		s = append(s, fmt.Sprintf("%s/%s", result.YearNumber, result.Year[2:]))
	}

	return c.Render(200, r.JSON(s))
}

// SuggestionsCagesAnimalInCare - specific implementation to only account for cages with animal (no outtake).
func SuggestionsCageWithAnimalInCare(c buffalo.Context) error {
	q := c.Param("q")

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var query *pop.Query
	qroot := "SELECT DISTINCT Cage FROM animals WHERE outtake_id IS null and Cage is not null"

	if len(q) > 0 {
		query = tx.RawQuery(qroot+" AND Cage like ?", "%"+q+"%")
	} else {
		query = tx.RawQuery(qroot)
	}

	s := []string{}
	if err := query.All(&s); err != nil {
		return err
	}

	return c.Render(200, r.JSON(s))
}
