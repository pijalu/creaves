package actions

import (
	"creaves/models"
	"fmt"
	"sync"

	"github.com/gobuffalo/nulls"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// tnameDefaultField maps a table to its translatable display field when it
// differs from "name" (species translates its creaves_species column).
var tnameDefaultField = map[string]string{
	"species":         "creaves_species",
	"zones":           "zone",
	"native_statuses": "status",
	"subside_groups":  "group",
	"entry_causes":    "cause",
}

// tnameMiddleware registers the `tname` template helper on each request.
// Usage in plush: <%= tname("animaltypes", x.ID, x.Name) %> — renders the
// translated name for the current UI language, falling back to the base
// (canonical French) value. Translations are bulk-loaded once per
// (table, locale) per request and cached.
func tnameMiddleware() buffalo.MiddlewareFunc {
	return func(next buffalo.Handler) buffalo.Handler {
		return func(c buffalo.Context) error {
			lang := currentLang(c)

			var mu sync.Mutex
			// cache key: table + "\x00" + field
			cache := map[string]map[string]string{}

			c.Set("tname", func(table string, id interface{}, base interface{}) string {
				field := "name"
				if f, ok := tnameDefaultField[table]; ok {
					field = f
				}
				return tnameResolve(c, lang, field, table, id, baseString(base), cache, &mu)
			})
			c.Set("tfield", func(table, field string, id interface{}, base interface{}) string {
				return tnameResolve(c, lang, field, table, id, baseString(base), cache, &mu)
			})
			c.Set("tdesc", func(table string, id interface{}, base interface{}) string {
				return tnameResolve(c, lang, "description", table, id, baseString(base), cache, &mu)
			})
			// tbase resolves canonical reference values (used by grouped views whose
			// keys are display strings rather than source record IDs). The map is
			// loaded once per table/field/language for this request.
			baseCache := map[string]map[string]string{}
			c.Set("tbase", func(table, field string, base interface{}) string {
				b := baseString(base)
				if lang == "" || b == "" {
					return b
				}
				key := table + "\\x00" + field + "\\x00" + lang
				mu.Lock()
				m, loaded := baseCache[key]
				mu.Unlock()
				if !loaded {
					m = loadBaseTranslationMap(c, table, field, lang)
					mu.Lock()
					baseCache[key] = m
					mu.Unlock()
				}
				if v := m[b]; v != "" {
					return v
				}
				return b
			})
			// tdrug resolves a canonical (base) drug name — as stored in
			// free-text fields like treatments.drug — to its localized name
			// for the current UI language. Falls back to the canonical value.
			var drugMap map[string]string
			var drugLoaded bool
			c.Set("tdrug", func(base interface{}) string {
				b := baseString(base)
				if lang == "" || lang == "fr" || b == "" {
					return b
				}
				mu.Lock()
				loaded := drugLoaded
				mu.Unlock()
				if !loaded {
					drugMap = loadDrugNameMap(c, lang)
					mu.Lock()
					drugLoaded = true
					mu.Unlock()
				}
				mu.Lock()
				v, ok := drugMap[b]
				mu.Unlock()
				if ok {
					return v
				}
				if v := tnameResolveByBase(c, lang, "name", "drugs", b); v != "" {
					return v
				}
				return b
			})
			// tspecies resolves a canonical (base) species common name — as
			// stored in free-text fields like animals.species — to its localized
			// name for the current UI language. Falls back to the canonical value.
			var speciesMap map[string]string
			var speciesLoaded bool
			c.Set("tspecies", func(base interface{}) string {
				b := baseString(base)
				if lang == "" || lang == "fr" || b == "" {
					return b
				}
				mu.Lock()
				loaded := speciesLoaded
				mu.Unlock()
				if !loaded {
					speciesMap = loadSpeciesNameMap(c, lang)
					mu.Lock()
					speciesLoaded = true
					mu.Unlock()
				}
				mu.Lock()
				v, ok := speciesMap[b]
				mu.Unlock()
				if ok {
					return v
				}
				if v := tnameResolveByBase(c, lang, "creaves_species", "species", b); v != "" {
					return v
				}
				return b
			})
			// Per-request memo for fallback-by-base lookups (tnameResolveByBase):
			// listing pages in non-French locales can hit the fallback JOIN once
			// per row; memoize within the request to avoid repeated identical
			// queries.
			c.Set("tnameFallbackCache", &sync.Map{})
			return next(c)
		}
	}
}

// loadDrugNameMap builds canonical drug name -> localized name for lang.
// Returns nil when unavailable (no tx / query error).
func loadDrugNameMap(c buffalo.Context, lang string) map[string]string {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil
	}
	drugs := models.Drugs{}
	if err := tx.All(&drugs); err != nil {
		return nil
	}
	var rows []models.Translation
	tr := map[string]string{}
	if err := tx.Where("table_name = ? AND field = ? AND locale = ?", "drugs", "name", lang).All(&rows); err == nil {
		for _, r := range rows {
			tr[r.RecordID] = r.Value
		}
	}
	m := map[string]string{}
	for _, d := range drugs {
		if v, ok2 := tr[d.ID.String()]; ok2 && v != "" {
			m[d.Name] = v
		}
	}
	return m
}

