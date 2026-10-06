package actions

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// R4-7.24: "never cut a description on any page — showing a cut description
// is a critical issue".
//
// The defect was a BYTE slice, `title[:60]`, applied to a diet string. That
// is wrong twice over:
//
//   - It cuts mid-rune. A 2-byte "é" or a 3-byte "…" straddling offset 60 is
//     split, and the stored name carries a replacement character (or an
//     invalid sequence MySQL will refuse).
//   - It cuts mid-word, so the caregiver reads "Alimentation — Lebensmittel
//     für kranke Naget" and has to guess.
//
// The column is varchar(200), which MySQL counts in CHARACTERS, so the cap
// must be in characters too — a byte cap silently under-fills it.

// TestTruncateWordsLeavesShortTextAlone: the common case must be untouched —
// no ellipsis appended to text that already fits.
func TestTruncateWordsLeavesShortTextAlone(t *testing.T) {
	require.Equal(t, "Nourrissage", truncateWords("Nourrissage", 60))
	require.Equal(t, "", truncateWords("", 60))
}

// TestTruncateWordsCutsAtAWordBoundary: never mid-word.
func TestTruncateWordsCutsAtAWordBoundary(t *testing.T) {
	long := "Alimentation speciale pour animaux sick with a very long diet name"
	got := truncateWords(long, 40)
	require.True(t, strings.HasSuffix(got, "…"), "a cut must be marked: %q", got)
	// Everything before the ellipsis must be a prefix of the ORIGINAL text,
	// and must end on a word boundary (last kept char is a space).
	body := strings.TrimSuffix(got, "…")
	require.True(t, strings.HasPrefix(long, body), "kept text %q is not a prefix of the source", body)
	// A word-boundary cut means the source rune right after the kept body is a
	// space — so no fragment is left dangling. The body itself must NOT end in
	// a space (that would render as "word …").
	require.False(t, strings.HasSuffix(body, " "), "space left before the ellipsis: %q", body)
	require.Equal(t, ' ', []rune(long)[len([]rune(body))],
		"the cut did not land on a word boundary in %q", body)
}

// TestTruncateWordsNeverSplitsARune: the core R4-7.24 guarantee. A byte slice
// would produce invalid UTF-8 here.
func TestTruncateWordsNeverSplitsARune(t *testing.T) {
	// Every rune is 3 bytes, so a byte-slice at 60 would land mid-rune.
	src := strings.Repeat("é", 80) + " suite"
	got := truncateWords(src, 20)
	require.True(t, utf8.ValidString(got), "truncation produced invalid UTF-8: %q", got)
	require.Equal(t, 20, utf8.RuneCountInString(strings.TrimSuffix(got, "…")),
		"must keep exactly max runes")
}

// TestTruncateWordsCountsRunesNotBytes: a 60-rune budget must hold 60 runes
// even when each is 3 bytes.
func TestTruncateWordsCountsRunesNotBytes(t *testing.T) {
	src := strings.Repeat("é", 60)
	require.Equal(t, src, truncateWords(src, 60),
		"exactly 60 runes fits the 60-rune budget untouched")
}

// TestTruncateWordsWithASingleGiantWord: when there is no space to fall back
// to, the text must still be CUT — returning an over-long string would break
// the varchar(200) column, which is the original bug.
func TestTruncateWordsWithASingleGiantWord(t *testing.T) {
	got := truncateWords(strings.Repeat("x", 100), 60)
	require.True(t, utf8.ValidString(got))
	require.LessOrEqual(t, utf8.RuneCountInString(strings.TrimSuffix(got, "…")), 60,
		"an unbreakable word must still be cut, or the name overflows the column")
	require.True(t, strings.HasSuffix(got, "…"))
}

// TestTruncateWordsWithTrailingSpace: a cut right at a space must not leave a
// trailing one before the ellipsis.
func TestTruncateWordsWithTrailingSpace(t *testing.T) {
	got := truncateWords("abc def ghi", 8) // budget lands just after "abc "
	require.NotContains(t, got, " …", "no space immediately before the ellipsis: %q", got)
}

// TestTruncateWordsKeepsExistingEllipsisIntact: a name already carrying "…"
// must not be double-marked.
func TestTruncateWordsKeepsExistingEllipsisIntact(t *testing.T) {
	got := truncateWords("Already cut…", 60)
	require.True(t, strings.HasSuffix(got, "…"))
	require.NotContains(t, got, "……", "an existing ellipsis must not be doubled")
}

