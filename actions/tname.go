package actions

import (
	"creaves/models"
	"fmt"
	"sync"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

// tnameDefaultField maps a table to its translatable display field when it
// differs from "name" (species translates its creaves_species column).
var tnameDefaultField = map[string]string{
	"species": "creaves_species",
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
			c.Set("tdesc", func(table string, id interface{}, base interface{}) string {
				return tnameResolve(c, lang, "description", table, id, baseString(base), cache, &mu)
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
				return b
			})
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

// tnameResolve resolves a localized value with a per-request lazy cache.
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

	return models.ResolveName(lang, base, tr, idStrVal)
}

// baseString coerces common plush/base value types to string (nulls.String,
// string, fmt.Stringer).
func baseString(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case interface{ String() string }:
		return t.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}