// loadSpeciesNameMap builds canonical creaves_species name -> localized name
// for lang. Returns nil when unavailable (no tx / query error).
func loadSpeciesNameMap(c buffalo.Context, lang string) map[string]string {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return nil
	}
	species := []models.Species{}
	if err := tx.All(&species); err != nil {
		return nil
	}
	var rows []models.Translation
	tr := map[string]string{}
	if err := tx.Where("table_name = ? AND field = ? AND locale = ?", "species", "creaves_species", lang).All(&rows); err == nil {
		for _, r := range rows {
			tr[r.RecordID] = r.Value
		}
	}
	m := map[string]string{}
	for _, s := range species {
		if v, ok2 := tr[s.ID]; ok2 && v != "" {
			m[s.CreavesSpecies] = v
		}
	}
	return m
}

// translationBaseFields lists reference tables whose canonical display field can
// safely be used as a fallback when imported startup IDs differ from database IDs.
var translationBaseFields = map[string]string{
	"animalages":   "name",
	"animaltypes":  "name",
	"caretypes":    "name",
	"drugs":        "name",
	"outtaketypes": "name",
	"species":      "creaves_species",
}

// loadBaseTranslationMap loads translated values keyed by canonical French value.
// Base values are stable identifiers for reference data and avoid per-row queries.
func loadBaseTranslationMap(c buffalo.Context, table, field, lang string) map[string]string {
	out := map[string]string{}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return out
	}
	var rows []struct {
		Base  string `db:"base"`
		Value string `db:"value"`
	}
	// Table and field are selected only from the fixed reference mappings below.
	allowed := map[string]bool{"animaltypes": true, "animalages": true, "caretypes": true, "outtaketypes": true, "species": true, "zones": true, "native_statuses": true, "subside_groups": true, "entry_causes": true}
	if !allowed[table] {
		return out
	}
	baseField, ok := translationBaseFields[table]
	if table == "zones" {
		baseField = "zone"
	}
	if table == "native_statuses" {
		baseField = "status"
	}
	if table == "subside_groups" {
		baseField = "group"
	}
	if table == "entry_causes" {
		baseField = "cause"
	}
	if baseField == "" || field != baseField {
		return out
	}
	q := fmt.Sprintf("SELECT fr.value AS base, tr.value AS value FROM translations fr JOIN translations tr ON tr.table_name = fr.table_name AND tr.record_id = fr.record_id AND tr.field = fr.field AND tr.locale = ? WHERE fr.table_name = ? AND fr.field = ? AND fr.locale = 'fr' AND tr.value <> ''")
	if err := tx.RawQuery(q, lang, table, field).All(&rows); err != nil {
		return out
	}
	for _, row := range rows {
		out[row.Base] = row.Value
	}
	return out
}

func tnameResolveByBase(c buffalo.Context, lang, field, table, base string) string {
	// Memoize fallback lookups per request: a base-name miss fires a JOIN
	// query, and listing pages (landing, registers) can hit this path once
	// per rendered row in non-French locales — the same base value would
	// otherwise query repeatedly.
	if memo, ok := c.Value("tnameFallbackCache").(*sync.Map); ok && memo != nil {
		key := lang + "\x00" + table + "\x00" + field + "\x00" + base
		if v, loaded := memo.Load(key); loaded {
			s, _ := v.(string)
			return s
		}
		res := tnameResolveByBaseQuery(c, lang, field, table, base)
		memo.Store(key, res)
		return res
	}
	return tnameResolveByBaseQuery(c, lang, field, table, base)
}

