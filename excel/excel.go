package excel

import (
	"bytes"
	"creaves/models"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/xuri/excelize/v2"
	"gopkg.in/yaml.v2"
)

//go:embed config/*
var excelConfig embed.FS

type Queries struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Template    string `yaml:"template"`
	Sheet       string `yaml:"sheet"`
	Query       string `yaml:"query"`
}

type Config struct {
	Queries []Queries `yaml:"queries"`
}

func getAllFilenames(efs *embed.FS) (files []string, err error) {
	if err := fs.WalkDir(efs, ".", func(path string, d fs.DirEntry, err error) error {
		if d.IsDir() {
			return nil
		}

		files = append(files, path)

		return nil
	}); err != nil {
		return nil, err
	}

	return files, nil
}

var config *Config = getConfig()

func getConfig() *Config {
	var config Config

	configYamlData, err := excelConfig.ReadFile("config/config.yaml")
	if err != nil {
		panic(fmt.Errorf("Failed to load configuration: %v", err))
	}

	if err := yaml.Unmarshal(configYamlData, &config); err != nil {
		panic(fmt.Sprintf("Error decoding configuration file: %v", err))
	}

	fmt.Printf("config Yaml: %s", configYamlData)

	if files, err := getAllFilenames(&excelConfig); err != nil {
		panic(fmt.Sprintf("failed getting file list: %v", err))
	} else {
		fmt.Printf("Embed files:\n")
		for _, file := range files {
			fmt.Printf("=> %s\n", file)
		}
	}

	return &config
}

func (c *Config) getQuery(id string) (*Queries, error) {
	for _, q := range c.Queries {
		if strings.ToLower(q.Name) == strings.ToLower(id) {
			return &q, nil
		}
	}
	return nil, fmt.Errorf("Could not find query %s", id)
}

// Return queries
func GetQueries() []Queries {
	return config.Queries
}

func sheetPosition(line, col int) string {
	result := ""

	for col > 0 {
		mod := (col - 1) % 26
		result = string(rune('A'+mod)) + result
		col = (col - 1) / 26
	}

	return fmt.Sprintf("%s%d", result, line)
}

// Execute queries
func RunQuery(c buffalo.Context, query string) error {
	// Connect to the database using the connection information from the config file.
	c.Logger().Debugf("Connecting to db type '%s' - with url %s", models.DB.Dialect.Name(), models.DB.Dialect.URL())
	db, err := sql.Open(models.DB.Dialect.Name(), models.DB.Dialect.URL())
	if err != nil {
		return fmt.Errorf("error connecting to database: %v", err)
	}
	defer db.Close()

	sqlQuery, err := config.getQuery(query)
	if err != nil {
		c.Logger().Debugf("Could not find query %s", query)
		c.Response().WriteHeader(http.StatusNotFound)
		c.Response().Write([]byte("404 - Not Found"))
		return nil
	}

	file, err := excelConfig.Open("config/" + sqlQuery.Template)
	if err != nil {
		return fmt.Errorf("error connecting opening file: %v", err)
	}
	defer file.Close()

	f, err := excelize.OpenReader(file)
	if err != nil {
		return fmt.Errorf("error opening template: %v", err)
	}
	defer f.Close()

	// Run the SQL query against the database and return the result set.
	c.Logger().Debugf("Running query %s: %s", sqlQuery.Name, sqlQuery.Query)
	rows, err := db.Query(sqlQuery.Query)
	if err != nil {
		c.Logger().Debugf("Error running query: %v", err)
		return fmt.Errorf("error running query: %s", err)
	}
	defer rows.Close()

	// save columns
	cols, err := rows.Columns()
	if err != nil {
		log.Printf("Error getting columns name: %v", err)
		return fmt.Errorf("error getting columns name: %v", err)
	}

	c.Response().Header().Add("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.xlsx"`, sqlQuery.Name))

	line := 1
	for i, co := range cols {
		pos := sheetPosition(line, i+1)
		if err := f.SetCellValue(sqlQuery.Sheet, pos, co); err != nil {
			c.Logger().Debugf("error exporting to cell %s: %v", pos, err)
			return fmt.Errorf("error exporting to cell %s: %s", pos, err)
		}
	}

	values := make([]interface{}, len(cols))
	valuePtrs := make([]interface{}, len(cols))

	for i := range values {
		valuePtrs[i] = &values[i]
	}

	for rows.Next() {
		line = line + 1

		err := rows.Scan(valuePtrs...)
		if err != nil {
			log.Printf("Error fetching results: %v", err)
			return fmt.Errorf("error getting fetching columns: %v", err)
		}

		for i, co := range values {
			pos := sheetPosition(line, i+1)
			if err := f.SetCellValue(sqlQuery.Sheet, pos, co); err != nil {
				c.Logger().Debugf("error exporting to cell %s: %v", pos, err)
				return fmt.Errorf("error exporting to cell %s: %s", pos, err)
			}
		}
	}

	return writeExcelResponse(c, f, sqlQuery.Sheet, line, len(cols))
}

// writeExcelResponse applies the Bug 5 pivot-cache fixups to f, serializes
// the workbook, patches the final archive (template row truncation +
// _FilterDatabase defined name) and writes it to the response.
func writeExcelResponse(c buffalo.Context, f *excelize.File, sheet string, lastRow, nCols int) error {
	// Update pivot caches to reference the data range actually written and
	// force Excel to refresh them on open (Bug 5). Without this, the cached
	// template range/records make Excel show a "repair/recover" prompt.
	lastCol := sheetPosition(1, nCols)
	lastCol = strings.TrimRight(lastCol, "0123456789")

	if err := updatePivotCaches(f, sheet, lastRow, lastCol); err != nil {
		c.Logger().Debugf("warning: failed to update pivot caches: %v", err)
	}

	// UpdateLinkedValue is intentionally not called: with pivot caches now
	// rebuilt by Excel on open, rewriting linked values risks reintroducing
	// stale cached content (Bug 5).

	// Serialize, then patch the final archive: drop leftover template rows
	// below the written range and update the _xlnm._FilterDatabase defined
	// name (both live in parts excelize re-serializes on WriteTo).
	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		c.Logger().Debugf("Failed writing excel file: %v", err)
		return fmt.Errorf("failed writing excel file: %s", err)
	}

	out, err := truncateSheetRowsInZip(buf.Bytes(), sheet, lastRow, lastCol)
	if err != nil {
		c.Logger().Debugf("warning: failed to finalize export archive: %v", err)
		out = buf.Bytes()
	}

	if cnt, err := c.Response().Write(out); err != nil {
		c.Logger().Debugf("Failed writing excel file: %v", err)
		return fmt.Errorf("failed writing excel file: %s", err)
	} else {
		c.Logger().Debugf("wrote %d bytes to file", cnt)
	}

	return nil
}
