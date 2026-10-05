package actions

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"creaves/models/careplan"

	"github.com/stretchr/testify/require"
)

// R4-7.2 (bugs.md): on the animal Protocol tab a day whose feeding times
// have passed without anything recorded used to list every dead row —
// "only one apply" for a day that had four. Misses of the last 24 h now
// leave the list and are summarised in ONE pill on the day's last row; an
// older miss stays as history, and the count stops at 5.
func TestAnimalDayHidesRecentMisses(t *testing.T) {
	now := time.Date(2026, 10, 2, 18, 0, 0, 0, time.Local)
	day := time.Date(2026, 10, 2, 0, 0, 0, 0, time.Local)

	item := func(h, m int, status careplan.PlanStatus) CardView {
		return CardView{
			Detail: "grenouilles",
			Status: string(status),
			DueAt:  day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute),
		}
	}

	items := []CardView{
		item(8, 0, careplan.StatusMissing),   // 10 h ago — recent
		item(10, 0, careplan.StatusMissing),  // 8 h ago — recent
		item(2, 0, careplan.StatusScheduled), // older than 24 h? no: same day, but keep it
		item(12, 0, careplan.StatusMissing),  // older than 24 h only if day-1
	}
	// The last one is deliberately two days old: history, not "just missed".
	items[3].DueAt = day.AddDate(0, 0, -2).Add(12 * time.Hour)

	var d AnimalTreatmentDay
	got := dayItemsWithoutMisses(items, now, &d)

	require.Len(t, got, 2, "the two recent misses leave the row list")
	require.Equal(t, 2, d.MissingCount, "both recent misses are counted")
	require.Equal(t, string(careplan.StatusScheduled), got[0].Status,
		"the actionable row stays")
	require.Equal(t, string(careplan.StatusMissing), got[1].Status,
		"an older miss stays visible as history")

	// The cap: a pile of misses never produces a 47.
	var busy AnimalTreatmentDay
	many := make([]CardView, 0, 9)
	for i := 0; i < 9; i++ {
		many = append(many, item(6+i%3, i, careplan.StatusMissing))
	}
	require.Empty(t, dayItemsWithoutMisses(many, now, &busy))
	require.Equal(t, missingCountCap, busy.MissingCount, "the count stops at 5")

	// A miss in the FUTURE is not a miss (clock skew, imported data): it
	// stays as a normal row.
	var future AnimalTreatmentDay
	far := []CardView{item(23, 0, careplan.StatusMissing)}
	require.Len(t, dayItemsWithoutMisses(far, now, &future), 1)
	require.Zero(t, future.MissingCount)

	// The pill reaches assistive tech through the aria-label-free markup:
	// the template renders `care_plan.animal_plans.missing` with the count.
	require.Equal(t, 5, missingCountCap, "the pill count is capped at 5")
}

// R4-7.2 (template): the pill renders once, beside the LAST row, and only
// when the day has something to show.
func TestAnimalDayMissingPillInAllLocales(t *testing.T) {
	forks := []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, `class="badge badge-pill badge-danger ml-1"`, f,
			"the missed-occurrences pill")
		require.Contains(t, raw, `t("care_plan.animal_plans.missing", day.MissingCount)`, f,
			"the pill label is localized with the count")
		require.Contains(t, raw, `i == len(day.Items)-1`, f,
			"the pill rides on the day's LAST row")
		require.Contains(t, raw, "for (i, item) in day.Items", f, "the loop exposes the index")
	}
}

// R4-7.1: the shared modals must live OUTSIDE every tab pane — a modal
// inside a hidden pane shows only Bootstrap's backdrop and locks the page.
func TestAnimalShowModalsOutsideTabPanes(t *testing.T) {
	forks := []string{
		"../templates/animals/show.plush.html",
		"../templates/animals/show.plush.de.html",
		"../templates/animals/show.plush.fr.html",
		"../templates/animals/show.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		for _, marker := range []string{
			`partial("care_plan/apply_toggle.plush.html")`,
			`partial("care_plan/plan_med_toggle.plush.html")`,
		} {
			i := strings.Index(raw, marker)
			require.True(t, i > 0, f+" — "+marker+" is missing")
			for _, pane := range tabPaneRanges(raw) {
				require.False(t, i > pane.start && i < pane.end, f+" — "+marker+
					" must not render INSIDE the tab pane at line "+strconv.Itoa(pane.line))
			}
		}
		// R4-7.3: ONE collapsed card on TOP of the Protocol tab merges the
		// traceability table and the protocol definitions table.
		trace := strings.Index(raw, `id="planTraceTable"`)
		details := strings.Index(raw, `id="planDetailsHead"`)
		days := strings.Index(raw, `id="animalCarePlan"`)
		require.True(t, trace > 0 && details > 0 && days > 0, f)
		require.True(t, details < trace, f+" — the merged card header comes first")
		require.True(t, trace < days, f+" — the merged card sits above the day cards")
		require.NotContains(t, raw, `id="planTraceHead"`, f,
			"the separate traceability card is gone")
	}
}

// R4-7.10: every care_plan.status.* key a template can render must exist in
// every locale file — a missing key showed up literally on the page.
func TestCarePlanStatusKeysLocalizedEverywhere(t *testing.T) {
	used := statusKeysUsedByTemplates(t)
	require.Contains(t, used, "care_plan.status.done", "the done alias is used")

	for _, lang := range []string{"en-us", "fr", "de", "nl"} {
		raw := readTemplate(t, "../locales/care_plan."+lang+".yaml")
		for key := range used {
			require.Contains(t, raw, `- id: "`+key+`"`, lang+" — "+key)
		}
	}
}

