//go:build !sqlite

package actions

import (
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// bugs.md R4-7.17: a translation written as "<%= t("...") %>" inside a
// <script> block is HTML-escaped by Plush, so the JS VARIABLE holds entity
// text. Measured on /care_plan in French before the fix: 7 entities inside the
// served scripts, and the observation confirmation read
// "La réponse à l&#39;observation sera enregistrée…". A translation holding a
// double quote, or "</script>", would additionally break out of the string —
// or the whole block — and inject markup.

// TestJSStringNeutralizesTheThreeWaysATranslationCanBreakAScript pins the
// helper itself: the apostrophe must survive as itself (the R4-7.17 defect),
// a double quote must be escaped (it used to end the JS string), and a closing
// script tag must be neutralised.
func TestJSStringNeutralizesTheThreeWaysATranslationCanBreakAScript(t *testing.T) {
	got, err := jsString("La réponse à l'observation sera enregistrée.")
	require.NoError(t, err)
	require.Equal(t, `"La réponse à l'observation sera enregistrée."`, string(got),
		"an apostrophe must stay an apostrophe, not &#39;")

	quoted, err := jsString(`il a dit "oui"`)
	require.NoError(t, err)
	require.Equal(t, `"il a dit \"oui\""`, string(quoted))

	injected, err := jsString(`</script><script>alert(1)</script>`)
	require.NoError(t, err)
	require.NotContains(t, string(injected), "</script>",
		"a translation must not be able to close the script block")
	require.Contains(t, string(injected), `\u003c`)

	// A newline would otherwise break the literal across two lines.
	multi, err := jsString("a\nb")
	require.NoError(t, err)
	require.Equal(t, `"a\nb"`, string(multi))
}

// jsCarryingTemplateFiles lists every template that carries a <script> block
// with a translation in it — the surface R4-7.17 touched.
func jsCarryingTemplateFiles() []string {
	return []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.nl.html",
		"../templates/care_plan/_apply_toggle.plush.html",
		"../templates/care_plan/_apply_toggle.plush.fr.html",
		"../templates/care_plan/_apply_toggle.plush.de.html",
		"../templates/care_plan/_apply_toggle.plush.nl.html",
		"../templates/care_plan/_plan_med_toggle.plush.html",
		"../templates/care_plan/_plan_med_toggle.plush.fr.html",
		"../templates/care_plan/_plan_med_toggle.plush.de.html",
		"../templates/care_plan/_plan_med_toggle.plush.nl.html",
		"../templates/care_rules/edit.plush.html",
		"../templates/care_rules/edit.plush.fr.html",
		"../templates/care_rules/edit.plush.de.html",
		"../templates/care_rules/edit.plush.nl.html",
		"../templates/care_rules/new.plush.html",
		"../templates/care_rules/new.plush.fr.html",
		"../templates/care_rules/new.plush.de.html",
		"../templates/care_rules/new.plush.nl.html",
		"../templates/outtakes/_form.plush.html",
		"../templates/outtakes/_form.plush.fr.html",
		"../templates/outtakes/_form.plush.de.html",
		"../templates/outtakes/_form.plush.nl.html",
	}
}

var (
	scriptBlockRE = regexp.MustCompile(`(?s)<script[^>]*>.*?</script>`)
	// A translation wrapped in quotes inside a script — the R4-7.17 shape.
	bareTranslationRE = regexp.MustCompile("[\"']<%=\\s*t\\([^)]*\\)\\s*%>[\"']")
	// A jsString call wrapped in quotes — the Round-8 shape (R8-1). jsString
	// already emits a quoted literal; an outer quote pair yields
	// `""value"" …` which is a hard JS syntax error and kills the whole
	// script block (measured live: window.planApply undefined, every shared
	// apply control dead on /care_plan and the animal tabs).
	quotedJSStringRE = regexp.MustCompile("[\"']<%=\\s*jsString\\(")
	htmlEntityRE     = regexp.MustCompile(`&#\d+;|&[a-z]+;`)
)

// TestNoTemplateInterpolatesATranslationIntoAScript is the call-site guard.
// TestJSString proves the helper; this proves no template still uses the old
// shape — a helper test alone does not pin its call sites.
func TestNoTemplateInterpolatesATranslationIntoAScript(t *testing.T) {
	for _, f := range jsCarryingTemplateFiles() {
		body, err := os.ReadFile(f)
		require.NoError(t, err, f)
		blocks := scriptBlockRE.FindAllString(string(body), -1)
		require.NotEmpty(t, blocks, "%s must still carry its script block", f)
		for i, block := range blocks {
			require.NotRegexp(t, bareTranslationRE, block,
				"%s script block %d still interpolates a translation; use jsString(t(...))", f, i)
			require.NotRegexp(t, quotedJSStringRE, block,
				"%s script block %d wraps jsString in quotes — it already emits a quoted literal (R8-1)", f, i)
		}
	}
}

// TestAutoRefreshGuardUsesActionAgeDirectly pins the R8-2 fix: the R4-4.1
// floor must compare the action AGE against 30 s. planActionAge() already
// returns Date.now() - lastActionAt; subtracting it from Date.now() again
// inverts the guard in both directions (fresh page defers forever — measured
// live: no reload for 95 s+; post-action page reloads immediately, defeating
// the floor).
func TestAutoRefreshGuardUsesActionAgeDirectly(t *testing.T) {
	invertedRE := regexp.MustCompile(`Date\.now\(\) - window\.planActionAge\(\)`)
	for _, f := range jsCarryingTemplateFiles() {
		body, err := os.ReadFile(f)
		require.NoError(t, err, f)
		require.NotRegexp(t, invertedRE, string(body),
			"%s double-subtracts planActionAge — guard inverted both ways (R8-2)", f)
	}
}

// TestCarePlanScriptCarriesRealTranslations is the rendered half: the served
// page must hand JavaScript the actual words. It fails if the helper silently
// renders empty again — the exact regression template.JS caused, where every
// value vanished and only a test reading the source would have stayed green.
func TestCarePlanScriptCarriesRealTranslations(t *testing.T) {
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)
	_ = f.feedRule(t, models.DB, itemDueSoon(time.Now()))

	req, err := http.NewRequest("GET", baseURL+"/care_plan", nil)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	html := string(raw)

	require.NotContains(t, html, `errorTitle: ,`, "an empty value means the helper is not being called")
	require.Contains(t, html, `errorTitle: "`, "the i18n block must carry real values")

	// Every script block must be free of HTML entities.
	for i, block := range scriptBlockRE.FindAllString(html, -1) {
		require.NotRegexp(t, htmlEntityRE, block,
			"script block %d carries an HTML entity: %s", i, jsSnippet(block))
	}
}

func jsSnippet(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > 220 {
		return s[:220] + "…"
	}
	return s
}
