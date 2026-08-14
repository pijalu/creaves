package actions

import (
	"creaves/models"
	"fmt"
	"sync"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
)

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
				return tnameResolve(c, lang, "name", table, id, baseString(base), cache, &mu)
			})
			c.Set("tdesc", func(table string, id interface{}, base interface{}) string {
				return tnameResolve(c, lang, "description", table, id, baseString(base), cache, &mu)
			})
			return next(c)
		}
	}
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
