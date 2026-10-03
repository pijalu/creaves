package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// R4-3.1/R4-3.2 — the ℹ is the FIRST entry of every medication series line and
// must land on the same x whatever the label length.
//
// Measured in the live DOM (dev, admin, 1108 px tier body): the series is
// `flex-grow-1` inside a row that ALSO holds the ANIMAL cell and that row
// WRAPS (`d-flex … flex-wrap`). With `flex-basis: auto` the series' hypothetical
// size is its MAX-CONTENT width (label + every button side by side); when that
// exceeded the space left by the animal column the WHOLE series jumped to a
// line of its own — where it fitted comfortably, so the misalignment looked
// innocent — and the ℹ sat 361 px left of the other rows (33 rows at x=455,
// 3 at x=94). The three offenders were the widest series.
//
// The fix is CSS-only and lives in the artefact of record (public/ is
// gitignored and rebuilt by webpack): a zero flex-basis declares "this item
// shares the row", so it shrinks to the remaining width and its own line folds
// per the A/B/C rule (B: buttons wrap, C: label wraps).
func TestMedSeriesStaysOnTheAnimalCellsLine(t *testing.T) {
	raw := readTemplate(t, "../assets/css/care-plan.scss")

	series := ruleBody(t, raw, ".med-series {")
	require.Contains(t, series, "min-width: 0;",
		"the automatic (min-content) minimum must be dropped, or the series keeps its full width")
	require.Contains(t, series, "flex: 1 1 0;",
		"a zero flex-basis is what keeps the series ON the animal cell's line")

	// The A/B/C folding still owns the line's internal layout: the fix must not
	// have taken the wrapping away from the buttons.
	line := ruleBody(t, raw, ".plan-med-line {")
	require.Contains(t, line, "flex-wrap: wrap;", "the line still folds its own content (A/B/C)")
	btns := ruleBody(t, raw, ".plan-med-btns {")
	require.Contains(t, btns, "flex-wrap: wrap;", "the button group still folds (shape B)")
	require.Contains(t, btns, "justify-content: flex-end;", "folded buttons stay right-aligned")

	// Nothing may clip a label to achieve alignment (R4-7.24).
	require.NotContains(t, series, "overflow: hidden", "the series must never hide its content")
	require.NotContains(t, series, "text-overflow", "no ellipsis on the series")
}