package actions

import (
	"regexp"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// R4-7.13 (feedback item 13): "Menu size should be reviewed — hamburger is
// shown when there is *still* important space available". The bar collapsed
// to the hamburger below lg (992px) although the icon-only bar fits far
// below that, so the breakpoint moved down to md (768px).
//
// The threshold appears in THREE places that must agree, or the bar lies:
// the Bootstrap expand class, the CSS query that keeps labels visible while
// the menu is collapsed, and the JS guard that only probes overflow on an
// expanded bar. These assertions pin all three together.
func TestNavbarBreakpointsAgree(t *testing.T) {
	// Bootstrap's md breakpoint.
	const mdBreakpoint = 768

	forks := []string{
		"../templates/application.plush.html",
		"../templates/application.plush.fr.html",
		"../templates/application.plush.de.html",
		"../templates/application.plush.nl.html",
	}
	expandRE := regexp.MustCompile(`navbar-expand-(sm|md|lg|xl)`)
	maxRE := regexp.MustCompile(`@media \(max-width: ([\d.]+)px\) \{ #navbarSupportedContent`)
	minRE := regexp.MustCompile(`@media \(min-width: ([\d.]+)px\) \{ nav\.navbar\.nav-compact`)
	jsRE := regexp.MustCompile(`if \(w < (\d+)\) \{ navbar\.classList\.remove\('nav-compact'\)`)

	for _, f := range forks {
		raw := readTemplate(t, f)

		m := expandRE.FindStringSubmatch(raw)
		require.NotNil(t, m, f+" — no navbar-expand-* class")
		require.Equal(t, "md", m[1], f+" — the bar must expand down to md (768px)")

		mm := maxRE.FindStringSubmatch(raw)
		require.NotNil(t, mm, f+" — no collapsed-menu label query")
		require.Equal(t, float64(mdBreakpoint-1)+0.98, mustFloat(t, mm[1]),
			f+" — labels must stay visible below the expand breakpoint only")

		mn := minRE.FindStringSubmatch(raw)
		require.NotNil(t, mn, f+" — no compact-bar query")
		require.Equal(t, float64(mdBreakpoint), mustFloat(t, mn[1]),
			f+" — the compact (icon-only) state only exists on an expanded bar")

		j := jsRE.FindStringSubmatch(raw)
		require.NotNil(t, j, f+" — no overflow-probe guard")
		require.Equal(t, strconv.Itoa(mdBreakpoint), j[1],
			f+" — the JS probe must use the same breakpoint as the CSS")
	}
}

func mustFloat(t *testing.T, s string) float64 {
	t.Helper()
	v, err := strconv.ParseFloat(s, 64)
	require.NoError(t, err, s)
	return v
}
