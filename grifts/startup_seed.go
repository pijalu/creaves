package grifts

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"fmt"

	"creaves/models"
	"creaves/utils"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/pop/v6"
	"github.com/pkg/errors"
)

//go:embed creaves-startup.sql.gz
var startupSQLGz []byte

// startupTables are seeded in this order (dump statement order may differ).
var startupTables = []string{"animalages", "animaltypes", "caretypes", "outtaketypes", "drugs", "species", "dosages"}

// startupModels maps each startup table to a model instance used for the
// skip-if-non-empty count check.
var startupModels = map[string]interface{}{
	"animalages":   &models.Animalage{},
	"animaltypes":  &models.Animaltype{},
	"caretypes":    &models.Caretype{},
	"outtaketypes": &models.Outtaketype{},
	"drugs":        &models.Drug{},
	"species":      &models.Species{},
	"dosages":      &models.Dosage{},
}

// seedStartup loads the embedded production reference data (French) into the
// 7 startup tables. It is idempotent per table: a table that already holds
// rows is skipped. All inserts run in a single transaction.
func seedStartup(c *grift.Context) error {
	gz, err := gzip.NewReader(bytes.NewReader(startupSQLGz))
	if err != nil {
		return errors.WithStack(err)
	}
	defer gz.Close()

	order, stmts, err := utils.ExtractInsertStatements(gz)
	if err != nil {
		return errors.WithStack(err)
	}

	// Execute known startup tables first, then any extras in dump order.
	ordered := make([]string, 0, len(order))
	seen := map[string]bool{}
	for _, t := range startupTables {
		if _, ok := stmts[t]; ok {
			ordered = append(ordered, t)
			seen[t] = true
		}
	}
	for _, t := range order {
		if !seen[t] {
			ordered = append(ordered, t)
		}
	}

	return models.DB.Transaction(func(tx *pop.Connection) error {
		for _, table := range ordered {
			model, ok := startupModels[table]
			if !ok {
				fmt.Printf("%s: no model registered, skipping\n", table)
				continue
			}
			cnt, err := tx.Q().Count(model)
			if err != nil {
				return errors.WithStack(err)
			}
			if cnt > 0 {
				fmt.Printf("%s: %d rows, skipping\n", table, cnt)
				continue
			}
			for _, stmt := range stmts[table] {
				if err := tx.RawQuery(stmt).Exec(); err != nil {
					return errors.WithStack(errors.Wrapf(err, "seeding %s", table))
				}
			}
			fmt.Printf("%s: seeded %d statements\n", table, len(stmts[table]))
		}
		return nil
	})
}

var _ = grift.Namespace("db", func() {
	grift.Desc("seed:startup", "Seeds the 7 startup reference tables from the embedded production dump (idempotent)")
	grift.Add("seed:startup", func(c *grift.Context) error {
		return seedStartup(c)
	})
})