// tnameResolveByBaseQuery performs the actual fallback lookup (uncached).
func tnameResolveByBaseQuery(c buffalo.Context, lang, field, table, base string) string {
	baseField, ok := translationBaseFields[table]
	if table == "outtaketypes" && field == "discoverer_news" {
		baseField, ok = "discoverer_news", true
	}
	if !ok || baseField != field || base == "" {
		return ""
	}
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return ""
	}
	var row struct {
		Value string `db:"value"`
	}
	q := fmt.Sprintf("SELECT tr.value FROM translations tr JOIN translations fr ON fr.table_name = tr.table_name AND fr.record_id = tr.record_id AND fr.field = tr.field AND fr.locale = 'fr' JOIN %s src ON src.%s = fr.value WHERE tr.table_name = ? AND tr.field = ? AND tr.locale = ? AND src.%s = ? AND tr.value <> fr.value LIMIT 1", table, baseField, baseField)
	if table == "outtaketypes" && field == "discoverer_news" {
		q = "SELECT tr.value FROM translations tr JOIN translations n ON n.table_name = 'outtaketypes' AND n.record_id = tr.record_id AND n.field = 'name' AND n.locale = 'fr' JOIN outtaketypes src ON src.name = n.value WHERE tr.table_name = 'outtaketypes' AND tr.field = ? AND tr.locale = ? AND src.name = ? AND tr.value <> '' LIMIT 1"
		if err := tx.RawQuery(q, field, lang, base).First(&row); err != nil {
			return ""
		}
		return row.Value
	}
	if err := tx.RawQuery(q, table, field, lang, base).First(&row); err != nil {
		return ""
	}
	return row.Value
}

func outtakeDiscovererNewsByName(c buffalo.Context, lang, id string) string {
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return ""
	}
	var row struct {
		Value string `db:"value"`
	}
	q := "SELECT tr.value FROM translations tr JOIN translations n ON n.table_name = 'outtaketypes' AND n.record_id = tr.record_id AND n.field = 'name' AND n.locale = 'fr' JOIN outtaketypes src ON src.name = n.value WHERE src.id = ? AND tr.table_name = 'outtaketypes' AND tr.field = 'discoverer_news' AND tr.locale = ? AND tr.value <> '' LIMIT 1"
	if err := tx.RawQuery(q, id, lang).First(&row); err != nil {
		return ""
	}
	return row.Value
}

func tnameResolve(c buffalo.Context, lang, field, table string, id interface{}, base string, cache map[string]map[string]string, mu *sync.Mutex) string {
	if lang == "" {
		return base
	}

	idStr, ok := id.(interface{ String() string })
	var idStrVal string
	if ok {
		idStrVal = idStr.String()
	} else if s, ok2 := id.(string); ok2 {
		idStrVal = s
	} else {
		return base
	}

	key := table + "\x00" + field

	mu.Lock()
	tr, loaded := cache[key]
	mu.Unlock()

	if !loaded {
		tx, ok := c.Value("tx").(*pop.Connection)
		if !ok {
			return base
		}
		// Load all translations for this (table, field, locale) in one query.
		var rows []models.Translation
		if err := tx.Where("table_name = ? AND field = ? AND locale = ?", table, field, lang).All(&rows); err != nil {
			return base
		}
		tr = map[string]string{}
		for _, r := range rows {
			tr[r.RecordID] = r.Value
		}
		mu.Lock()
		cache[key] = tr
		mu.Unlock()
	}

	if value := models.ResolveName(lang, base, tr, idStrVal); value != base {
		return value
	}
	if table == "outtaketypes" && field == "discoverer_news" {
		if value := outtakeDiscovererNewsByName(c, lang, idStrVal); value != "" {
			return value
		}
	}
	if value := tnameResolveByBase(c, lang, field, table, base); value != "" {
		return value
	}
	return base
}

// baseString coerces common plush/base value types to string (nulls.String,
// string, fmt.Stringer).
func baseString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case nulls.String:
		if !t.Valid {
			return ""
		}
		return t.String
	case interface{ String() string }:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
