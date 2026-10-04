//go:build !sqlite
// +build !sqlite

package actions

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

// R4-7.15 — `/care_plan?view=compact&kind=medication`: "format the animal
// column to make sure all animal have the same lenght + same light red
// background color".
//
// Measured before the fix: 36 animal cells, 13 distinct widths (168..181px
// plus one 239px outlier) and `bg=rgba(0,0,0,0)` on every single one — the
// labels did not read as a column at all.
//
// The tension the fix has to satisfy: "the same length" wants a hard width,
// but R4-7.24 ("never cut a description on any page") forbids cutting the
// longest label. A fixed px constant satisfies one and breaks the other
// depending on the data, so the width is COMPUTED per page from the longest
// label.

// TestMedAnimalColumnIsSizedFromTheLongestLabel: the column must fit the
// longest label on the page — that is the only way every cell can be equal
// AND nothing truncated.
func TestMedAnimalColumnIsSizedFromTheLongestLabel(t *testing.T) {
	longest := "1903/26 · Tourterelle turque · S11" // 34 runes: the measured worst case
	tiers := [3][]MedTierLine{
		{medLine("1903/26 · Tourterelle turque · S11")},
		{medLine("1912/26 · Hérisson · VI39")},
		{medLine("1444/26 · Hérisson · R3")},
	}
	col := medAnimalColCh(tiers)
	require.Greater(t, col, utf8.RuneCountInString(longest),
		"the column must be at least as wide as the longest label, or that label is cut")
	// +1 spare column: `ch` is the width of "0", so a label of narrow glyphs
	// renders wider than its rune count.
	require.Equal(t, utf8.RuneCountInString(longest)+1, col)
}

// TestMedAnimalColumnCountsRunesNotBytes: a French species name is multi-byte.
// Counting bytes would size the column ~3× too wide (a huge empty gutter on
// every row) — the same defect class as R4-7.20's byte slice.
func TestMedAnimalColumnCountsRunesNotBytes(t *testing.T) {
	label := "1903/26 · Hérisson · R3"
	tiers := [3][]MedTierLine{{medLine(label)}}
	col := medAnimalColCh(tiers)
	require.Equal(t, utf8.RuneCountInString(label)+1, col)
	require.Less(t, col, len(label)+1,
		"the width must follow the rune count, not the byte count")
}

// TestMedAnimalColumnIsZeroWithoutMedication: a feeding/observation page has
// no medication rows; the template must not emit a `0ch` width. (0ch would
// collapse the cell if the class were ever reused.)
func TestMedAnimalColumnIsZeroWithoutMedication(t *testing.T) {
	require.Equal(t, 0, medAnimalColCh([3][]MedTierLine{}))
}

// TestMedAnimalCellIsTintedInEveryFork: the second half of the request — the
// SAME light red on every cell. The colour lives in the stylesheet, so the
// template's job is to carry the class; pin the class, not the hex.
func TestMedAnimalCellIsTintedInEveryFork(t *testing.T) {
	forks := []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/index.plush.fr.html",
		"../templates/care_plan/index.plush.de.html",
		"../templates/care_plan/index.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.Equal(t, 3, strings.Count(raw, `class="mr-2 text-nowrap py-1 plan-med-animal"`), f+
			": every medication tier body (late/now/later) carries the animal cell")
		// R9: the ONE page-wide width is gone — the user asked for the
		// animal cell to size to its content so the whole line reads on a
		// single line (<info> <animal> | <drug> | <toggles>) without the
		// long empty tail the widest-label width produced.
		require.NotContains(t, raw, `min-width: <%= view.MedAnimalColCh %>ch`, f+
			": the page-wide animal column must not come back (R9 single-line layout)")
	}
}

// TestMedAnimalColumnStyleNeverCuts: the width is a min-width on purpose. A
// `width` plus an overflow rule would hide a long label, which is exactly the
// critical defect R4-7.24 was raised for.
func TestMedAnimalColumnStyleNeverCuts(t *testing.T) {
	raw := readTemplate(t, "../templates/care_plan/index.plush.html")
	require.NotContains(t, raw, `plan-med-animal" style="width:`,
		"a hard width would clip the longest label")
	require.NotContains(t, raw, `text-overflow:ellipsis`,
		"the animal column must never ellipsise a label")

	scss := readTemplate(t, "../assets/css/care-plan.scss")
	require.Contains(t, scss, ".plan-med-animal", "the column needs its own rule")
	require.Contains(t, scss, "flex: 0 0 auto", "it must not stretch with the series")
}

// medLine is a minimal MedGroupView carrying only what the width reads.
func medLine(label string) MedTierLine {
	return MedTierLine{AnimalLabel: label}
}
