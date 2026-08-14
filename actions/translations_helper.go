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
	Name  string // form input name
	Value string
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
			}
			rows = append(rows, trRowView{Locale: loc, Field: field, Name: trFormKey(loc, field), Value: v})
		}
	}
	c.Set("trRows", rows)
	return nil
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
