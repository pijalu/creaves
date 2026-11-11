package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestHumanizeMatcher (bugs.md U18): a stored matcher DSL expression renders
// as a readable sentence with localized field labels and operators in every
// supported locale — never raw careplan.field.* keys or DSL identifiers.
func TestHumanizeMatcher(t *testing.T) {
	expr := `force_feed = true AND weight_g < 300`
	cases := []struct{ lang, want string }{
		{"en-US", "Force feed: yes and Weight (g) < 300"},
		{"fr", "Gavage: oui et Poids (g) < 300"},
		{"de", "Zwangsfütterung: ja und Gewicht (g) < 300"},
		{"nl", "Gedwongen voeden: ja en Gewicht (g) < 300"},
	}
	for _, c := range cases {
		got := humanizeMatcherWith(expr, humanizerTestTranslate(t, c.lang))
		require.Equal(t, c.want, got, "lang %s", c.lang)
	}
}

// TestHumanizeMatcherExact pins the full rendering for one locale and the
// structural variants (OR, NOT, BETWEEN, IN, string quoting).
func TestHumanizeMatcherExact(t *testing.T) {
	tr := humanizerTestTranslate(t, "en-US")

	require.Equal(t,
		"Force feed: yes",
		humanizeMatcherWith(`force_feed = true`, tr))
	require.Equal(t,
		"Force feed: no",
		humanizeMatcherWith(`force_feed = false`, tr))
	require.Equal(t,
		"Force feed: yes and Weight (g) < 300",
		humanizeMatcherWith(`force_feed = true AND weight_g < 300`, tr))
	require.Equal(t,
		"Species is “Hedgehog” or Cage is “E12”",
		humanizeMatcherWith(`species = "Hedgehog" OR cage = "E12"`, tr))
	require.Equal(t,
		"not Has wounds: yes",
		humanizeMatcherWith(`NOT has_wounds = true`, tr))
	require.Equal(t,
		"Weight (g) is between 100 – 250",
		humanizeMatcherWith(`weight_g BETWEEN 100 AND 250`, tr))
	require.Equal(t,
		"Zone is one of “E”, “F”",
		humanizeMatcherWith(`zone IN ("E", "F")`, tr))
	// Explicit CI operators (bugs.md U26/U27 R5-1a) read as their CS label
	// with an "any case" qualifier.
	require.Equal(t,
		"Cage is (any case) “Enclos renards”",
		humanizeMatcherWith(`cage =* "Enclos renards"`, tr))
	require.Equal(t,
		"Cage is not (any case) “VE5”",
		humanizeMatcherWith(`cage !=* "VE5"`, tr))
	require.Equal(t,
		"Zone is one of (any case) “E”, “F”",
		humanizeMatcherWith(`zone INCI ("E", "F")`, tr))
	// Nested groups keep parentheses so precedence stays visible.
	require.Equal(t,
		"(Force feed: yes and Weight (g) < 300) or Species is “Hedgehog”",
		humanizeMatcherWith(`(force_feed = true AND weight_g < 300) OR species = "Hedgehog"`, tr))
	// Unknown fields fall back to the raw key (registry miss).
	require.Equal(t,
		"some_future_field is “x”",
		humanizeMatcherWith(`some_future_field = "x"`, tr))
	// Unparseable → verbatim fallback (display-only path).
	require.Equal(t, `weight_g <`, humanizeMatcherWith(`weight_g <`, tr))
}

// TestHumanizeMatcherAllLocales: every shipped locale resolves every field
// label and connective (missing keys fail loudly via humanizerTestTranslate).
func TestHumanizeMatcherAllLocales(t *testing.T) {
	expr := `force_feed = true AND weight_g BETWEEN 100 AND 250 OR zone INCI ("E", "F")`
	for _, lang := range []string{"en-US", "fr", "de", "nl"} {
		got := humanizeMatcherWith(expr, humanizerTestTranslate(t, lang))
		require.NotContains(t, got, "careplan.field.", "lang %s leaks i18n key: %q", lang, got)
		require.NotContains(t, got, "force_feed", "lang %s leaks DSL key: %q", lang, got)
		require.NotContains(t, got, "weight_g", "lang %s leaks DSL key: %q", lang, got)
		require.NotContains(t, got, "BETWEEN", "lang %s leaks DSL keyword: %q", lang, got)
		require.NotContains(t, got, "INCI", "lang %s leaks DSL CI keyword: %q", lang, got)
	}
}
