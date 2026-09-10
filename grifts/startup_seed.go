package grifts

import (
	"bytes"
	"compress/gzip"
	"embed"
	"fmt"
	"regexp"
	"strings"

	"creaves/models"
	"creaves/utils"

	"github.com/gobuffalo/grift/grift"
	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// startupRowNamespace is the fixed uuid v5 namespace used to derive a stable
// replacement id when a dump row's id is taken by a row with a DIFFERENT
// canonical name (see syncStartupTable).
var startupRowNamespace = uuid.Must(uuid.FromString("6ba7b812-9dad-11d1-80b4-00c04fd430c8"))

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

// rowID returns the primary-key value of a parsed dump row. The dump uses
// lowercase `id` everywhere except `species`, whose PK column is `ID`.
func rowID(row map[string]string) string {
	for k, v := range row {
		if strings.EqualFold(k, "id") {
			return v
		}
	}
	return ""
}

// startupFKColumns lists child tables whose dump rows carry foreign keys into
// other startup tables. When such a row is inserted, the FK values must be
// re-keyed through the id mapping built from the parent tables' sync: a
// name-matched parent keeps its EXISTING database id, so dump ids appearing
// in child rows must be translated or the INSERT violates the FK constraint.
var startupFKColumns = map[string][]string{
	"dosages": {"animaltype_id", "drug_id"},
}

// startupNameField lists startup tables whose rows are matched between the
// embedded dump and an existing database by normalized canonical name (display
// name column). Tables not listed here are matched by primary key only.
// Production databases are pre-seeded with reference rows whose ids may drift
// from the dump; matching by name lets the seed attach shipped translations to
// the EXISTING rows instead of duplicating them.
var startupNameField = map[string]string{
	"animalages":   "name",
	"animaltypes":  "name",
	"caretypes":    "name",
	"outtaketypes": "name",
	"drugs":        "name",
}

// seedStartup syncs the embedded production reference data (French) into the
// 7 startup tables. It is add-only: rows that already exist are preserved
// (production databases must never be rewritten), rows whose id or canonical
// name matches stay untouched, and only genuinely new items are inserted.
// All writes run in a single transaction.
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
		// table -> dump record id -> target record id (identity when the dump
		// row exists under its own id; the existing row id on a name match).
		mapping := map[string]map[string]string{}
		// fkMap accumulates every table's dump-id -> target-id mapping; it is
		// passed to later syncStartupTable calls so child-table inserts can
		// re-key their FK columns (dosages reference drugs + animaltypes).
		fkMap := map[string]string{}
		for _, table := range ordered {
			model, ok := startupModels[table]
			if !ok {
				fmt.Printf("%s: no model registered, skipping\n", table)
				continue
			}
			if _, known := startupTableColumns[table]; !known {
				cnt, err := tx.Q().Count(model)
				if err != nil {
					return errors.WithStack(err)
				}
				// Unknown extra table: legacy behavior — seed only when empty.
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
				continue
			}
			m, err := syncStartupTable(tx, table, stmts[table], fkMap)
			if err != nil {
				return errors.WithStack(errors.Wrapf(err, "syncing %s", table))
			}
			mapping[table] = m
			for dumpID, target := range m {
				fkMap[dumpID] = target
			}
		}

		// Backfill fr translations from the base rows (existing + newly
		// inserted). Idempotent per (table, record, field).
		for _, table := range startupTables {
			fields, ok := startupTranslatableFields[table]
			if !ok {
				continue
			}
			if err := seedFrTranslations(tx, table, fields); err != nil {
				return err
			}
		}

		// Apply any shipped translations_<lang>.sql files (G11 pipeline),
		// re-keying artifact rows through the name-match mapping.
		if err := applyTranslationFiles(tx, mapping); err != nil {
			return err
		}
		return nil
	})
}

