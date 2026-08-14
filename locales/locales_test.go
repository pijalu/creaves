package locales

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

type translationEntry struct {
	ID          string `yaml:"id"`
	Translation string `yaml:"translation"`
}

// loadIDs parses a single locale file and returns its translation ids.
func loadIDs(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := files.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var entries []translationEntry
	if err := yaml.Unmarshal(data, &entries); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	ids := map[string]bool{}
	for _, e := range entries {
		if e.ID == "" {
			t.Fatalf("%s: entry with empty id", path)
		}
		if ids[e.ID] {
			t.Fatalf("%s: duplicate id %q", path, e.ID)
		}
		ids[e.ID] = true
	}
	return ids
}

// TestLocaleKeyParity ensures every domain file has an identical id set in
// all four supported locales (en-us, fr, de, nl).
func TestLocaleKeyParity(t *testing.T) {
	locales := []string{"en-us", "fr", "de", "nl"}

	domains := map[string]map[string]map[string]bool{} // domain -> locale -> ids

	dir, err := files.ReadDir(".")
	if err != nil {
		t.Fatalf("read locales dir: %v", err)
	}
	for _, de := range dir {
		name := de.Name()
		if !strings.HasSuffix(name, ".yaml") {
			continue
		}
		base := strings.TrimSuffix(name, ".yaml")
		idx := strings.LastIndex(base, ".")
		if idx < 0 {
			t.Fatalf("unexpected locale file name: %s", name)
		}
		domain, locale := base[:idx], base[idx+1:]
		if domains[domain] == nil {
			domains[domain] = map[string]map[string]bool{}
		}
		domains[domain][locale] = loadIDs(t, name)
	}

	if len(domains) == 0 {
		t.Fatal("no locale files found")
	}

	var failures []string
	for domain, byLocale := range domains {
		ref, ok := byLocale["en-us"]
		if !ok {
			failures = append(failures, fmt.Sprintf("%s: missing en-us file", domain))
			continue
		}
		for _, locale := range locales {
			ids, ok := byLocale[locale]
			if !ok {
				failures = append(failures, fmt.Sprintf("%s: missing %s file", domain, locale))
				continue
			}
			var missing, extra []string
			for id := range ref {
				if !ids[id] {
					missing = append(missing, id)
				}
			}
			for id := range ids {
				if !ref[id] {
					extra = append(extra, id)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				failures = append(failures, fmt.Sprintf("%s.%s: missing %d keys: %s", domain, locale, len(missing), strings.Join(missing, ", ")))
			}
			if len(extra) > 0 {
				sort.Strings(extra)
				failures = append(failures, fmt.Sprintf("%s.%s: %d extra keys: %s", domain, locale, len(extra), strings.Join(extra, ", ")))
			}
		}
	}

	if len(failures) > 0 {
		sort.Strings(failures)
		t.Fatalf("locale key parity failures (%d):\n%s", len(failures), strings.Join(failures, "\n"))
	}
}
