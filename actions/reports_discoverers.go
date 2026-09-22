package actions

import (
	"fmt"
	"net/http"
	"net/url"
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
  and (? = '' or concat(coalesce(d.firstname, ''), ' ', coalesce(d.lastname, '')) like ?)
  and (? = '' or d.city like ?)
  and (? = '' or d.postal_code like ?)
order by d.lastname asc, d.firstname asc, a.year desc, a.yearNumber asc`

// discovererFilters holds the optional discoverers register filters
// (issue #202 bug 4): case-insensitive substring match on the discoverer
// name (firstname + lastname), city and postal code. Empty filters are
// ignored.
type discovererFilters struct {
	Name       string
	City       string
	PostalCode string
}

// discovererFiltersFrom reads and trims the filter params from the request.
func discovererFiltersFrom(c buffalo.Context) discovererFilters {
	return discovererFilters{
		Name:       strings.TrimSpace(c.Param("name")),
		City:       strings.TrimSpace(c.Param("city")),
		PostalCode: strings.TrimSpace(c.Param("postal_code")),
	}
}

// likeArgs builds the raw-query arguments for the three optional LIKE
// conjuncts of SQL_DISCOVERER_ROWS (value and sentinel per conjunct).
func (f discovererFilters) likeArgs() []interface{} {
	like := func(s string) interface{} { return "%" + s + "%" }
	return []interface{}{
		f.Name, like(f.Name),
		f.City, like(f.City),
		f.PostalCode, like(f.PostalCode),
	}
}

// query encodes the filters (plus the year) as URL query parameters, used to
// carry the active filters over to the CSV export link.
func (f discovererFilters) query(year string) string {
	v := url.Values{}
	if year != "" {
		v.Set("year", year)
	}
	if f.Name != "" {
		v.Set("name", f.Name)
	}
	if f.City != "" {
		v.Set("city", f.City)
	}
	if f.PostalCode != "" {
		v.Set("postal_code", f.PostalCode)
	}
	return v.Encode()
}

func listDiscovererRows(tx *pop.Connection, year string, filters discovererFilters) ([]discovererRow, error) {
	rows := []discovererRow{}
	args := append([]interface{}{year, year}, filters.likeArgs()...)
	if err := tx.RawQuery(SQL_DISCOVERER_ROWS, args...).All(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// ReportsDiscoverersIndex handles GET /reports/discoverers?year=YYYY with
// optional name/city/postal_code filters (issue #202 bug 4).
func ReportsDiscoverersIndex(c buffalo.Context) error {
	years, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	c.Set("years", years)
	c.Set("selectedYear", selectedYear)

	filters := discovererFiltersFrom(c)
	c.Set("filters", filters)
	c.Set("csvQuery", filters.query(selectedYear))

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows := []discovererRow{}
	if selectedYear != "" {
		if rows, err = listDiscovererRows(tx, selectedYear, filters); err != nil {
			return err
		}
	}
	c.Set("discovererRows", rows)

	return c.Render(http.StatusOK, r.HTML("reports/discoverers.plush.html"))
}

// ReportsDiscoverersExportCSV handles GET
// /reports/discoverers/export.csv?year=YYYY, honouring the same optional
// filters as the index view.
func ReportsDiscoverersExportCSV(c buffalo.Context) error {
	_, selectedYear, err := selectAnnualYear(c)
	if err != nil {
		return err
	}
	if selectedYear == "" {
		return fmt.Errorf("year not provided")
	}
	filters := discovererFiltersFrom(c)

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	rows, err := listDiscovererRows(tx, selectedYear, filters)
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
	// Search names, but also city / postal code: stored city values may
	// merge the zip ("67000 Strasbourg") and users search by either part
	// (bugs.md #9).
	if err := tx.Where("firstname LIKE ? OR lastname LIKE ? OR city LIKE ? OR postal_code LIKE ?",
		"%"+q+"%", "%"+q+"%", "%"+q+"%", "%"+q+"%").
		Order("lastname asc, firstname asc").
		Limit(10).
		All(&discoverers); err != nil {
		return err
	}

	entries := make([]discovererLookupEntry, 0, len(discoverers))
	for _, d := range discoverers {
		// Fill must be correct: split a zip merged into the stored city
		// before building the entry and its display label (bugs.md #9).
		postalCode, city := splitPostalCity(d.PostalCode.String, d.City.String)
		name := strings.TrimSpace(d.Firstname.String + " " + d.Lastname.String)
		place := strings.Trim(strings.TrimSpace(d.Address.String+", "+postalCode+" "+city), ", ")
		label := name
		if place != "" {
			label = name + " — " + place
		}
		entries = append(entries, discovererLookupEntry{
			ID:            d.ID,
			Firstname:     d.Firstname.String,
			Lastname:      d.Lastname.String,
			Address:       d.Address.String,
			PostalCode:    postalCode,
			City:          city,
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
