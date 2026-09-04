package actions

import (
	"html/template"
	"net/http"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/plush/v4"
	"github.com/gobuffalo/pop/v6"
)

// animalSortColumns maps the public `sort` parameter to a fixed ORDER BY SQL
// expression. Only keys present in this whitelist ever reach the database —
// user input is used as a map lookup, never inside SQL. Related-table columns
// (intake date, type name, age name) are sorted via scalar subqueries to
// avoid JOIN + DISTINCT row duplication, mirroring the search filters.
//
// Sorting happens on canonical base-locale values; localized display values
// (translations table) are not considered for ordering.
var animalSortColumns = map[string]string{
	"number":         "animals.yearNumber",
	"year":           "animals.year",
	"intake_date":    "(SELECT i.date FROM intakes i WHERE i.id = animals.intake_id)",
	"zone":           "animals.zone",
	"cage":           "animals.cage",
	"type":           "(SELECT t.name FROM animaltypes t WHERE t.id = animals.animaltype_id)",
	"species":        "animals.species",
	"age":            "(SELECT a.name FROM animalages a WHERE a.id = animals.animalage_id)",
	"identification": "animals.ring",
}

// applyAnimalSort applies the `sort`/`dir` request parameters to q, falling
// back to the default `animals.id desc` register order when the parameters
// are absent or invalid. See animalSortClauses for the clause mapping.
func applyAnimalSort(q *pop.Query, c buffalo.Context) *pop.Query {
	for _, clause := range animalSortClauses(c.Param("sort"), c.Param("dir")) {
		q = q.Order(clause)
	}
	return q
}

// animalSortClauses is the pure core of applyAnimalSort: it maps a sort key
// and direction to ORDER BY SQL fragments. `animals.id desc` is always
// appended last so the order is stable (and matches the historical default
// within equal keys). For `number` the year is sorted newest-first as a
// secondary key, matching how year numbers repeat across years.
func animalSortClauses(sortKey, dir string) []string {
	col, ok := animalSortColumns[sortKey]
	if !ok {
		return []string{"animals.id desc"}
	}
	dir = strings.ToLower(dir)
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	clauses := []string{col + " " + dir}
	if sortKey == "number" {
		clauses = append(clauses, "animals.year desc")
	}
	return append(clauses, "animals.id desc")
}

// animalSortFields lists the sortable column keys in the display order of the
// animals index table. Used by the template helpers below.
var animalSortFields = []string{
	"number", "year", "intake_date", "zone", "cage", "type", "species",
	"age", "identification",
}

// sortParams extracts the current `sort`/`dir` pair from the request inside
// a plush helper. Missing request (unit tests, mail renders) yields defaults.
func sortParams(help plush.HelperContext) (sortKey, dir string) {
	req, _ := help.Value("request").(*http.Request)
	if req == nil {
		return "", ""
	}
	sortKey = req.URL.Query().Get("sort")
	dir = strings.ToLower(req.URL.Query().Get("dir"))
	if dir != "asc" && dir != "desc" {
		dir = ""
	}
	return sortKey, dir
}

// sortLink is a template helper: href for a sortable column header link.
// Clicking the active column toggles the direction; any other column sorts
// ascending. Existing query parameters (filters, page) are preserved, but the
// page resets to 1 because the row order changes.
func sortLink(field string, help plush.HelperContext) (template.HTML, error) {
	req, _ := help.Value("request").(*http.Request)
	if req == nil {
		return template.HTML("#"), nil
	}
	vals := req.URL.Query()
	cur, dir := sortParams(help)
	next := "asc"
	if cur == field && dir == "asc" {
		next = "desc"
	}
	vals.Set("sort", field)
	vals.Set("dir", next)
	vals.Del("page")
	return template.HTML(req.URL.Path + "?" + vals.Encode()), nil
}

// sortIcon is a template helper: ▲/▼ for the active sort column, empty for
// inactive ones.
func sortIcon(field string, help plush.HelperContext) (template.HTML, error) {
	cur, dir := sortParams(help)
	if cur != field {
		return template.HTML(""), nil
	}
	if dir == "desc" {
		return template.HTML("▼"), nil
	}
	return template.HTML("▲"), nil
}
