package actions

import (
	"creaves/excel"
	"creaves/export"
	"errors"
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

// ExportView renders the selected export query as an online HTML table.
// Without a "query" param it renders the chooser page.
func ExportView(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		c.Set("queries", export.GetQueries())
		return c.Render(http.StatusOK, r.HTML("export/index.html"))
	}

	sqlQuery, cols, rows, err := export.FetchRows(query)
	if err != nil {
		if errors.Is(err, export.ErrQueryNotFound) {
			c.Set("queries", export.GetQueries())
			return c.Render(http.StatusNotFound, r.HTML("export/index.html"))
		}
		return err
	}

	c.Set("query", sqlQuery)
	c.Set("cols", cols)
	c.Set("rows", rows)
	return c.Render(http.StatusOK, r.HTML("export/view.html"))
}

// ExportCsv default implementation.
func ExportExcel(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		c.Set("queries", excel.GetQueries())
		return c.Render(http.StatusOK, r.HTML("export/excel.html"))
	}

	return excel.RunQuery(c, query)
}
