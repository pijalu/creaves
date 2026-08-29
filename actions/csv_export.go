package actions

import (
	"encoding/csv"
	"fmt"
	"io"
	"net/http"

	"github.com/gobuffalo/buffalo"
)

// utf8BOM is written at the start of CSV files so Excel detects UTF-8.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// writeCSVTo writes header and rows to w as CSV with a UTF-8 BOM prefix
// (Excel compatibility) and ';' as separator (Excel-FR convention, matches
// existing exports). NULL/empty values must be passed as empty strings.
func writeCSVTo(w io.Writer, header []string, rows [][]string) error {
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	cw := csv.NewWriter(w)
	cw.Comma = ';'
	if err := cw.Write(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// writeCSV writes header and rows to the response as a CSV file download.
func writeCSV(c buffalo.Context, filename string, header []string, rows [][]string) error {
	res := c.Response()
	res.Header().Set("Content-Type", "text/csv; charset=utf-8")
	res.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))
	res.WriteHeader(http.StatusOK)
	return writeCSVTo(res, header, rows)
}
