package actions

// Phase 0b equivalence harness (docs/care-presentation-guideline.md §7).
//
// renderPlanDOM renders /care_plan for a rich fixture (feeding + medication
// + cleanup + history) across every kind tab and returns a NORMALIZED DOM
// serialization: the same structure/attributes/text, with whitespace-only
// text nodes dropped and attribute order canonicalized. The refactor must
// leave this serialization unchanged for every section except the two
// DELIBERATE deltas the phase introduces:
//   - History becomes a first-class .plan-tier panel (was a bare button).
//   - Toggle buttons lose the hardcoded btn-outline-secondary (colour now
//     comes from the tier policy, §2).
//
// normalizePlanDOM parses HTML and walks it emitting one line per node:
//   <tag attr="v" attr2="v2">   (attributes sorted)
//   "text"                      (non-whitespace text, trimmed)
//   </tag>
// Indentation encodes depth, so a structural diff is a plain text diff.

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// phase0bVolatile matches the run-to-run variable parts of a rendered page —
// CSRF tokens, timestamps, UUIDs, asset digests, the fixture marker and the
// random animal/cage numbers — so the DOM baseline survives a fresh fixture.
// Each is replaced by a stable placeholder BEFORE parsing.
var phase0bVolatile = []struct {
	re  *regexp.Regexp
	rep string
}{
	{regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), "UUID"},
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[+-]\d{2}:\d{2}`), "RFC"},
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z`), "RFC"},
	{regexp.MustCompile(`application\.[0-9a-f]{20}\.(css|js)`), "application.HASH.$1"},
	{regexp.MustCompile(`name="csrf-token" content="[^"]*"`), `name="csrf-token" content="CSRF"`},
	{regexp.MustCompile(`CP-[0-9a-f]{8}`), "CP-MARKER"},
	// The fixture marker suffix rides on rule/drug/question names too
	// (RMED-DOM-<m>, ROBS-L-<m>, CPDrug-<m>, Q-L-<m>, …).
	{regexp.MustCompile(`-[0-9a-f]{8}\b`), "-MARKER"},
	{regexp.MustCompile(`cfgtest_admin_[0-9a-f]{8}`), "ADMIN"},
	{regexp.MustCompile(`\b\d{6}/\d{2}\b`), "YEARNUM"},
	// Animal primary keys are auto-increment and differ per run. The
	// data-items JSON arrives HTML-escaped (&quot;), so match both forms.
	{regexp.MustCompile(`/animals/\d+`), "/animals/AID"},
	{regexp.MustCompile(`data-animal-id="\d+"`), `data-animal-id="AID"`},
	{regexp.MustCompile(`(?:&quot;|&#34;|")animal_id(?:&quot;|&#34;|"):\s*\d+`), `animal_id:AID`},
	{regexp.MustCompile(`\("Animal",\s*\d+\)`), `("Animal", AID)`},
	// "Updated at HH:MM" indicator (auto-refresh) — re-rendered each run.
	{regexp.MustCompile(`(Updated at |Mis à jour |Aktualisiert |Bijgewerkt )\d{2}:\d{2}`), "${1}TIME"},
	// Every due-time label (HH:MM) derives from the fixture's wall-clock
	// "now", so it differs between the baseline run and the check run.
	{regexp.MustCompile(`\b\d{2}:\d{2}\b`), "HHMM"},
	// Day-qualifier badges ("DD/MM", DueLabelPartsOf) drift with the wall
	// clock the same way the HH:MM labels do — a baseline recorded
	// yesterday fails today on the "later" tier alone. Mask them.
	{regexp.MustCompile(`\b\d{2}/\d{2}\b`), "DDMM"},
	// The medication SLOT BUCKET word rides on toggle titles
	// (`title="Apply — Noon"`, `title="Evening"`) and flips when the
	// wall clock crosses a bucket boundary between runs — same structural
	// DOM, different localized word. All four locales.
	{regexp.MustCompile(`— (Morning|Noon|Evening|Matin|Midi|Soir|Morgen|Mittag|Abend|Ochtend|Middag|Avond)"`), `— SLOT"`},
	{regexp.MustCompile(`title="(Morning|Noon|Evening|Matin|Midi|Soir|Morgen|Mittag|Abend|Ochtend|Middag|Avond)"`), `title="SLOT"`},
}

// maskVolatile replaces every run-variable substring with a placeholder so
// two renders of the SAME structure compare equal across fixture runs.
func maskVolatile(raw string) string {
	for _, v := range phase0bVolatile {
		raw = v.re.ReplaceAllString(raw, v.rep)
	}
	return raw
}

