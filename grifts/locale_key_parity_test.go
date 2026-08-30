package grifts

import (
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"
	"testing"

	"creaves/locales"
	"creaves/templates"

	"gopkg.in/yaml.v2"
)

// ---------------------------------------------------------------------------
// Locale key parity (Phase 4 of I18N_UI_LOCALIZATION_FIX_PLAN.md)
// ---------------------------------------------------------------------------

// uiLocales is the set of UI locale files every base must provide. "fr" uses
// its own file even though the base .plush.html templates are French, because
// mail/log/UI copy in locales/*.yaml is en-US-authored: fr carries real
// translations, not empty fallbacks.
var uiLocales = []string{"de", "en-us", "fr", "nl"}

// i18nEntry mirrors the go-i18n v1 YAML flat-list format used in locales/.
type i18nEntry struct {
	ID          string `yaml:"id"`
	Translation string `yaml:"translation"`
}

// TestLocaleKeyParity proves the key set (go-i18n message ids) is identical
// across locales/{de,en-US,fr,nl} for every base yaml, with no duplicate ids
// and no empty translations. DB-less: reads the embedded locales FS.
func TestLocaleKeyParity(t *testing.T) {
	entries, err := fs.ReadDir(locales.FS(), ".")
	if err != nil {
		t.Fatalf("read locales fs: %v", err)
	}

	// base -> locale -> parsed entries
	bases := map[string]map[string][]i18nEntry{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".yaml") {
			continue
		}
		stem := strings.TrimSuffix(name, ".yaml")
		dot := strings.LastIndex(stem, ".")
		if dot < 0 {
			t.Errorf("%s: expected <base>.<locale>.yaml", name)
			continue
		}
		base, locale := stem[:dot], stem[dot+1:]

		data, err := fs.ReadFile(locales.FS(), name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		var list []i18nEntry
		if err := yaml.Unmarshal(data, &list); err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		if bases[base] == nil {
			bases[base] = map[string][]i18nEntry{}
		}
		bases[base][locale] = list
	}

	if len(bases) == 0 {
		t.Fatal("no locale yaml bases found")
	}

	seen := map[string]bool{}
	for _, base := range sortedKeys(bases) {
		byLocale := bases[base]

		// 1. every base ships every UI locale
		for _, loc := range uiLocales {
			if _, ok := byLocale[loc]; !ok {
				t.Errorf("%s.%s.yaml: file missing", base, loc)
			}
		}
		for loc := range byLocale {
			if !contains(uiLocales, loc) {
				t.Errorf("%s.%s.yaml: unexpected locale file", base, loc)
			}
		}

		// 2. reference key set = en-US (authoring locale)
		refList, ok := byLocale["en-us"]
		if !ok {
			continue
		}
		refKeys := map[string]bool{}
		for _, e := range refList {
			if e.ID == "" {
				t.Errorf("%s.en-us.yaml: entry with empty id", base)
				continue
			}
			if refKeys[e.ID] {
				t.Errorf("%s.en-us.yaml: duplicate id %q", base, e.ID)
			}
			if strings.TrimSpace(e.Translation) == "" {
				t.Errorf("%s.en-us.yaml: empty translation for %q", base, e.ID)
			}
			refKeys[e.ID] = true
		}

		// 3. every other locale: no dupes, no empties, exact key parity
		for _, loc := range uiLocales {
			if loc == "en-us" {
				continue
			}
			list, ok := byLocale[loc]
			if !ok {
				continue
			}
			seenKeys := map[string]bool{}
			for _, e := range list {
				if e.ID == "" {
					t.Errorf("%s.%s.yaml: entry with empty id", base, loc)
					continue
				}
				if seenKeys[e.ID] {
					t.Errorf("%s.%s.yaml: duplicate id %q", base, loc, e.ID)
				}
				if strings.TrimSpace(e.Translation) == "" {
					t.Errorf("%s.%s.yaml: empty translation for %q", base, loc, e.ID)
				}
				seenKeys[e.ID] = true
				if !refKeys[e.ID] {
					t.Errorf("%s.%s.yaml: extra key not in en-us: %q", base, loc, e.ID)
				}
			}
			for id := range refKeys {
				if !seenKeys[id] {
					t.Errorf("%s.%s.yaml: missing key %q", base, loc, id)
				}
			}
		}

		seen[base] = true
	}
	_ = seen
}

func sortedKeys(m map[string]map[string][]i18nEntry) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Template variant structural parity (Phase 4 of I18N_UI_LOCALIZATION_FIX_PLAN.md)
// ---------------------------------------------------------------------------

