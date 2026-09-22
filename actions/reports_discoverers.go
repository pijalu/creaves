package actions

import (
	"fmt"
	"net/http"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// Discoverers register (issue #85): lists discoverers together with the
// animals linked to them through their discovery, with a CSV export.

// discovererRow is one line of the discoverers register: one row per
// discoverer per linked animal (discoverers without animals are listed with
// empty animal columns). Raw-query mapped: yearNumber keeps the animals
// table's camelCase column name.
type discovererRow struct {
	DiscovererID uuid.UUID    `db:"discoverer_id"`
	Firstname    nulls.String `db:"firstname"`
	Lastname     nulls.String `db:"lastname"`
	Address      nulls.String `db:"address"`
	PostalCode   nulls.String `db:"postal_code"`
	City         nulls.String `db:"city"`
	Country      nulls.String `db:"country"`
	Email        nulls.String `db:"email"`
	Phone        nulls.String `db:"phone"`

	AnimalID   nulls.Int    `db:"animal_id"`
	Year       nulls.Int    `db:"year"`
	YearNumber nulls.Int    `db:"yearNumber"`
	Species    nulls.String `db:"species"`
}

// AnimalNumber returns the "number/year" animal identifier or an empty
// string when the discoverer has no linked animal.
func (r discovererRow) AnimalNumber() string {
	if !r.AnimalID.Valid {
		return ""
	}
	return fmt.Sprintf("%d/%d", r.YearNumber.Int, r.Year.Int%100)
}

const SQL_DISCOVERER_ROWS = `
select d.id as discoverer_id, d.firstname, d.lastname, d.address,
       d.postal_code, d.city, d.country, d.email, d.phone,
       a.id as animal_id, a.year, a.yearNumber, a.species
from discoverers d
left join discoveries dis on dis.discoverer_id = d.id
left join animals a on a.discovery_id = dis.id
where (? = '' or a.year = ? or a.id is null)
order by d.lastname asc, d.firstname asc, a.year desc, a.yearNumber asc`

func listDiscovererRows(tx *pop.Connection, year string) ([]discovererRow, error) {
	rows := []discovererRow{}
	if err := tx.RawQuery(SQL_DISCOVERER_ROWS, year, year).All(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ReportsDiscoverersIndex handles GET /reports/discoverers?year=YYYY.
func ReportsDiscoverersIndex(c buffalo.Context) error {
	years, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	c.Set("years", years)
	c.Set("selectedYear", selectedYear)

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows := []discovererRow{}
	if selectedYear != "" {
		if rows, err = listDiscovererRows(tx, selectedYear); err != nil {
			return err
		}
	}
	c.Set("discovererRows", rows)

	return c.Render(http.StatusOK, r.HTML("reports/discoverers.plush.html"))
}

// ReportsDiscoverersExportCSV handles GET /reports/discoverers/export.csv?year=YYYY.
func ReportsDiscoverersExportCSV(c buffalo.Context) error {
	_, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	if selectedYear == "" {
		return fmt.Errorf("year not provided")
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows, err := listDiscovererRows(tx, selectedYear)
	if err != nil {
		return err
	}

	header := []string{
		T.Translate(c, "reports.discoverers.csv.firstname"),
		T.Translate(c, "reports.discoverers.csv.lastname"),
		T.Translate(c, "reports.discoverers.csv.address"),
		T.Translate(c, "reports.discoverers.csv.postal_code"),
		T.Translate(c, "reports.discoverers.csv.city"),
		T.Translate(c, "reports.discoverers.csv.country"),
		T.Translate(c, "reports.discoverers.csv.email"),
		T.Translate(c, "reports.discoverers.csv.phone"),
		T.Translate(c, "reports.discoverers.csv.animal"),
		T.Translate(c, "reports.discoverers.csv.species"),
	}
	records := make([][]string, 0, len(rows))
	for _, row := range rows {
		records = append(records, []string{
			row.Firstname.String,
			row.Lastname.String,
			row.Address.String,
			row.PostalCode.String,
			row.City.String,
			row.Country.String,
			row.Email.String,
			row.Phone.String,
			row.AnimalNumber(),
			row.Species.String,
		})
	}

	return writeCSV(c, fmt.Sprintf("reports-discoverers-%s.csv", selectedYear), header, records)
}

// discovererLookupEntry is the JSON payload returned by
// SuggestionsDiscovererLookup for the reception form discoverer picker
// (issue #85): full discoverer record so the form can prefill every field.
type discovererLookupEntry struct {
	ID            uuid.UUID `json:"id"`
	Firstname     string    `json:"firstname"`
	Lastname      string    `json:"lastname"`
	Address       string    `json:"address"`
	PostalCode    string    `json:"postal_code"`
	City          string    `json:"city"`
	Country       string    `json:"country"`
	Email         string    `json:"email"`
	Phone         string    `json:"phone"`
	Donation      string    `json:"donation"`
	ReturnRequest bool      `json:"return_request"`
	Label         string    `json:"label"`
}

// SuggestionsDiscovererLookup handles GET /suggestions/discoverer_lookup?q=...
// and returns up to 10 discoverers whose firstname or lastname matches q,
// with all fields needed to reuse the record in the reception form.
func SuggestionsDiscovererLookup(c buffalo.Context) error {
	q := strings.TrimSpace(c.Param("q"))
	if len(q) < 2 {
		return c.Render(http.StatusOK, r.JSON([]discovererLookupEntry{}))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	discoverers := models.Discoverers{}
	if err := tx.Where("firstname LIKE ? OR lastname LIKE ?", "%"+q+"%", "%"+q+"%").
		Order("lastname asc, firstname asc").
		Limit(10).
		All(&discoverers); err != nil {
		return err
	}

	entries := make([]discovererLookupEntry, 0, len(discoverers))
	for _, d := range discoverers {
		name := strings.TrimSpace(d.Firstname.String + " " + d.Lastname.String)
		place := strings.Trim(strings.TrimSpace(d.Address.String+", "+d.PostalCode.String+" "+d.City.String), ", ")
		label := name
		if place != "" {
			label = name + " — " + place
		}
		entries = append(entries, discovererLookupEntry{
			ID:            d.ID,
			Firstname:     d.Firstname.String,
			Lastname:      d.Lastname.String,
			Address:       d.Address.String,
			PostalCode:    d.PostalCode.String,
			City:          d.City.String,
			Country:       d.Country.String,
			Email:         d.Email.String,
			Phone:         d.Phone.String,
			Donation:      d.Donation.String,
			ReturnRequest: d.ReturnRequest,
			Label:         label,
		})
	}

	return c.Render(http.StatusOK, r.JSON(entries))
}