// planFixtureRich builds a plan fixture exercising every tier/kind branch:
// feeding (grouped cage×diet), medication (tiered series + a Done series),
// cleanup (cage rows) and history (an applied terminal row).
func planFixtureRich(t *testing.T) (*planFixture, *http.Client, string) {
	t.Helper()
	f := setupPlanFixture(t)
	client, baseURL := planAdminClient(t)

	now := time.Now()
	soon := itemDueSoon(now)              // due-now tier
	late := now.Add(-2 * time.Hour)       // late tier (past due, still open)
	later := now.Add(26 * time.Hour)      // later tier (future, beyond today)

	// Feeding: one cage × diet group due now (tiered grouped list).
	f.feedRule(t, models.DB, soon)

	// Observation (row kind): one LATE, one NOW, one LATER occurrence so the
	// row-kind tiers (Late/Now/Later) all render; a fourth is APPLIED below
	// to populate History.
	obsLate := ruleWithoutMatcher(t, models.DB, "ROBS-L-"+f.marker, "observation",
		planRulePayload(t, "observation", map[string]interface{}{"question": "Q-L-" + f.marker}),
		careScheduleJSON(t, late))
	_ = obsLate
	ruleWithoutMatcher(t, models.DB, "ROBS-N-"+f.marker, "observation",
		planRulePayload(t, "observation", map[string]interface{}{"question": "Q-N-" + f.marker}),
		careScheduleJSON(t, soon))
	// NOTE: obsHist is due 1 minute AFTER obsNow — same tier, but a distinct
	// sort key. Two rules sharing the exact same due time order
	// non-deterministically within the tier (flaky baseline).
	obsHist := ruleWithoutMatcher(t, models.DB, "ROBS-H-"+f.marker, "observation",
		planRulePayload(t, "observation", map[string]interface{}{"question": "Q-H-" + f.marker}),
		careScheduleJSON(t, soon.Add(1*time.Minute)))
	ruleWithoutMatcher(t, models.DB, "ROBS-F-"+f.marker, "observation",
		planRulePayload(t, "observation", map[string]interface{}{"question": "Q-F-" + f.marker}),
		careScheduleJSON(t, later))

	// Medication: a single-slot rule; one occurrence is APPLIED below so the
	// medication page renders both an open tier and a Done/history row.
	med := ruleWithoutMatcher(t, models.DB, "RMED-DOM-"+f.marker, "medication",
		planRulePayload(t, "medication", map[string]interface{}{"drug": "CPDrug-" + f.marker, "dosage": "0.5 ml"}),
		careScheduleJSON(t, soon))

	// Cleanup: a cage rule so the cleanup kind renders rows.
	ruleWithoutMatcher(t, models.DB, "RCLN-DOM-"+f.marker, "cleanup",
		planRulePayload(t, "cleanup", map[string]interface{}{"caretype_id": f.defCare.String()}),
		careScheduleJSON(t, soon))

	// Apply ONE observation + ONE medication occurrence so the page has
	// terminal rows (history) in addition to the open tiers.
	token := planToken(t, client, baseURL)
	_, body := planGetJSON(t, client, baseURL, "/care_plan")
	items := planItemsOf(t, body)
	if it := findItemFrom(items, f.animalIDs[0], "observation", obsHist.ID); it != nil {
		ref := itemRef(it)
		ref["answer"] = "yes"
		code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, ref)
		require.Equal(t, http.StatusCreated, code, "apply obs: %s", raw)
	}
	if it := findItemFrom(items, f.animalIDs[0], "medication", med.ID); it != nil {
		code, raw := planDoJSON(t, client, baseURL, "POST", "/care_plan/apply", token, itemRef(it))
		require.Equal(t, http.StatusCreated, code, "apply med: %s", raw)
	}

	return f, client, baseURL
}

// fetchPlanHTML GETs /care_plan (HTML) for a kind and returns the body.
func fetchPlanHTML(t *testing.T, client *http.Client, baseURL, query string) string {
	t.Helper()
	req, err := http.NewRequest("GET", baseURL+"/care_plan"+query, nil)
	require.NoError(t, err)
	req.Header.Set("Accept", "text/html")
	resp, err := client.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	require.Equal(t, http.StatusOK, resp.StatusCode, "GET %s: %.300s", query, raw)
	return maskVolatile(string(raw))
}

// normalizePlanDOM parses HTML and emits the canonical structural form.
func normalizePlanDOM(t *testing.T, raw string) string {
	t.Helper()
	doc, err := html.Parse(strings.NewReader(raw))
	require.NoError(t, err)
	var b strings.Builder
	var walk func(n *html.Node, depth int)
	walk = func(n *html.Node, depth int) {
		switch n.Type {
		case html.ElementNode:
			indent := strings.Repeat("  ", depth)
			attrs := make([]string, 0, len(n.Attr))
			for _, a := range n.Attr {
				attrs = append(attrs, fmt.Sprintf(`%s="%s"`, a.Key, a.Val))
			}
			sort.Strings(attrs)
			if len(attrs) > 0 {
				fmt.Fprintf(&b, "%s<%s %s>\n", indent, n.Data, strings.Join(attrs, " "))
			} else {
				fmt.Fprintf(&b, "%s<%s>\n", indent, n.Data)
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, depth+1)
			}
			fmt.Fprintf(&b, "%s</%s>\n", indent, n.Data)
		case html.TextNode:
			txt := strings.TrimSpace(n.Data)
			if txt != "" {
				fmt.Fprintf(&b, "%s\"%s\"\n", strings.Repeat("  ", depth), txt)
			}
		default:
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c, depth)
			}
		}
	}
	walk(doc, 0)
	return b.String()
}

