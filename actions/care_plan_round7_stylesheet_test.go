package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// R4-7.8 (bugs.md, feedback item 10): "buttons on a table should be aligned
// to offer a clear and clean view". The medication slot buttons live in a
// wrapping flex group; without a fixed cell width they wrap raggedly and the
// times stop reading as a column. These assertions pin the alignment rules
// on the committed stylesheet source (public/ is gitignored and rebuilt by
// webpack, so the .scss is the artefact of record).
func TestCarePlanStylesheetAlignment(t *testing.T) {
	raw := readTemplate(t, "../assets/css/care-plan.scss")

	// .plan-med-cell: fixed width + right-align => every button's left edge
	// lands on the same x.
	require.Contains(t, raw, "min-width: 4.75rem;", "slot cells get a fixed width")
	require.Contains(t, raw, ".plan-med-cell {", "the slot cell rule exists")

	cell := ruleBody(t, raw, ".plan-med-cell {")
	require.Contains(t, cell, "justify-content: flex-end;", "slot cells right-align their content")

	// .plan-med-bucket: reserved height so buckets are aligned bands.
	bucket := ruleBody(t, raw, ".plan-med-bucket {")
	require.Contains(t, bucket, "min-height: 1.1rem;", "bucket captions reserve a height")

	// R4-7.4: the late row's kind pill must not touch the red left border.
	late := ruleBody(t, raw, ".plan-item-late {")
	require.Contains(t, late, "padding-left:", "the late row keeps inner padding")

	// The apply/count affordances the earlier rounds introduced must survive.
	for _, sel := range []string{
		".plan-apply-space",
		".plan-chip-due",
		".plan-item-view",
	} {
		require.Contains(t, raw, sel, sel+" is still styled")
	}
}

// ruleBody returns the declarations inside the first CSS rule whose selector
// header matches `header`, up to its closing brace.
func ruleBody(t *testing.T, css, header string) string {
	t.Helper()
	i := requireIndex(t, css, header)
	open := i + len(header)
	closeIdx := requireIndex(t, css[open:], "}")
	return css[open : open+closeIdx]
}

func requireIndex(t *testing.T, s, sub string) int {
	t.Helper()
	i := indexOf(s, sub)
	require.GreaterOrEqual(t, i, 0, "not found: "+sub)
	return i
}