// templateLocales is the suffix set of translated template variants.
var templateLocales = []string{"fr", "de", "nl"}

// normalizeTemplate reduces a plush template to its structural skeleton:
//
//   - HTML comments removed
//   - plush expressions <% ... %> / <%= ... %> kept, but with string literals
//     and whitespace collapsed (translated literals like
//     {"data-confirm": "Sind Sie sicher?"} must not count as drift)
//   - remaining double/single-quoted string literals (JS/HTML) collapsed
//   - text nodes between tags removed — including text between two plush
//     block tags (<% if ... { %> Copy <% } %>); plush expressions are made
//     opaque to the eraser (inner angle brackets become sentinel bytes)
//   - all whitespace collapsed
//
// Anything left — element names, nesting, field names, plush call shapes,
// attribute names — must be byte-identical between a base template and its
// locale variants. Text-only diffs are intentional and erased here.
func normalizeTemplate(t *testing.T, src string) string {
	s := src

	// 1. HTML comments
	for {
		i := strings.Index(s, "<!--")
		if i < 0 {
			break
		}
		j := strings.Index(s[i:], "-->")
		if j < 0 {
			t.Fatalf("unterminated HTML comment")
		}
		s = s[:i] + s[i+j+3:]
	}

	// 2. plush expressions: strip quoted literals, collapse whitespace,
	//    neutralize inner angle brackets
	var b strings.Builder
	b.Grow(len(s))
	for {
		i := strings.Index(s, "<%")
		if i < 0 {
			b.WriteString(normalizeQuotes(s))
			break
		}
		b.WriteString(normalizeQuotes(s[:i]))
		j := strings.Index(s[i:], "%>")
		if j < 0 {
			t.Fatalf("unterminated plush expression near %q", clip(s[i:], 60))
		}
		expr := normalizeQuotes(s[i : i+j+2])
		expr = strings.ReplaceAll(expr, "<", "\x01")
		expr = strings.ReplaceAll(expr, ">", "\x02")
		b.WriteString(collapseWS(expr))
		s = s[i+j+2:]
	}
	s = b.String()

	// 3. erase text nodes: delete everything between a tag end (>) or plush
	//    expression boundary and the next tag start, until fixpoint
	for {
		next := tagGapRe.ReplaceAllString(s, "$1$2")
		if next == s {
			break
		}
		s = next
	}

	return collapseWS(s)
}

// tagGapRe removes text between two structural boundaries: tag end (>) or
// a plush expression's neutralized close, and the next tag start (<) or
// expression open. Sentinels keep expressions opaque to the eraser.
var tagGapRe = regexp.MustCompile(`(\x02|>)[^<\x01]*(\x01|<)`)

// normalizeQuotes removes "..." and '...' literals so translated copy inside
// attributes/JS cannot mask real structural drift (and vice versa).
func normalizeQuotes(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			end := strings.IndexByte(s[i+1:], '"')
			if end < 0 {
				b.WriteByte(s[i])
				continue
			}
			b.WriteString(`"·"`)
			i += end + 1
		case '\'':
			end := strings.IndexByte(s[i+1:], '\'')
			if end < 0 {
				b.WriteByte(s[i])
				continue
			}
			b.WriteString(`'·'`)
			i += end + 1
		default:
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

func collapseWS(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	space := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case ' ', '\t', '\n', '\r':
			space = true
		default:
			if space {
				b.WriteByte(' ')
				space = false
			}
			b.WriteByte(c)
		}
	}
	if space {
		b.WriteByte(' ')
	}
	return b.String()
}