// phase0bKinds lists the kind tabs rendered for the equivalence sweep.
var phase0bKinds = []string{"feeding", "medication", "cleanup", "observation"}

// TestCarePlanPhase0bDOMEquivalence renders /care_plan for every kind tab
// and compares the normalized DOM to the recorded baseline. Record the
// baseline with:  PHASE0B_RECORD=1 go test ./actions -run TestCarePlanPhase0bDOMEquivalence
// The baseline is re-recorded whenever the phase lands one of its two
// DOCUMENTED deliberate deltas; each re-record is justified by an
// equivalence diff showing everything else unchanged:
//   - History became a first-class .plan-tier panel (was a bare button):
//     verified 0-diff outside the history block on all 4 kind tabs.
//   - Toggle buttons lose the hardcoded btn-outline-secondary (colour now
//     comes from the tier policy, §2).
//   - B10-7: scheduled cleanup occurrences within the future horizon render
//     as open work — the cleanup baseline gained the "tomorrow HHMM" time
//     group and its ○/✓ slots; feeding/medication/observation 0-diff.
//   - B10-10: cleanup group rows collapse with count/extra-time header; cage
//     group apply carries earliest time, while per-animal toggles retain time.
//   - Bug 2026-10-27 #1/#2: the feeding group Apply button quotes the earliest
//     applicable occurrence ("○ HHMM", day-qualified) — the bare clock icon
//     is gone; feeding 1-line delta, other kinds 0-diff.
//   - Bug 2026-10-27 #3a: the medication animal cell wires the shared column
//     width again (style=min-width NNch from the widest label).
//   - Bug 2026-10-27 #5: a multi-animal cleanup cage renders ONE line per
//     animal (.plan-animal-row: number link at line level + its toggles);
//     single-animal cages would render the .plan-apply-space spacer (the
//     rich fixture has none — covered by the viewmodel test + E2E).
//   - Bug 2026-10-27 #4: observation/care/weighing item lines adopt the med
//     row layout classes (d-flex align-items-start flex-wrap border-bottom
//     py-1); text colour delta is text-info on the task (not in the DOM
//     normalization, which drops classes' effect but keeps attribute lists).
//   - Bug 2026-10-27 #3b + #8 (second review batch): the kind tab bar follows
//     the importance order Medication · Care · Feeding · Observation ·
//     Weighing · Cleanup on every kind tab (order-only delta, counts
//     unchanged); the feeding chip rows lose the `plan-time-label` sub-group
//     header (no time on top of the animal — each ○/✓ toggle carries its own
//     time) and their button wrapper gains ml-2 spacing. Verified by diff:
//     medication/cleanup/observation tab-order only; feeding the three
//     markup lines above, nothing else.
//   - Bug 2026-10-27 #2 (Treatment tab retired): the med-line animal links
//     and ℹ detail buttons retarget #nav-treatment → #nav-plan (the series
//     render on the protocol tab now). Verified by diff: medication tab
//     fragment swaps only; feeding/cleanup/observation 0-diff.
//   - Parity batch 2026-10-27 (TestTemplateVariantStructuralParity debt paydown):
//     base application layout gained <html lang="<%= uiLang() %>"> — the fr/de/nl
//     forks already carried a (hardcoded) lang attribute, so the rendered root
//     gains lang=<locale>; one-line delta on every kind tab, no other change
//     (verified by the equivalence diff on re-record).
func TestCarePlanPhase0bDOMEquivalence(t *testing.T) {
	f, client, baseURL := planFixtureRich(t)
	_ = f

	record := os.Getenv("PHASE0B_RECORD") == "1"
	historySeen := false
	for _, kind := range phase0bKinds {
		raw := fetchPlanHTML(t, client, baseURL, "?kind="+kind)
		// Deliberate delta 1: when the kind tab has terminal rows, History
		// renders as a .plan-tier panel whose collapse body keeps the
		// pre-refactor id #plan-history-rows.
		if strings.Contains(raw, `id="plan-history"`) {
			historySeen = true
			require.Contains(t, raw, `class="plan-tier plan-tier-history" id="plan-history"`,
				"history must be a .plan-tier panel (kind=%s)", kind)
		}
		norm := normalizePlanDOM(t, raw)
		path := filepath.Join("testdata", "phase0b_baseline_"+kind+".txt")
		if record {
			require.NoError(t, os.WriteFile(path, []byte(norm), 0644))
			continue
		}
		base, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("no baseline recorded for kind=%s (%v); record with PHASE0B_RECORD=1", kind, err)
		}
		require.Equal(t, string(base), norm, "DOM drift on kind=%s", kind)
	}
	require.True(t, historySeen, "fixture must render the History tier on at least one kind tab")
}
