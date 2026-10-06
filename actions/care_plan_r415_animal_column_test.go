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
//
// Phase 0b: the FOUR hand-copied medication tier bodies (late/now/later +
// Done) collapsed into the ONE `_plan_tier_med_rows` partial, whose row
// shell then became the ONE `_plan_med_row` component — the animal cell
// exists exactly once and every tier renders through it, so "every
// tier carries the cell" now holds by construction.
func TestMedAnimalCellIsTintedInEveryFork(t *testing.T) {
	forks := []string{
		"../templates/care_plan/_plan_med_row.plush.html",
		"../templates/care_plan/_plan_med_row.plush.fr.html",
		"../templates/care_plan/_plan_med_row.plush.de.html",
		"../templates/care_plan/_plan_med_row.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		// B10-2: the cell no longer forces text-nowrap — the loc line
		// (Cage · Zone · Espèce) wraps beneath the year like the sibling
		// tables' animal cells.
		require.Equal(t, 1, strings.Count(raw, `class="mr-2 py-1 plan-med-animal"`), f+
			": the ONE shared medication tier body carries the animal cell")
		// Bug 2026-10-27 #3a: the ONE page-wide width is WIRED AGAIN — R9's
		// removal let every cell size its own content and the labels stopped
		// reading as a column (the reported defect). The width is a MIN-width
		// (never cuts a long label, R4-7.24) computed once from the widest
		// label on the page; the loc line still wraps beneath the year.
		require.Contains(t, raw, `style="min-width: <%= view.MedAnimalColCh %>ch"`, f+
			": the shared animal-column width must be wired onto the cell")
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