func clip(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// knownVariantDrift records base templates whose existing locale variants
// predate this check and still carry structural drift (documented as debt in
// I18N_UI_LOCALIZATION_FIX_PLAN.md §3 Phase 4). Drift on these is logged,
// not failed; drift on ANY other template fails the test (ratchet: existing
// debt frozen, new drift blocked). Entries disappear as debt is paid down.
var knownVariantDrift = map[string]bool{
	"animalages/_form.plush.html":                  true,
	"animals/_form.plush.html":                     true,
	"animals/edit.plush.html":                      true,
	"animals/show.plush.html":                      true,
	"animaltypes/_form.plush.html":                 true,
	"animaltypes/index.plush.html":                 true,
	"animaltypes/show.plush.html":                  true,
	"application.plush.html":                       true,
	"auth/new.plush.html":                          true,
	"cares/_form.plush.html":                       true,
	"cares/index.plush.html":                       true,
	"cares/new.plush.html":                         true,
	"caretypes/_form.plush.html":                   true,
	"dashboard/dashboard.plush.html":               true,
	"drugs/_form.plush.html":                       true,
	"drugs/show.plush.html":                        true,
	"entry_causes/_form.plush.html":                true,
	"entry_causes/index.plush.html":                true,
	"entry_causes/show.plush.html":                 true,
	"feeding/index.plush.html":                     true,
	"landing/index.plush.html":                     true,
	"localities/_form.plush.html":                  true,
	"logentries/_form.plush.html":                  true,
	"maintenance/index.plush.html":                 true,
	"native_statuses/_form.plush.html":             true,
	"native_statuses/index.plush.html":             true,
	"native_statuses/show.plush.html":              true,
	"outtakes/_form.plush.html":                    true,
	"outtakes/edit.plush.html":                     true,
	"outtakes/new.plush.html":                      true,
	"outtakes/show.plush.html":                     true,
	"outtaketypes/_form.plush.html":                true,
	"outtaketypes/show.plush.html":                 true,
	"reception/new.plush.html":                     true,
	"registersnapshot/registersnapshot.plush.html": true,
	"registertable/registertable.plush.html":       true,
	"species/_form.plush.html":                     true,
	"species/index.plush.html":                     true,
	"species/show.plush.html":                      true,
	"subside_groups/_form.plush.html":              true,
	"subside_groups/index.plush.html":              true,
	"subside_groups/show.plush.html":               true,
	"travels/_form.plush.html":                     true,
	"travels/new.plush.html":                       true,
	"traveltypes/_form.plush.html":                 true,
	"traveltypes/index.plush.html":                 true,
	"treatments/_form.plush.html":                  true,
	"treatments/new.plush.html":                    true,
	"treatments/show.plush.html":                   true,
	"users/_aform.plush.html":                      true,
	"users/_form.plush.html":                       true,
	"users/new.plush.html":                         true,
	"veterinaryvisits/_form.plush.html":            true,
	"veterinaryvisits/new.plush.html":              true,
	"zones/_form.plush.html":                       true,
	"zones/show.plush.html":                        true,
}

// TestTemplateVariantStructuralParity proves each .plush.fr/.de/.nl.html
// variant is a pure text translation of its base .plush.html: identical tag
// structure, field names, and plush call shapes after normalized-copy
// erasure. Also proves the variant inventory is consistent (documented in
// I18N_UI_LOCALIZATION_FIX_PLAN.md §3 Phase 4). DB-less: embedded templates FS.
func TestTemplateVariantStructuralParity(t *testing.T) {
	var bases []string
	err := fs.WalkDir(templates.FS(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".plush.html") {
			return nil
		}
		base := path.Dir(p) + "/" + strings.TrimSuffix(path.Base(p), ".plush.html")
		// skip locale variants themselves
		for _, loc := range templateLocales {
			if strings.HasSuffix(base, "."+loc) {
				return nil
			}
		}
		bases = append(bases, p)
		return nil
	})
	if err != nil {
		t.Fatalf("walk templates: %v", err)
	}
	if len(bases) == 0 {
		t.Fatal("no base templates found")
	}
	sort.Strings(bases)

	variantsFound := 0
	known := 0
	for _, p := range bases {
		src, err := fs.ReadFile(templates.FS(), p)
		if err != nil {
			t.Fatalf("read %s: %v", p, err)
		}
		want := normalizeTemplate(t, string(src))

		baseNoExt := strings.TrimSuffix(p, ".plush.html")
		for _, loc := range templateLocales {
			vp := baseNoExt + ".plush." + loc + ".html"
			vsrc, err := fs.ReadFile(templates.FS(), vp)
			if err != nil {
				// variant legitimately absent (e.g. config/*) — inventory note only
				t.Logf("variant absent: %s", vp)
				continue
			}
			variantsFound++
			got := normalizeTemplate(t, string(vsrc))
			if got != want {
				i := 0
				for i < len(got) && i < len(want) && got[i] == want[i] {
					i++
				}
				start := i - 40
				if start < 0 {
					start = 0
				}
				detail := fmt.Sprintf("%s vs %s: structural drift at offset %d\nbase : %s\nvar  : %s",
					p, vp, i, clip(want[start:], 120), clip(got[start:], 120))
				if knownVariantDrift[p] {
					known++
					t.Logf("KNOWN debt: %s", detail)
				} else {
					t.Errorf("NEW drift — fix the template pair or, if text-only, extend the eraser: %s", detail)
				}
			}
		}
	}
	t.Logf("checked %d base templates, %d variants, %d pairs with known (frozen) drift", len(bases), variantsFound, known)
}
