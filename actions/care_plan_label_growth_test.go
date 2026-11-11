package actions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// Bugs.md second batch #1 (2026-10-27): every care table — medication,
// care, feeding, cleanup, weighing, observation — must share ONE row
// presentation: `<ℹ> <description taking ALL available space> | (right
// aligned) <toggle buttons>`. The day-plan/animal-page item rows capped
// their label at `max-width: 34ch` (R9 shrink-to-fit), so cleanup/feeding
// descriptions stopped at ~a third of the row while the medication
// series (which keeps the growing base `.plan-med-label`) used the full
// width — the exact "medication is bigger, the others take less than
// 50 %" report. These pins forbid the shrink-to-fit cap: the label grows,
// wraps when the buttons need the room, and the right-aligned button
// group keeps the §5 ladder behaviour.
func TestCareLabelTakesAllAvailableSpace(t *testing.T) {
	css := readTemplate(t, "../assets/css/care-plan.scss")

	for _, sel := range []string{
		".plan-item-line .plan-med-label",
		"div.plan-med-row .plan-med-label",
	} {
		block := cssBlock(t, css, sel)
		require.NotContains(t, block, "max-width: 34ch",
			sel+" must not cap the description width — it must take all available space")
		require.Contains(t, block, "flex: 1 1 auto",
			sel+" must grow (same as the medication series label)")
	}
}

// cssBlock returns the `{ ... }` body that follows selector in css.
// Matching is exact-prefix (the selector line starts with selector), so
// `.plan-med-label` does not accidentally match `.plan-item-line
// .plan-med-label`.
func cssBlock(t *testing.T, css, selector string) string {
	t.Helper()
	needle := selector + " {"
	i := strings.Index(css, needle)
	require.GreaterOrEqual(t, i, 0, "missing CSS rule for "+selector)
	open := strings.Index(css[i:], "{")
	close := strings.Index(css[i:], "}")
	require.Greater(t, close, open, "malformed CSS block for "+selector)
	return css[i+open : i+close]
}
