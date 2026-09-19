package actions

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

// inputRe / selectRe / textareaRe harvest the animal edit form so the test
// can replay exactly what a browser would post.
var (
	inputRe    = regexp.MustCompile(`(?is)<input[^>]*name="([^"]+)"[^>]*>`)
	inputValRe = regexp.MustCompile(`(?is)value="([^"]*)"`)

	selectRe   = regexp.MustCompile(`(?is)<select[^>]*name="([^"]+)"[^>]*>(.*?)</select>`)
	selectedRe = regexp.MustCompile(`(?is)<option[^>]*value="([^"]*)"[^>]*selected`)
	optionRe   = regexp.MustCompile(`(?is)<option[^>]*value="([^"]*)"`)

	textareaRe = regexp.MustCompile(`(?is)<textarea[^>]*name="([^"]+)"[^>]*>(.*?)</textarea>`)
)

// parseFormFields extracts name/value pairs from a rendered form page:
// inputs keep their value, selects the selected (or first) option,
// textareas their content. File inputs and unnamed fields are skipped.
func parseFormFields(t *testing.T, html string) url.Values {
	t.Helper()
	form := url.Values{}
	seen := map[string]bool{}
	parseInputFields(t, html, form, seen)
	parseSelectFields(t, html, form, seen)
	parseTextareaFields(t, html, form, seen)
	return form
}

// parseInputFields adds plain input fields; checkboxes are skipped because
// callers decide their posted value, file inputs and dupes are skipped too.
func parseInputFields(t *testing.T, html string, form url.Values, seen map[string]bool) {
	t.Helper()
	for _, m := range inputRe.FindAllStringSubmatch(html, -1) {
		name := m[1]
		if name == "" || seen[name] || strings.Contains(m[0], `type="file"`) {
			continue
		}
		if strings.Contains(m[0], `type="checkbox"`) {
			continue
		}
		val := ""
		if vm := inputValRe.FindStringSubmatch(m[0]); vm != nil {
			val = vm[1]
		}
		form.Set(name, val)
		seen[name] = true
	}
}

// parseSelectFields adds selects with their selected (or first) option value.
func parseSelectFields(t *testing.T, html string, form url.Values, seen map[string]bool) {
	t.Helper()
	for _, m := range selectRe.FindAllStringSubmatch(html, -1) {
		name := m[1]
		if name == "" || seen[name] {
			continue
		}
		val := ""
		if sm := selectedRe.FindStringSubmatch(m[2]); sm != nil {
			val = sm[1]
		} else if om := optionRe.FindStringSubmatch(m[2]); om != nil {
			val = om[1]
		}
		form.Set(name, val)
		seen[name] = true
	}
}

// parseTextareaFields adds textareas with their trimmed content.
func parseTextareaFields(t *testing.T, html string, form url.Values, seen map[string]bool) {
	t.Helper()
	for _, m := range textareaRe.FindAllStringSubmatch(html, -1) {
		name := m[1]
		if name == "" || seen[name] {
			continue
		}
		form.Set(name, strings.TrimSpace(m[2]))
		seen[name] = true
	}
}

// TestAnimalReadyForReleaseFlag: the "ready for release" checkbox on the
// animal general tab persists through the update flow and can be cleared
// again; the edit form reflects the stored state (#197 sub-item 8).
func TestAnimalReadyForReleaseFlag(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB
	f := createQuickOuttakeFixture(t, tx)
	client, baseURL := adminClientWithURL(t)

	editURL := fmt.Sprintf("%s/animals/%d/edit", baseURL, f.freeID)
	token := todoToken(t, client, baseURL, fmt.Sprintf("/animals/%d/edit", f.freeID))
	editHTML := fetchPageGET(t, client, editURL)
	require.Contains(t, editHTML, `name="ReadyForRelease"`)

	reloadForm := func() url.Values {
		t.Helper()
		raw := parseFormFields(t, fetchPageGET(t, client, editURL))
		// The rendered reference selects (animaltype, age, ...) come from a
		// per-process cached list: a freshly seeded fixture type/age is not in
		// it yet, so those selects render without a selected option and the
		// parser falls back to the leading empty option. Posting "" would
		// break the uuid binder, and the handler already knows the loaded
		// values — drop empty keys so the replay only changes the flag.
		form := url.Values{}
		for k, vs := range raw {
			if len(vs) > 0 && vs[0] != "" {
				form[k] = vs
			}
		}
		form.Set("_method", "PUT")
		return form
	}

	// 1. Check the flag: CheckboxTag renders the checkbox first, then the
	// hidden unchecked input, so a browser posts ["true","false"] and the
	// binder keeps the first value.
	form := reloadForm()
	form.Set("ReadyForRelease", "true")
	form.Add("ReadyForRelease", "false")
	resp := postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	b, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "update with flag on failed: %s", string(b))

	a := models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	require.True(t, a.ReadyForRelease.Valid, "flag must be stored")
	require.True(t, a.ReadyForRelease.Bool, "flag must be true")

	// 2. The edit form reflects the stored state.
	editHTML = fetchPageGET(t, client, editURL)
	require.Regexp(t, `(?is)<input[^>]*name="ReadyForRelease"[^>]*checked`, editHTML)

	// 3. Uncheck: only the hidden false is posted.
	form = reloadForm()
	form.Set("ReadyForRelease", "false")
	resp = postTodoForm(t, client, baseURL, fmt.Sprintf("/animals/%d", f.freeID), token, form)
	require.Equal(t, http.StatusSeeOther, resp.StatusCode, "update with flag off failed")

	a = models.Animal{}
	require.NoError(t, tx.Find(&a, f.freeID))
	require.True(t, a.ReadyForRelease.Valid, "flag must be stored as false")
	require.False(t, a.ReadyForRelease.Bool, "flag must be cleared")
}