// statusKeysUsedByTemplates collects every literal care_plan.status.* key the
// templates can render, across the templates that render statuses.
func statusKeysUsedByTemplates(t *testing.T) map[string]bool {
	// validI18nKey matches an i18n key suffix: lowercase word + underscore —
	// anything else came from prose, not from a t() call.
	validI18nKey := regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

	used := map[string]bool{}
	for _, f := range []string{
		"../templates/care_plan/index.plush.html",
		"../templates/care_plan/_med_series.plush.html",
		"../templates/animals/show.plush.html",
	} {
		for _, key := range literalStatusKeys(readTemplate(t, f), validI18nKey) {
			used[key] = true
		}
	}
	return used
}

// literalStatusKeys extracts the static care_plan.status.* keys of one
// template. A dynamic concatenation — `t("care_plan.status." + x)` — carries no
// literal key to verify and is skipped.
func literalStatusKeys(raw string, validI18nKey *regexp.Regexp) []string {
	var keys []string
	for _, chunk := range strings.Split(raw, "care_plan.status.") {
		if chunk == "" || strings.HasPrefix(chunk, `"`) {
			continue
		}
		key := chunk
		if i := strings.IndexAny(key, `" `); i >= 0 {
			key = key[:i]
		}
		if key != "" && validI18nKey.MatchString(key) {
			keys = append(keys, "care_plan.status."+key)
		}
	}
	return keys
}

// paneRange is the byte span of one `class="tab-pane"` element: the offset of
// its opening tag and the offset just past its closing `</div>`.
type paneRange struct {
	start, end int
	line       int
}

// openDiv is one currently-open `<div>` in the template, and whether that div
// is a tab pane (so its span is the one we report).
type openDiv struct {
	offset int
	isPane bool
	line   int
}

// tabPaneRanges finds every tab pane by walking the template and matching
// `<div` / `</div>` nesting, so a pane ends at the `</div>` that closes it and
// NOT at the first `</div>` of something nested inside it. A naive "after the
// last pane" check is wrong: this page legitimately has panes AFTER the shared
// modals (media, outtake, audit) — what matters is only that the modals are not
// INSIDE one.
func tabPaneRanges(raw string) []paneRange {
	var out []paneRange
	var stack []openDiv
	line := 1
	for i := 0; i < len(raw); {
		switch tag, size := nextDivTag(raw, i); tag {
		case divClose:
			if pane, ok := popDiv(&stack, i+size); ok {
				pane.end = i + size
				out = append(out, pane)
			}
			i += size
		case divOpen:
			end := strings.Index(raw[i:], ">")
			if end < 0 {
				return out
			}
			stack = append(stack, openDiv{
				offset: i,
				isPane: strings.Contains(raw[i:i+end], `class="tab-pane`),
				line:   line,
			})
			i += end + 1
		default:
			if raw[i] == '\n' {
				line++
			}
			i++
		}
	}
	// innermost-first is fine for the "is X inside a pane" check.
	return out
}

// divTag names the kind of div marker at an offset.
const (
	divNone = iota
	divOpen
	divClose
)

// nextDivTag reports which div marker (`<div` / `</div>`) starts at offset i,
// and its length; divNone when neither does. Splitting the scan out of
// tabPaneRanges keeps the nesting logic flat.
func nextDivTag(raw string, i int) (int, int) {
	switch {
	case strings.HasPrefix(raw[i:], "</div"):
		return divClose, len("</div>")
	case strings.HasPrefix(raw[i:], "<div"):
		return divOpen, len("<div")
	}
	return divNone, 0
}

// popDiv removes the innermost open <div and reports whether it was a pane,
// returning its identity so the caller can complete the range.
func popDiv(stack *[]openDiv, closeAt int) (paneRange, bool) {
	s := *stack
	if len(s) == 0 {
		return paneRange{}, false
	}
	d := s[len(s)-1]
	*stack = s[:len(s)-1]
	return paneRange{start: d.offset, line: d.line}, d.isPane
}

// readTemplate reads a template/locale file, failing the test on error.
func readTemplate(t *testing.T, path string) string {
	t.Helper()
	raw, err := os.ReadFile(path)
	require.NoError(t, err, path)
	return string(raw)
}

// carePlanPartialFeeder returns a plush `partialFeeder` (the callback the
// plush `partial()` helper consults for the partial's source) backed by the
// templates directory on disk: "care_plan/plan_slot_toggle.plush.html"
// resolves to templates/care_plan/_plan_slot_toggle.plush.html — the same
// underscore convention Buffalo's renderer uses. Tests that render a
// partial calling nested partials need this in their plush context,
// otherwise plush fails with "could not find partial feeder from helpers".
//
// The nested partial renders with a CHILD of the test's context, so loop
// variables (slot, mg, …) and helpers (t, dueLabel, …) propagate exactly
// as they do in production.
func carePlanPartialFeeder(t *testing.T) func(string) (string, error) {
	t.Helper()
	return func(name string) (string, error) {
		dir, base := filepath.Split(name)
		raw, err := os.ReadFile(filepath.Join("../templates", dir, "_"+base))
		return string(raw), err
	}
}
