package actions

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// R4-7.24: "never cut a description on any page — showing a cut description
// is a critical issue".
//
// The care-plan converter shortened a diet string with a BYTE slice
// (`title[:60]`). A byte slice is wrong twice over:
//
//   - It splits multi-byte runes. "é" is 2 bytes, "…" 3; a diet crossing
//     offset 60 mid-rune stores an invalid sequence, which MySQL rejects or
//     renders as U+FFFD — exactly the garbled text R4-7.24 is about.
//   - It splits words, so the caregiver reads "… pour animaux sick" and has
//     to guess at the cut.
//
// The target columns are varchar(200), which MySQL counts in CHARACTERS, so
// the budget must be in characters too — a byte budget silently under-fills
// the column and, worse, measures 3× short on accented text.

// ellipsis is the single cut marker. One rune, not "...", so the budget stays
// honest and the display stays one glyph wide.
const ellipsis = "…"

// truncateWords shortens s to at most max RUNES, cutting at the last word
// boundary that fits and marking the cut with "…".
//
// Rules, in order:
//   - text already within budget is returned untouched (no ellipsis);
//   - the cut lands on a word boundary (after a space), never mid-word;
//   - with no boundary to fall back to (one giant word), the text is still cut
//     — returning it over-long would overflow the varchar(200) column, which
//     is the very bug being fixed;
//   - the result is always valid UTF-8, whatever the input was.
func truncateWords(s string, max int) string {
	if max <= 0 {
		return ""
	}
	// Fast path: fits as-is. Counted in runes, matching the column's rule.
	if utf8.RuneCountInString(s) <= max {
		return s
	}

	runes := []rune(s)
	// Walk back from the budget to the last space, so the cut is a word edge.
	cut := max
	for cut > 0 && runes[cut-1] != ' ' {
		cut--
	}
	if cut == 0 {
		// One unbreakable word: fall back to a hard cut. Rune slicing keeps
		// the result valid UTF-8 where a byte slice would not.
		cut = max
	}

	body := strings.TrimRight(string(runes[:cut]), " ")
	return body + ellipsis
}

// convertedFeedingName is the converted feeding plan/rule name, with the diet
// shortened to fit the varchar(200) column alongside its wrappers.
func convertedFeedingName(diet string, fallback bool) string {
	name := fmt.Sprintf("Alimentation — %s (conversion)", truncateWords(diet, convertedDietBudget))
	if fallback {
		name += " (à vérifier)"
	}
	return name
}

// convertedMatcherName is the converted cluster-matcher name for a diet.
func convertedMatcherName(diet string) string {
	return fmt.Sprintf("Régime « %s » (conversion)", truncateWords(diet, convertedDietBudget))
}

// RebuildConvertedFeedingName re-derives the conversion name of a feeding
// plan/rule from its FULL diet text. Exported for the careplan:fixnames
// maintenance grift, which repairs rows written by the earlier byte-based
// truncation (names cut mid-word, e.g. "…1 dose d’e (conversion)").
func RebuildConvertedFeedingName(diet string, fallback bool) string {
	return convertedFeedingName(diet, fallback)
}

// RebuildConvertedMatcherName is the matcher-name counterpart of
// RebuildConvertedFeedingName.
func RebuildConvertedMatcherName(diet string) string {
	return convertedMatcherName(diet)
}

// convertedDietBudget is how many characters of diet text survive inside the
// conversion name. The wrappers ("Alimentation — " + " (conversion)" and
// "Régime « " + " » (conversion)") are well under 40 characters, so 60 leaves
// the composed name far inside varchar(200) while showing a whole word.
const convertedDietBudget = 60
