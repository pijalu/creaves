package grifts

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"regexp"

	"creaves/models"
	"creaves/utils"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/pkg/errors"
)

//go:embed creaves-startup.sql.gz
var startupSQLGz []byte

//go:embed translations_*.sql
var translationSQLFS embed.FS

// translationFileLocaleRe extracts the locale from translations_<locale>.sql.
var translationFileLocaleRe = regexp.MustCompile(`^translations_([a-z]{2})\.sql$`)

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

// startupTranslatableFields maps each startup table to the columns mirrored
// into the translations table (locale fr). The base column keeps the
// canonical French value; translations make the value addressable per locale.
var startupTranslatableFields = map[string][]string{
	"animalages":   {"name", "description"},
	"animaltypes":  {"name", "description"},
	"caretypes":    {"name", "description"},
	"outtaketypes": {"name", "description"},
	"drugs":        {"name", "description"},
	"dosages":      {"description"},
	"species":      {"creaves_species"},
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

		// Backfill fr translations from the just-present base rows. Runs
		// whether the base rows came from the dump or pre-existed, and is
		// idempotent per (table, locale) group.
		for _, table := range startupTables {
			fields, ok := startupTranslatableFields[table]
			if !ok {
				continue
			}
			if err := seedFrTranslations(tx, table, fields); err != nil {
				return err
			}
		}

		// Apply any shipped translations_<lang>.sql files (G11 pipeline).
		if err := applyTranslationFiles(tx); err != nil {
			return err
		}
		return nil
	})
}

// applyTranslationFiles executes embedded translations_<lang>.sql files.
// Each file is skipped wholesale when any row for its locale already exists.
func applyTranslationFiles(tx *pop.Connection) error {
	entries, err := translationSQLFS.ReadDir(".")
	if err != nil {
		return errors.WithStack(err)
	}
	for _, e := range entries {
		m := translationFileLocaleRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		locale := m[1]
		cnt, err := tx.Q().Where("locale = ?", locale).Count(&models.Translation{})
		if err != nil {
			return errors.WithStack(err)
		}
		if cnt > 0 {
			fmt.Printf("translations[%s]: %d rows present, skipping %s\n", locale, cnt, e.Name())
			continue
		}
		data, err := translationSQLFS.ReadFile(e.Name())
		if err != nil {
			return errors.WithStack(err)
		}
		_, stmts, err := utils.ExtractInsertStatements(bytes.NewReader(data))
		if err != nil {
			return errors.Wrapf(err, "parsing %s", e.Name())
		}
		n := 0
		for _, stmt := range stmts["translations"] {
			if err := tx.RawQuery(stmt).Exec(); err != nil {
				return errors.Wrapf(err, "applying %s", e.Name())
			}
			n++
		}
		fmt.Printf("translations[%s]: applied %d rows from %s\n", locale, n, e.Name())
	}
	return nil
}

// seedFrTranslations mirrors base columns of table into the translations
// table under locale fr. Skipped entirely when rows already exist for that
// (table, fr) group.
func seedFrTranslations(tx *pop.Connection, table string, fields []string) error {
	cnt, err := tx.Q().Where("table_name = ? AND locale = ?", table, "fr").Count(&models.Translation{})
	if err != nil {
		return errors.WithStack(err)
	}
	if cnt > 0 {
		fmt.Printf("translations %s[fr]: %d rows, skipping\n", table, cnt)
		return nil
	}

	n := 0
	for _, field := range fields {
		var rows []trRow
		if err := tx.Store.Select(&rows, "SELECT id, "+field+" AS value FROM "+table); err != nil {
			return errors.WithStack(err)
		}
		for _, row := range rows {
			if !row.Value.Valid || row.Value.String == "" {
				continue
			}
			if err := models.SaveTranslation(tx, table, row.ID, field, "fr", row.Value.String); err != nil {
				return errors.WithStack(err)
			}
			n++
		}
	}
	fmt.Printf("translations %s[fr]: seeded %d rows\n", table, n)
	return nil
}

// trRow is one (id, value) pair scanned from a startup table for fr
// translation backfill.
type trRow struct {
	ID    string       `db:"id"`
	Value nulls.String `db:"value"`
}

var _ = grift.Namespace("db", func() {
	grift.Desc("seed:startup", "Seeds the 7 startup reference tables from the embedded production dump (idempotent)")
	grift.Add("seed:startup", func(c *grift.Context) error {
		return seedStartup(c)
	})
})