// syncStartupTable aligns one startup table with the embedded dump without
// modifying or deleting any existing row. For each dump row:
//   - the id already exists      -> keep the DB row (identity mapping)
//   - the name matches (normalized; exact first, then a unique prefix match)
//     an existing row            -> keep the DB row and map the dump id onto
//     it, so shipped translations attach to the existing record (no duplicate)
//   - otherwise                  -> genuinely new item: INSERT the dump row
//
// Returns the mapping dump record id -> target record id.
func syncStartupTable(tx *pop.Connection, table string, statements []string, fkMapping map[string]string) (map[string]string, error) {
	columns := startupTableColumns[table]
	nameField := startupNameField[table]

	// Cache existing ids and names once; per-row COUNT queries are prohibitively
	// expensive for the large embedded dump and provide no additional state.
	existingIDs := map[string]bool{}
	existingNames := map[string]string{}
	var rows []trRow
	query := "SELECT `id`"
	if nameField != "" {
		query += ", `" + nameField + "` AS `value`"
	}
	query += " FROM `" + table + "`"
	if err := tx.Store.Select(&rows, query); err != nil {
		return nil, errors.WithStack(err)
	}
	for _, row := range rows {
		existingIDs[row.ID] = true
		if row.Value.Valid {
			existingNames[row.ID] = row.Value.String
		}
	}

	tuples, err := splitRawTuples(statements)
	if err != nil {
		return nil, err
	}

	usedTargets := map[string]bool{} // record ids already claimed by a dump row
	normIndex := map[string]string{} // normalized name -> existing id
	for id, raw := range existingNames {
		n := normKey(raw)
		if n != "" {
			normIndex[n] = id
		}
	}

	mapping := map[string]string{}
	inserted, kept, matched := 0, 0, 0
	for i, stmt := range statements {
		parsed, err := utils.ParseInsertRows(stmt, columns)
		if err != nil {
			return nil, errors.WithStack(err)
		}
		if len(parsed) != len(tuples[i]) {
			return nil, fmt.Errorf("%s: parsed %d rows but split %d raw tuples", table, len(parsed), len(tuples[i]))
		}
		for j, row := range parsed {
			id := rowID(row)
			if id == "" {
				return nil, fmt.Errorf("%s: dump tuple %d has empty id", table, j)
			}
			raw := tuples[i][j]
			if existingIDs[id] {
				// Id exists. When the table has a canonical name, decide by
				// name: identical (normalized) name -> keep the existing row
				// (identity mapping); different name -> the dump id is taken
				// by a DIFFERENT concept in this database, so the dump row is
				// a genuinely new item and must be inserted under a fresh,
				// deterministic id (never reuse or rewrite the existing row).
				if nameField != "" && !strings.EqualFold(normKey(existingNames[id]), normKey(row[nameField])) {
					// The dump name may still exist under ANOTHER id (the
					// unique name index would reject a duplicate insert):
					// attach to it before considering the dump row new.
					if id2 := matchRefName(normIndex, usedTargets, normKey(row[nameField])); id2 != "" {
						mapping[id] = id2
						usedTargets[id2] = true
						matched++
						continue
					}
					newID := uuid.NewV5(startupRowNamespace, table+"|"+strings.TrimSpace(row[nameField])).String()
					if !usedTargets[newID] {
						raw = strings.Replace(raw, "'"+id+"'", "'"+newID+"'", 1)
						if err := tx.RawQuery("INSERT INTO `" + table + "` VALUES " + raw).Exec(); err != nil {
							return nil, errors.WithStack(errors.Wrapf(err, "inserting %s row %s (renamed %s)", table, newID, id))
						}
						existingNames[newID] = row[nameField]
						existingIDs[newID] = true
						if n := normKey(row[nameField]); n != "" {
							normIndex[n] = newID
						}
						usedTargets[newID] = true
						mapping[id] = newID
						inserted++
						continue
					}
					// Same dump name already inserted under newID: point this
					// row's translations at that target instead.
					mapping[id] = newID
					matched++
					continue
				}
				mapping[id] = id
				usedTargets[id] = true
				kept++
				continue
			}
			if nameField != "" {
				if id2 := matchRefName(normIndex, usedTargets, normKey(row[nameField])); id2 != "" {
					mapping[id] = id2
					usedTargets[id2] = true
					matched++
					continue
				}
			}
			// Re-key FK columns through the PARENT tables' id mapping
			// (quoted exact match inside the raw tuple; ids are
			// self-delimiting quoted tokens, so this cannot hit a
			// substring of another value).
			for _, fk := range startupFKColumns[table] {
				if target, ok := fkMapping[row[fk]]; ok && target != row[fk] {
					raw = strings.Replace(raw, "'"+row[fk]+"'", "'"+target+"'", 1)
				}
			}
			if err := tx.RawQuery("INSERT INTO `" + table + "` VALUES " + raw).Exec(); err != nil {
				return nil, errors.WithStack(errors.Wrapf(err, "inserting %s row %s", table, id))
			}
			if nameField != "" {
				existingNames[id] = row[nameField]
				existingIDs[id] = true
				if n := normKey(row[nameField]); n != "" {
					normIndex[n] = id
				}
			}
			usedTargets[id] = true
			mapping[id] = id
			inserted++
		}
	}
	fmt.Printf("%s: %d kept, %d name-matched, %d inserted\n", table, kept, matched, inserted)
	return mapping, nil
}

