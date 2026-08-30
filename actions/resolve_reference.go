package actions

import (
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// Option A (I18N_UI_LOCALIZATION_FIX_PLAN.md): suggestion popups display
// localized reference names, but stored values stay canonical French so the
// webhook contract and free-text columns (animals.species, treatments.drug)
// are unaffected. resolveReferenceInput maps a user-submitted — possibly
// localized — value back to its canonical form before persisting or
// filtering. Canonical input passes through unchanged; unknown input passes
// through unchanged too (free text stays free text).

// refBaseField maps a reference table to its canonical display field.
var refBaseField = map[string]string{
	"species": "creaves_species",
	"drugs":   "name",
}

// resolveReferenceInput returns the canonical value for a submitted
// reference value, using the request language + transaction.
func resolveReferenceInput(c buffalo.Context, table, input string) string {
	lang := currentLang(c)
	tx, _ := c.Value("tx").(*pop.Connection)
	return resolveReferenceInputTx(tx, lang, table, input)
}

// resolveReferenceInputTx is the transaction/lang-based core of
// resolveReferenceInput (usable without a buffalo.Context, e.g. from query
// builders and tests). A nil tx or empty input returns the input unchanged.
func resolveReferenceInputTx(tx *pop.Connection, lang, table, input string) string {
	input = strings.TrimSpace(input)
	if input == "" || lang == "" || tx == nil {
		return input
	}
	baseField, ok := refBaseField[table]
	if !ok {
		return input
	}

	// Already canonical? (source column equality; MySQL collation is
	// case-insensitive by default, matching LIKE-based suggestions)
	exists, err := tx.Where(baseField+" = ?", input).Exists(table)
	if err == nil && exists {
		return input
	}

	// Localized submission: recover the canonical French value through the
	// translations pair (fr + lang) for the record referenced by the
	// localized translation.
	var canonical struct {
		Value string `db:"value"`
	}
	q := "SELECT fr.value AS value FROM translations fr " +
		"JOIN translations tr ON tr.table_name = fr.table_name AND tr.record_id = fr.record_id AND tr.field = fr.field AND tr.locale = ? " +
		"WHERE fr.table_name = ? AND fr.field = ? AND fr.locale = 'fr' AND tr.value = ? LIMIT 1"
	if err := tx.RawQuery(q, lang, table, baseField, input).First(&canonical); err == nil && canonical.Value != "" {
		return canonical.Value
	}

	return input
}

// localizeSuggestions maps canonical suggestion values to their localized
// labels for the request language. lang == "" (fr) or misses fall back to the
// canonical value. Uses the request-scoped base translation map.
func localizeSuggestions(c buffalo.Context, table, field string, values []string) []string {
	lang := currentLang(c)
	if lang == "" || len(values) == 0 {
		return values
	}
	m := loadBaseTranslationMap(c, table, field, lang)
	if len(m) == 0 {
		return values
	}
	out := make([]string, len(values))
	for i, v := range values {
		if t := m[v]; t != "" {
			out[i] = t
		} else {
			out[i] = v
		}
	}
	return out
}
