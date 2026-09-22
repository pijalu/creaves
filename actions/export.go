package actions

import (
	"creaves/excel"
	"creaves/export"
	"errors"
	"net/http"

	"github.com/gobuffalo/buffalo"
)

// ExportCsv streams one configured query (export/config.yaml) as a CSV
// download. Without a "query" param it redirects to the online view chooser.
func ExportCsv(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		return c.Redirect(http.StatusFound, "/export/view")
	}

	return export.RunQuery(c, query)
}

// ExportView renders the selected export query as an online HTML table.
// Without a "query" param it renders the chooser page.
func ExportView(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		c.Set("queries", export.GetQueries())
		if err := setExportYearOptions(c); err != nil {
			return err
		}
		return c.Render(http.StatusOK, r.HTML("export/index.html"))
	}

	year := export.ParseYear(c)
	sqlQuery, cols, rows, err := export.FetchRows(query, year)
	if err != nil {
		if errors.Is(err, export.ErrQueryNotFound) {
			c.Set("queries", export.GetQueries())
			if err := setExportYearOptions(c); err != nil {
				return err
			}
			return c.Render(http.StatusNotFound, r.HTML("export/index.html"))
		}
		return err
	}

	c.Set("query", sqlQuery)
	c.Set("year", year)
	if err := setExportYearOptions(c); err != nil {
		return err
	}
	c.Set("cols", cols)
	c.Set("rows", rows)
	return c.Render(http.StatusOK, r.HTML("export/view.html"))
}

// ExportCsv default implementation.
func ExportExcel(c buffalo.Context) error {
	query := c.Param("query")
	if query == "" {
		c.Set("queries", excel.GetQueries())
		if err := setExportYearOptions(c); err != nil {
			return err
		}
		return c.Render(http.StatusOK, r.HTML("export/excel.html"))
	}

	return excel.RunQuery(c, query)
}

// setExportYearOptions provides the export pages' single year dropdown with
// the years present in the database (bugs.md bug 5): the year selected
// through the request params is marked Selected, no selection means
// "all years".
func setExportYearOptions(c buffalo.Context) error {
	years, err := listRegisterYears(c)
	if err != nil {
		return err
	}
	selected := c.Param("year")
	for i := range years {
		years[i].Selected = years[i].Year == selected
	}
	c.Set("years", years)
	c.Set("selectedYear", selected)
	return nil
}