// TestConvertedNamesAreNeverCutMidRune: the R4-7.24 regression guard on the
// REAL producer — the converted feeding/rule/matcher names.
func TestConvertedNamesAreNeverCutMidRune(t *testing.T) {
	diet := strings.Repeat("nourriture spéciale ", 6)
	for _, got := range []string{
		convertedFeedingName(diet, false),
		convertedFeedingName(diet, true),
		convertedMatcherName(diet),
	} {
		require.True(t, utf8.ValidString(got), "invalid UTF-8 in %q", got)
		require.LessOrEqual(t, utf8.RuneCountInString(got), 200,
			"name must fit varchar(200): %q", got)
		require.False(t, strings.Contains(got, "�"),
			"replacement character in %q", got)
	}
}

// TestTruncatedNameStillShowsTheCompleteContent: R4-7.24's whole point is
// that the caregiver reads the FULL description. A name cut at a word
// boundary must not become the end of the story — richPlanName detects that
// the (truncated) name no longer contains the content and appends the
// payload-derived full text, so nothing the caregiver needs is ever hidden by
// a cut.
func TestTruncatedNameStillShowsTheCompleteContent(t *testing.T) {
	diet := "nourriture speciale pour animaux sick avec un tres long regime alimentaire"
	cut := convertedFeedingName(diet, false)
	// The cut is marked with "…" inside the wrapper, not at the end of the
	// whole composed name.
	require.Contains(t, cut, "…", "precondition: the name is cut: %q", cut)

	payload := fmt.Sprintf(`{"food":%q}`, diet)
	got := richPlanName(cut, "feeding", []byte(payload))

	require.Contains(t, got, diet,
		"the COMPLETE diet must appear even though the stored name is cut")
	require.NotContains(t, got, "�", "no replacement character: %q", got)
	require.Equal(t, 1, strings.Count(got, diet),
		"the content must be appended once, not duplicated: %q", got)
}

// TestConverterNeverCutsANameWithAByteSlice: pins the CALL SITES, not just the
// helper. The helper's own tests pass no matter what the producer does, so
// without this the original `title[:60]` could come back unnoticed — a byte
// slice is exactly the defect R4-7.24 is about.
func TestConverterNeverCutsANameWithAByteSlice(t *testing.T) {
	src := readTemplate(t, "../actions/care_plan_convert_data.go")
	for _, banned := range []string{
		"title[:60]",
		"title2[:60]",
		"%.60s",
	} {
		require.NotContains(t, src, banned,
			"R4-7.24: a byte slice cuts a name mid-rune/mid-word — use truncateWords")
	}
	// v3: the converter no longer builds cluster matchers at all, so the
	// matcher-naming call site is gone from the converter (the helper itself
	// survives in text_truncate.go for the careplan:fixnames grift).
	require.NotContains(t, src, "convertedMatcherName(diet)",
		"v3: the converter must not build matchers — no matcher naming call site")
	require.Equal(t, 1, strings.Count(src, "convertedFeedingName("),
		"the per-animal plan is the only converted feeding name site (v3: no cluster rules)")
}

// TestConvertedNamesKeepAWholeWord: the visible tail of a converted name must
// not be a fragment of a word.
func TestConvertedNamesKeepAWholeWord(t *testing.T) {
	diet := strings.Repeat("nourriture speciale ", 6)
	name := convertedFeedingName(diet, false)
	// The diet is embedded between the "Alimentation — " and " (conversion)"
	// wrappers; whatever survived must not end on a partial word.
	inner := strings.TrimSuffix(strings.TrimPrefix(name, "Alimentation — "), " (conversion)")
	require.NotContains(t, inner, "�", "no replacement character in %q", name)
	// This diet IS longer than the budget, so a marked cut is expected — what
	// must not happen is a cut mid-word or mid-rune.
	require.True(t, strings.HasSuffix(inner, "…"), "expected a marked cut in %q", inner)
	body := strings.TrimSuffix(inner, "…")
	runes := []rune(strings.Repeat("nourriture speciale ", 6))
	require.Equal(t, ' ', runes[len([]rune(body))],
		"the cut landed mid-word in %q", inner)
}
