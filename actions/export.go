package actions

import (
	"creaves/export"
	"net/http"

	"github.com/gobuffalo/buffalo"
)

// ExportCsv default implementation.
func ExportCsv(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		c.Set("queries", export.GetQueries())
		return c.Render(http.StatusOK, r.HTML("export/csv.html"))
	}

	return export.RunQuery(c, query)
}
