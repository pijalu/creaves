package export

import (
	"creaves/models"
	"database/sql"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gobuffalo/buffalo"
	"gopkg.in/yaml.v2"

	_ "embed"
)

//go:embed config.yaml
var configYamlData []byte

type Queries struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Query       string `yaml:"query"`
}

type Config struct {
	Queries []Queries `yaml:"queries"`
}

var config *Config = getConfig()

func getConfig() *Config {
	var config Config

	if err := yaml.Unmarshal(configYamlData, &config); err != nil {
		panic(fmt.Sprintf("Error decoding configuration file: %v", err))
	}

	return &config
}

func (c *Config) getQuery(id string) (*Queries, error) {
	for _, q := range c.Queries {
		if strings.EqualFold(q.Name, id) {
			return &q, nil
		}
	}
	return nil, fmt.Errorf("%w: %s", ErrQueryNotFound, id)
}

// ErrQueryNotFound is returned when no export query matches the given id.
var ErrQueryNotFound = errors.New("could not find query")

// Return queries
func GetQueries() []Queries {
	return config.Queries
}

// FetchRows runs the named export query and returns the query definition,
// the column names and the result rows as strings (NULL rendered as "").
func FetchRows(query string) (*Queries, []string, [][]string, error) {
	// Connect to the database using the connection information from the config file.
	db, err := sql.Open(models.DB.Dialect.Name(), models.DB.Dialect.URL())
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error connecting to database: %v", err)
	}
	defer db.Close()

	sqlQuery, err := config.getQuery(query)
	if err != nil {
		return nil, nil, nil, err
	}

	rows, err := db.Query(sqlQuery.Query)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error running query: %s", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error getting columns name: %v", err)
	}

	rowPtr := make([]any, len(cols))
	rowString := make([]*string, len(cols))
	for i := range rowString {
		rowPtr[i] = &rowString[i]
	}

	result := [][]string{}
	for rows.Next() {
		if err := rows.Scan(rowPtr...); err != nil {
			return nil, nil, nil, fmt.Errorf("error getting fetching columns: %v", err)
		}
		rowStringNull := make([]string, len(cols))
		for i, str := range rowString {
			if str == nil {
				rowStringNull[i] = ""
			} else {
				rowStringNull[i] = *str
			}
		}
		result = append(result, rowStringNull)
	}

	return sqlQuery, cols, result, nil
}

// utf8BOM is written at the start of CSV files so Excel and other
// locale-dependent readers detect UTF-8 instead of assuming the local
// ANSI codepage (accented characters would render as mojibake).
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// writeCSV writes cols and rows to w as comma-separated CSV prefixed with a
// UTF-8 BOM. NULL/empty values must be passed as empty strings.
func writeCSV(w io.Writer, cols []string, rows [][]string) error {
	if _, err := w.Write(utf8BOM); err != nil {
		return err
	}
	csvWriter := csv.NewWriter(w)
	if err := csvWriter.Write(cols); err != nil {
		return err
	}
	for _, row := range rows {
		if err := csvWriter.Write(row); err != nil {
			return err
		}
	}
	csvWriter.Flush()
	return csvWriter.Error()
}

// ExportFilter captures the search state of the online export view
// (DataTables global search + per-column filters) so the CSV download can
// return the same rows the user sees, not the raw query result (#197
// sub-item 3).
type ExportFilter struct {
	Global   string
	Contains map[int]string // column index -> case-insensitive substring
}

// FilterFromParams reads the filter state from request params: "q" is the
// global search, "cols" is a comma-separated list of "idx=substring" pairs
// (e.g. cols=0=pigeon,3=2026) for per-column filters.
func FilterFromParams(params buffalo.ParamValues) ExportFilter {
	f := ExportFilter{Contains: map[int]string{}}
	f.Global = params.Get("q")
	if raw := params.Get("cols"); raw != "" {
		for _, pair := range strings.Split(raw, ",") {
			kv := strings.SplitN(pair, "=", 2)
			if len(kv) != 2 || kv[1] == "" {
				continue
			}
			idx, err := strconv.Atoi(kv[0])
			if err != nil || idx < 0 {
				continue
			}
			f.Contains[idx] = kv[1]
		}
	}
	return f
}

// Empty reports whether the filter is a no-op.
func (f ExportFilter) Empty() bool {
	return f.Global == "" && len(f.Contains) == 0
}

// matchRow reports whether one row satisfies the global search (any column)
// and every per-column filter (case-insensitive contains).
func (f ExportFilter) matchRow(row []string) bool {
	fold := strings.ToLower
	if f.Global != "" {
		hit := false
		for _, cell := range row {
			if strings.Contains(fold(cell), fold(f.Global)) {
				hit = true
				break
			}
		}
		if !hit {
			return false
		}
	}
	for idx, want := range f.Contains {
		if idx < 0 || idx >= len(row) {
			continue
		}
		if !strings.Contains(fold(row[idx]), fold(want)) {
			return false
		}
	}
	return true
}

// FilterRows returns the subset of rows matching the filter. When the filter
// is empty the original slice is returned untouched.
func FilterRows(rows [][]string, f ExportFilter) [][]string {
	if f.Empty() {
		return rows
	}
	out := make([][]string, 0, len(rows))
	for _, row := range rows {
		if f.matchRow(row) {
			out = append(out, row)
		}
	}
	return out
}

// Execute queries
func RunQuery(c buffalo.Context, query string) error {
	sqlQuery, cols, rows, err := FetchRows(query)
	if err != nil {
		if errors.Is(err, ErrQueryNotFound) {
			c.Logger().Debugf("Could not find query %s", query)
			c.Response().WriteHeader(http.StatusNotFound)
			c.Response().Write([]byte("404 - Not Found"))
			return nil
		}
		c.Logger().Debugf("Error running query: %v", err)
		return err
	}

	// Honor the filters applied in the online view (#197 sub-item 3):
	// the download must match what the user sees on /export/view.
	rows = FilterRows(rows, FilterFromParams(c.Params()))

	c.Response().Header().Add("Content-Type", "text/csv; charset=utf-8")
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, sqlQuery.Name))

	return writeCSV(c.Response(), cols, rows)
}
