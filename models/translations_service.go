package models

import (
	"database/sql"

	"github.com/gobuffalo/pop/v6"
	"github.com/pkg/errors"
)

// LoadTranslations loads translations for one table/field/locale in a single
// batched query. The returned map is keyed by record_id.
func LoadTranslations(tx *pop.Connection, table, field, locale string, ids []string) (map[string]string, error) {
	out := map[string]string{}
	if len(ids) == 0 {
		return out, nil
	}

	args := make([]interface{}, 0, len(ids)+3)
	args = append(args, table, field, locale)
	inClause := ""
	for i, id := range ids {
		if i > 0 {
			inClause += ","
		}
		inClause += "?"
		args = append(args, id)
	}

	var rows []Translation
	q := tx.RawQuery("SELECT record_id, value FROM translations WHERE table_name = ? AND field = ? AND locale = ? AND record_id IN ("+inClause+")", args...)
	if err := q.All(&rows); err != nil {
		return nil, errors.WithStack(err)
	}
	for _, r := range rows {
		out[r.RecordID] = r.Value
	}
	return out, nil
}

// ResolveName returns the translated value for id if present, else the base
// (canonical French) value.
func ResolveName(lang, base string, tr map[string]string, id string) string {
	if lang != "" && lang != "fr" {
		if v, ok := tr[id]; ok && v != "" {
			return v
		}
	}
	return base
}

// SaveTranslation upserts a translation (SELECT then INSERT or UPDATE — pop
// has no portable ON DUPLICATE KEY support).
func SaveTranslation(tx *pop.Connection, table, id, field, locale, value string) error {
	var existing Translation
	err := tx.Where("table_name = ? AND record_id = ? AND field = ? AND locale = ?", table, id, field, locale).First(&existing)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return errors.WithStack(err)
	}
	if err == nil {
		existing.Value = value
		return errors.WithStack(tx.Update(&existing))
	}
	t := &Translation{
		TableName: table,
		RecordID:  id,
		Field:     field,
		Locale:    locale,
		Value:     value,
	}
	return errors.WithStack(tx.Create(t))
}
