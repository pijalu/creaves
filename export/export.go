package export

import (
	"creaves/models"
	"database/sql"
	"encoding/csv"
	"fmt"
	"log"
	"net/http"
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

	c.Response().Header().Add("Content-Type", "text/csv")
	c.Response().Header().Add("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, sqlQuery.Name))

	// Create a new CSV writer for the response stream.
	csvWriter := csv.NewWriter(c.Response())
	defer csvWriter.Flush()
	csvWriter.Write(cols)

	rowPtr := make([]any, len(cols))
	rowString := make([]*string, len(cols))
	for i := range rowString {
		rowPtr[i] = &rowString[i]
	}
	rowStringNull := make([]string, len(cols))

	for rows.Next() {
		err := rows.Scan(rowPtr...)
		if err != nil {
			log.Printf("Error fetching results: %v", err)
			return fmt.Errorf("error getting fetching columns: %v", err)
		}
		for i, str := range rowString {
			if str == nil {
				rowStringNull[i] = "null"
			} else {
				rowStringNull[i] = *str
			}
		}

		csvWriter.Write(rowStringNull)
	}

	return nil
}