// matchRefName finds an existing record id whose normalized name matches norm.
// Exact normalized equality wins; otherwise a unique prefix match (both sides
// at least 5 chars) absorbs accent/spelling drift between dump generations
// (e.g. "Relacher" vs "Relaché", "Transferer" vs "Transféré"). Ambiguous or
// too-weak matches return "" so the dump row is treated as a new item.
func matchRefName(normIndex map[string]string, used map[string]bool, norm string) string {
	if norm == "" {
		return ""
	}
	if id, ok := normIndex[norm]; ok && !used[id] {
		return id
	}
	candidates := []string{}
	for n, id := range normIndex {
		if used[id] || len(n) < 5 || len(norm) < 5 {
			continue
		}
		if strings.HasPrefix(n, norm) || strings.HasPrefix(norm, n) {
			candidates = append(candidates, id)
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return ""
}

// normKey normalizes a canonical name for matching: lowercase, accents
// folded, punctuation/quote variants treated as separators, whitespace
// collapsed.
func normKey(s string) string {
	s = strings.ToLower(s)
	s = accentReplacer.Replace(s)
	return strings.Join(strings.Fields(s), " ")
}

var accentReplacer = strings.NewReplacer(
	"à", "a", "á", "a", "â", "a", "ä", "a", "ã", "a", "å", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e", "í", "i", "ì", "i", "î", "i", "ï", "i",
	"ó", "o", "ò", "o", "ô", "o", "ö", "o", "õ", "o",
	"ú", "u", "ù", "u", "û", "u", "ü", "u",
	"ý", "y", "ÿ", "y", "ç", "c", "ñ", "n",
	"œ", "oe", "æ", "ae", "ß", "ss",
	"'", " ", "’", " ", "`", " ", "´", " ",
	"-", " ", "_", " ", ".", " ", ",", " ", "/", " ", "(", " ", ")", " ",
	":", " ", ";", " ", "“", " ", "”", " ", "«", " ", "»", " ",
)

// splitRawTuples splits INSERT ... VALUES statements into raw "(...)" tuple
// strings, preserving the original SQL text (quote-aware, top-level parens)
// so dump rows can be re-inserted verbatim.
func splitRawTuples(statements []string) ([][]string, error) {
	out := make([][]string, 0, len(statements))
	for _, statement := range statements {
		up := strings.ToUpper(statement)
		v := strings.Index(up, " VALUES")
		if v < 0 {
			return nil, fmt.Errorf("INSERT has no VALUES clause")
		}
		body := strings.TrimSpace(strings.TrimSuffix(statement[v+len(" VALUES"):], ";"))
		var tuples []string
		depth, start := 0, -1
		inQuote := false
		for i := 0; i < len(body); i++ {
			c := body[i]
			if inQuote {
				if c == '\\' {
					i++
					continue
				}
				if c == '\'' {
					inQuote = false
				}
				continue
			}
			switch c {
			case '\'':
				inQuote = true
			case '(':
				if depth == 0 {
					start = i
				}
				depth++
			case ')':
				depth--
				if depth == 0 {
					tuples = append(tuples, body[start:i+1])
					start = -1
				}
			}
		}
		if inQuote || depth != 0 {
			return nil, fmt.Errorf("unbalanced INSERT tuples")
		}
		out = append(out, tuples)
	}
	return out, nil
}

// applyTranslationFiles applies embedded translations_<lang>.sql files,
// re-keying each artifact row's record_id through mapping (dump id -> existing
// row id, identity when the dump row exists under its own id). French rows are
// skipped — they are derived from the base columns by seedFrTranslations.
// Semantics per artifact row, keyed by (table, record, field, locale):
//   - absent            -> INSERT (missing translation)
//   - present under the artifact's primary key id -> UPDATE value (corrected
//     translations reach databases seeded previously)
//   - present under a different id -> left untouched (administrator edits are
//     preserved)
func applyTranslationFiles(tx *pop.Connection, mapping map[string]map[string]string) error {
	entries, err := translationSQLFS.ReadDir(".")
	if err != nil {
		return errors.WithStack(err)
	}

	// Existing translations for the startup tables: key -> (pk id, value).
	type trKey struct{ table, record, field, locale string }
	type trState struct {
		id, value string
	}
	existing := map[trKey]trState{}
	var rows []models.Translation
	if err := tx.Where("table_name IN (?)", startupTables).All(&rows); err != nil {
		return errors.WithStack(err)
	}
	for _, r := range rows {
		existing[trKey{r.TableName, r.RecordID, r.Field, r.Locale}] = trState{id: r.ID.String(), value: r.Value}
	}

	inserts := [][]interface{}{} // args for chunked batch INSERT
	updated := 0
	for _, e := range entries {
		m := translationFileLocaleRe.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		locale := m[1]
		if locale == "fr" {
			continue // derived from base columns by seedFrTranslations
		}

		data, err := translationSQLFS.ReadFile(e.Name())
		if err != nil {
			return errors.WithStack(err)
		}
		_, stmts, err := utils.ExtractInsertStatements(bytes.NewReader(data))
		if err != nil {
			return errors.Wrapf(err, "parsing %s", e.Name())
		}
		columns := []string{"id", "table_name", "record_id", "field", "locale", "value", "created_at", "updated_at"}
		for _, stmt := range stmts["translations"] {
			parsed, err := utils.ParseInsertRows(stmt, columns)
			if err != nil {
				return errors.Wrapf(err, "parsing %s", e.Name())
			}
			for _, row := range parsed {
				table := row["table_name"]
				record := row["record_id"]
				if tmap, ok := mapping[table]; ok {
					if target, ok2 := tmap[record]; ok2 {
						record = target
					}
				}
				k := trKey{table, record, row["field"], row["locale"]}
				pk := row["id"]
				value := row["value"]
				if prev, ok := existing[k]; ok {
					if prev.id == pk && prev.value != value {
						if err := tx.RawQuery("UPDATE translations SET value = ?, updated_at = NOW() WHERE id = ?", value, pk).Exec(); err != nil {
							return errors.Wrapf(err, "updating %s", e.Name())
						}
						updated++
					}
					continue
				}
				existing[k] = trState{id: pk, value: value}
				inserts = append(inserts, []interface{}{pk, table, record, row["field"], row["locale"], value})
			}
		}
		fmt.Printf("translations[%s]: applied from %s\n", locale, e.Name())
	}

	for start := 0; start < len(inserts); start += 100 {
		end := start + 100
		if end > len(inserts) {
			end = len(inserts)
		}
		args := []interface{}{}
		clause := ""
		for _, ins := range inserts[start:end] {
			clause += "(?,?,?,?,?,?,NOW(),NOW()),"
			args = append(args, ins...)
		}
		clause = strings.TrimSuffix(clause, ",")
		q := "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES " +
			clause + " ON DUPLICATE KEY UPDATE record_id = VALUES(record_id), value = VALUES(value), updated_at = NOW()"
		if err := tx.RawQuery(q, args...).Exec(); err != nil {
			return errors.WithStack(err)
		}
	}
	fmt.Printf("translations: inserted %d, updated %d\n", len(inserts), updated)
	return nil
}

// seedFrTranslations mirrors base columns of table into the translations
// table under locale fr. Missing rows are inserted; rows whose value drifted
// from the base column (e.g. corrupted escaping in earlier artifacts) are
// corrected — fr is a generated mirror of the canonical column, not
// administrator-managed content.
const startupTranslationBatchSize = 100

func seedFrTranslations(tx *pop.Connection, table string, fields []string) error {
	// Load all existing French rows once, then reconcile in memory. This keeps
	// idempotency and generated-value correction semantics without one lookup per
	// source row.
	var existingRows []models.Translation
	if err := tx.Where("table_name = ? AND locale = ?", table, "fr").All(&existingRows); err != nil {
		return errors.WithStack(err)
	}
	source := make(map[string][]trRow, len(fields))
	for _, field := range fields {
		var rows []trRow
		if err := tx.Store.Select(&rows, "SELECT `id`, `"+field+"` AS `value` FROM `"+table+"`"); err != nil {
			return errors.WithStack(err)
		}
		source[field] = rows
	}
	inserts, updates := reconcileFrTranslations(table, fields, existingRows, source)

	// Batch INSERTs; unique key remains the final idempotency guard.
	for start := 0; start < len(inserts); start += startupTranslationBatchSize {
		end := start + startupTranslationBatchSize
		if end > len(inserts) {
			end = len(inserts)
		}
		args := make([]interface{}, 0, (end-start)*6)
		clause := make([]string, 0, end-start)
		for _, row := range inserts[start:end] {
			clause = append(clause, "(?,?,?,?,?,?,NOW(),NOW())")
			args = append(args, row.ID.String(), row.TableName, row.RecordID, row.Field, row.Locale, row.Value)
		}
		q := "INSERT INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES " + strings.Join(clause, ",") + " ON DUPLICATE KEY UPDATE value = VALUES(value), updated_at = NOW()"
		if err := tx.RawQuery(q, args...).Exec(); err != nil {
			return errors.WithStack(err)
		}
	}
	// Corrections intentionally update only rows identified by their existing
	// primary key, preserving any administrator-created conflicting row.
	for start := 0; start < len(updates); start += startupTranslationBatchSize {
		end := start + startupTranslationBatchSize
		if end > len(updates) {
			end = len(updates)
		}
		args := make([]interface{}, 0, (end-start)*3)
		cases := make([]string, 0, end-start)
		placeholders := make([]string, 0, end-start)
		ids := make([]string, 0, end-start)
		for _, row := range updates[start:end] {
			id := row.ID.String()
			cases = append(cases, "WHEN ? THEN ?")
			args = append(args, id, row.Value)
			placeholders = append(placeholders, "?")
			ids = append(ids, id)
		}
		args = append(args, idsToInterfaces(ids)...)
		q := "UPDATE translations SET value = CASE id " + strings.Join(cases, " ") + " END, updated_at = NOW() WHERE id IN (" + strings.Join(placeholders, ",") + ")"
		if err := tx.RawQuery(q, args...).Exec(); err != nil {
			return errors.WithStack(err)
		}
	}
	fmt.Printf("translations %s[fr]: seeded %d rows, corrected %d\n", table, len(inserts), len(updates))
	return nil
}

// trRow is one (id, value) pair scanned from a startup table for fr
// translation backfill.
type trRow struct {
	ID    string       `db:"id"`
	Value nulls.String `db:"value"`
}

// frTranslationKey identifies one generated French mirror row.
type frTranslationKey struct{ record, field string }

func reconcileFrTranslations(table string, fields []string, existingRows []models.Translation, source map[string][]trRow) (inserts, updates []models.Translation) {
	existing := make(map[frTranslationKey]models.Translation, len(existingRows))
	for _, row := range existingRows {
		existing[frTranslationKey{row.RecordID, row.Field}] = row
	}
	for _, field := range fields {
		for _, row := range source[field] {
			if !row.Value.Valid || row.Value.String == "" {
				continue
			}
			key := frTranslationKey{row.ID, field}
			if current, ok := existing[key]; ok {
				if current.Value != row.Value.String {
					current.Value = row.Value.String
					updates = append(updates, current)
				}
				continue
			}
			newRow := models.Translation{ID: uuid.Must(uuid.NewV4()), TableName: table, RecordID: row.ID, Field: field, Locale: "fr", Value: row.Value.String}
			existing[key] = newRow
			inserts = append(inserts, newRow)
		}
	}
	return inserts, updates
}

func idsToInterfaces(ids []string) []interface{} {
	args := make([]interface{}, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return args
}

var _ = grift.Namespace("db", func() {
	grift.Desc("seed:startup", "Seeds the 7 startup reference tables from the embedded production dump (idempotent)")
	grift.Add("seed:startup", func(c *grift.Context) error {
		return seedStartup(c)
	})
})
