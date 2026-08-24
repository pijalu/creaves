package actions

import (
	"fmt"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// translationLocales are the editable (non-base) locales in admin forms.
var translationLocales = []string{"en-US", "de", "nl"}

// trFormKey is the form field name for one translation input.
func trFormKey(locale, field string) string {
	return "tr_" + strings.ReplaceAll(locale, "-", "_") + "_" + field
}

// trRowView is one translation input row for the _fields partial.
type trRowView struct {
	Locale string
	Field  string
	Name   string // form input name
	Value  string
}

// setTranslationValues loads existing translations for a record and sets a
// flat []trRowView for the _fields partial.
func setTranslationValues(c buffalo.Context, tx *pop.Connection, table, id string, fields []string) error {
	rows := []trRowView{}
	for _, loc := range translationLocales {
		for _, field := range fields {
			v := ""
			if id != "" {
				tr, err := models.LoadTranslations(tx, table, field, loc, []string{id})
				if err != nil {
					return err
				}
				v = tr[id]
				if v == "" {
					v, err = translationValueByCanonicalValue(tx, table, id, field, loc)
					if err != nil {
						return err
					}
				}
			}
			rows = append(rows, trRowView{Locale: loc, Field: field, Name: trFormKey(loc, field), Value: v})
		}
	}
	c.Set("trRows", rows)
	return nil
}

// translationValueByCanonicalValue recovers translations imported with a stale
// record ID by matching the source row's canonical French value. Reference
// data IDs can change when startup data is reloaded, while canonical values are
// stable and are already used by the display translation helpers.
func translationValueByCanonicalValue(tx *pop.Connection, table, id, field, locale string) (string, error) {
	allowed := map[string]map[string]bool{
		"animalages":      {"name": true, "description": true},
		"animaltypes":     {"name": true, "description": true, "default_species": true},
		"caretypes":       {"name": true, "description": true},
		"drugs":           {"name": true, "description": true},
		"outtaketypes":    {"name": true, "description": true, "discoverer_news": true},
		"species":         {"creaves_species": true},
		"zones":           {"zone": true, "type": true},
		"entry_causes":    {"cause": true, "detail": true, "nature": true, "indication": true},
		"native_statuses": {"status": true, "indication": true, "precision": true},
		"subside_groups":  {"group": true},
	}
	if !allowed[table][field] {
		return "", nil
	}
	var row struct {
		Base  string `db:"base"`
		Value string `db:"value"`
	}
	query := fmt.Sprintf(`SELECT fr.value AS base, tr.value AS value
		FROM %s src
		JOIN translations fr ON fr.table_name = ? AND fr.record_id = src.id AND fr.field = ? AND fr.locale = 'fr'
		JOIN translations tr ON tr.table_name = fr.table_name AND tr.record_id = fr.record_id AND tr.field = fr.field AND tr.locale = ?
		WHERE src.id = ? AND fr.value <> '' AND tr.value <> ''
		UNION ALL
		SELECT fr.value AS base, tr.value AS value
		FROM translations fr
		JOIN translations tr ON tr.table_name = fr.table_name AND tr.record_id = fr.record_id AND tr.field = fr.field AND tr.locale = ?
		JOIN %s src ON src.%s = fr.value
		WHERE fr.table_name = ? AND fr.field = ? AND fr.locale = 'fr' AND src.id = ? AND tr.value <> '' LIMIT 1`, table, table, field)
	if err := tx.RawQuery(query, table, field, locale, id, locale, table, field, id).First(&row); err != nil {
		return "", nil
	}
	return row.Value, nil
}

// saveTranslations persists non-empty tr_<locale>_<field> form values.
// Empty values are ignored (no deletion of existing translations).
func saveTranslations(c buffalo.Context, tx *pop.Connection, table, id string, fields []string) error {
	if err := c.Request().ParseForm(); err != nil {
		return err
	}
	for _, loc := range translationLocales {
		for _, field := range fields {
			v := strings.TrimSpace(c.Request().FormValue(trFormKey(loc, field)))
			if v == "" {
				continue
			}
			if err := models.SaveTranslation(tx, table, id, field, loc, v); err != nil {
				return fmt.Errorf("save translation %s/%s/%s: %w", table, field, loc, err)
			}
		}
	}
	return nil
}
