package grifts

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"creaves/models"

	"github.com/gobuffalo/grift/grift"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

var langArgRe = regexp.MustCompile(`^--lang=(en-US|de|nl)$`)

// exportTranslations writes translations_<lang>.sql with one commented
// INSERT per (table, record, field) present in the fr translations, ready
// for translators to fill in.
func exportTranslations(c *grift.Context) error {
	lang := ""
	for _, a := range c.Args {
		if m := langArgRe.FindStringSubmatch(a); m != nil {
			lang = m[1]
		}
	}
	if lang == "" {
		return errors.New("usage: buffalo task i18n:export:translations --lang=en-US|de|nl")
	}

	var b strings.Builder
	b.WriteString("-- Translation skeleton for locale " + lang + "\n")
	b.WriteString("-- Fill in the '' value with the translated text, then ship the file as\n")
	b.WriteString("-- grifts/translations_" + lang + ".sql — db:seed:startup applies it.\n\n")

	total := 0
	for _, table := range startupTables {
		fields := startupTranslatableFields[table]
		if len(fields) == 0 {
			continue
		}
		var frs []models.Translation
		if err := models.DB.Where("table_name = ? AND locale = ?", table, "fr").Order("record_id, field").All(&frs); err != nil {
			return errors.WithStack(err)
		}
		// Existing translations for target lang (skip those).
		have := map[string]bool{}
		var existing []models.Translation
		if err := models.DB.Where("table_name = ? AND locale = ?", table, lang).All(&existing); err != nil {
			return errors.WithStack(err)
		}
		for _, e := range existing {
			have[e.RecordID+"/"+e.Field] = true
		}

		b.WriteString("-- table: " + table + "\n")
		for _, fr := range frs {
			if have[fr.RecordID+"/"+fr.Field] {
				continue
			}
			id := uuid.Must(uuid.NewV4()).String()
			base := strings.ReplaceAll(fr.Value, "'", "''")

			fmt.Fprintf(&b, "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('%s', '%s', '%s', '%s', '%s', '', NOW(), NOW()); -- base: %s\n",
				id, table, fr.RecordID, fr.Field, lang, base)
			total++
		}
		b.WriteString("\n")
	}

	fname := "translations_" + lang + ".sql"
	if err := os.WriteFile(fname, []byte(b.String()), 0644); err != nil {
		return errors.WithStack(err)
	}
	fmt.Printf("wrote %s with %d skeleton rows\n", fname, total)
	return nil
}

var _ = grift.Namespace("i18n", func() {
	grift.Desc("export:translations", "Writes translations_<lang>.sql skeleton (--lang=en-US|de|nl)")
	grift.Add("export:translations", func(c *grift.Context) error {
		return exportTranslations(c)
	})
})
