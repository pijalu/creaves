package grifts

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"embed"
	"fmt"
	"regexp"
	"strings"

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
var translationFileLocaleRe = regexp.MustCompile(`^translations_([a-z]{2}(?:-[A-Z]{2})?)\.sql$`)

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
	"animaltypes":  {"name", "description", "default_species"},
	"caretypes":    {"name", "description"},
	"outtaketypes": {"name", "description", "discoverer_news"},
	"drugs":        {"name", "description"},
	"dosages":      {"description", "dosage_per_grams_unit"},
	"species":      {"species", "class", "family", "creaves_species", "subside_group", "order", "agw_group", "native_status"},
}

// startupTableColumns records dump/schema order for INSERT statements without
// column lists. Keep this beside startupTranslatableFields: parser and
// inventory tests therefore share one authoritative startup definition.
var startupTableColumns = map[string][]string{
	"animalages":   {"id", "name", "description", "def", "created_at", "updated_at"},
	"animaltypes":  {"id", "name", "description", "def", "created_at", "updated_at", "has_ring", "default_species"},
	"caretypes":    {"id", "name", "description", "def", "warning", "reset_warning", "created_at", "updated_at", "type"},
	"outtaketypes": {"id", "name", "description", "def", "created_at", "updated_at", "dead", "rating", "discoverer_news", "error"},
	"drugs":        {"id", "name", "description", "created_at", "updated_at"},
	"dosages":      {"id", "animaltype_id", "drug_id", "enabled", "description", "dosage_per_grams", "dosage_per_grams_unit", "created_at", "updated_at"},
	"species":      {"ID", "species", "class", "family", "creaves_species", "subside_group", "created_at", "updated_at", "order", "game", "agw_group", "native_status", "huntable"},
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
// A locale whose row count already matches its artifact is skipped. Partial
// locales are filled from shipped artifacts; existing values are updated so
// corrected translations reach databases seeded previously.
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

		data, err := translationSQLFS.ReadFile(e.Name())
		if err != nil {
			return errors.WithStack(err)
		}
		_, stmts, err := utils.ExtractInsertStatements(bytes.NewReader(data))
		if err != nil {
			return errors.Wrapf(err, "parsing %s", e.Name())
		}
		statements := stmts["translations"]
		n := 0
		for _, stmt := range statements {
			stmt = strings.TrimSuffix(strings.TrimSpace(stmt), ";") + " ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = VALUES(updated_at);"
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
	n := 0
	for _, field := range fields {
		var rows []trRow
		if err := tx.Store.Select(&rows, "SELECT `id`, `"+field+"` AS `value` FROM `"+table+"`"); err != nil {
			return errors.WithStack(err)
		}
		for _, row := range rows {
			if !row.Value.Valid || row.Value.String == "" {
				continue
			}
			var existing models.Translation
			err := tx.Where("table_name = ? AND record_id = ? AND field = ? AND locale = ?", table, row.ID, field, "fr").First(&existing)
			if err == nil {
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return errors.WithStack(err)
			}
			if err := tx.Create(&models.Translation{TableName: table, RecordID: row.ID, Field: field, Locale: "fr", Value: row.Value.String}); err != nil {
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
