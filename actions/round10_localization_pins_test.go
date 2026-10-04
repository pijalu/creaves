package actions

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"creaves/models"
	"creaves/models/careplan"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/require"
)

// Round 10 (docs/round-10-full-assessment.md) pins. Each finding below
// shipped broken and was found by crawling all screens in all four
// languages; the pins keep the class of bug out.

// R10-1: /discoveries/new + /discoveries/{id}/edit returned 500 in every
// locale — the form is bound to the Discovery model itself, so
// `f.InputTag("Discovery.PostalCode")` has no nested `Discovery` struct to
// walk and the tags helper panicked on a zero reflect.Value.
func TestDiscoveryFormFieldPathsAreOnTheModel(t *testing.T) {
	forks := []string{
		"../templates/discoveries/_form.plush.html",
		"../templates/discoveries/_form.plush.fr.html",
		"../templates/discoveries/_form.plush.de.html",
		"../templates/discoveries/_form.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.NotContains(t, raw, `f.InputTag("Discovery.`, f+
			": the form binds `discovery` — there is no nested Discovery struct")
		require.Contains(t, raw, `f.InputTag("PostalCode"`, f+": postal code must bind")
		require.Contains(t, raw, `f.InputTag("City"`, f+": city must bind")
		// The autoComplete selectors must match the renamed inputs.
		require.NotContains(t, raw, `input[name="Discovery.`, f)
		require.Contains(t, raw, `input[name="PostalCode"]`, f)
	}
}

// R10-3: the dashboard shipped as four byte-identical English copies —
// every section title and table header was hardcoded English in every
// locale. All visible strings must go through t("dashboard.…").
func TestDashboardRendersNoHardcodedChrome(t *testing.T) {
	forks := []string{
		"../templates/dashboard/dashboard.plush.html",
		"../templates/dashboard/dashboard.plush.fr.html",
		"../templates/dashboard/dashboard.plush.de.html",
		"../templates/dashboard/dashboard.plush.nl.html",
	}
	english := []string{
		"Animals in alert", "weight loss", "Veterinary visits today",
		"Log entries (last 24h)", "Animals in cares", "<p>None</p>",
		"<th>Date</th>", "<th>Animal</th>", "<th>Species</th>",
		"<th>Weights</th>", "<th>Diagnostic</th>", "<th>Count</th>",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		for _, s := range english {
			require.NotContains(t, raw, s, f+": dashboard chrome must be localized")
		}
	}
	// And the keys the template now references exist in every locale file.
	for _, loc := range []string{"en-us", "fr", "de", "nl"} {
		yaml := readTemplate(t, "../locales/dashboard."+loc+".yaml")
		for _, k := range []string{
			"dashboard.alerts.title", "dashboard.weight_loss.title",
			"dashboard.vet_visits.title", "dashboard.log.title",
			"dashboard.stats.in_care", "dashboard.col.species", "dashboard.none",
		} {
			require.Contains(t, yaml, `id: "`+k+`"`, loc)
		}
	}
}

// R10-4: the DE/NL animal-show forks kept English labels ("Cage", "Age",
// "Date", "Note", "Contacts", "Parasites") — each fork carries its own
// language, so the pin is per fork.
func TestAnimalShowForksCarryTheirOwnLanguage(t *testing.T) {
	cases := []struct{ fork, bad, good string }{
		{"../templates/animals/show.plush.de.html", ">Age<", ">Alter<"},
		{"../templates/animals/show.plush.de.html", ">Date<", ">Datum<"},
		{"../templates/animals/show.plush.de.html", ">Cage<", ">Käfig<"},
		{"../templates/animals/show.plush.de.html", ">Parasites<", ">Parasiten<"},
		{"../templates/animals/show.plush.nl.html", ">Age<", ">Leeftijd<"},
		{"../templates/animals/show.plush.nl.html", ">Date<", ">Datum<"},
		{"../templates/animals/show.plush.nl.html", ">Cage<", ">Kooi<"},
		{"../templates/animals/show.plush.nl.html", ">Parasites<", ">Parasieten<"},
	}
	for _, c := range cases {
		raw := readTemplate(t, c.fork)
		require.NotContains(t, raw, c.bad, c.fork)
		require.Contains(t, raw, c.good, c.fork)
		// The §10.5-N1 subtitle must not leak the English word either.
		require.NotContains(t, raw, "%>Cage <%= animal.Cage %>", c.fork)
	}
}

// R10-6: the intakes index was a scaffold stub — one empty header cell and
// rows of bare action buttons. It must render real data columns.
func TestIntakesIndexHasDataColumns(t *testing.T) {
	forks := []string{
		"../templates/intakes/index.plush.html",
		"../templates/intakes/index.plush.fr.html",
		"../templates/intakes/index.plush.de.html",
		"../templates/intakes/index.plush.nl.html",
	}
	for _, f := range forks {
		raw := readTemplate(t, f)
		require.Contains(t, raw, "<%= intake.DateFormated() %>", f)
		require.Contains(t, raw, "<%= intake.General %>", f)
		require.Contains(t, raw, "<%= intake.Remarks %>", f)
		// The stub signature was a table with a LONE empty header cell;
		// the real table has five data headers plus the actions column.
		require.GreaterOrEqual(t, strings.Count(raw, "<th>"), 6, f)
	}
}

// R10-2: species labels — the plan label convention (§10.5-N1) reads
// animals.species raw (canonical French). localizePlanSpecies rewrites the
// plan's display rows through the request's tspecies helper and must
// leave the matcher contexts untouched; without a tspecies helper in the
// context (grifts, tests) it is a no-op, not a panic.
func TestLocalizePlanSpeciesRewritesRowsOnly(t *testing.T) {
	row := models.Animal{Species: "Hérisson", Cage: nulls.NewString("A12")}
	plan := &DayPlan{Animals: &planAnimals{
		ctxs: map[int]*careplan.AnimalContext{},
		rows: map[int]models.Animal{7: row},
	}}

	a := buffalo.New(buffalo.Options{Env: "test"})
	var ran bool
	a.GET("/probe", func(c buffalo.Context) error {
		localizePlanSpecies(c, plan)
		got, ok := plan.AnimalRow(7)
		require.True(t, ok)
		require.Equal(t, "Hérisson", got.Species,
			"no tspecies helper in context → identity, never a panic")
		ran = true
		return c.Render(http.StatusOK, nil)
	})
	w := httptest.NewRecorder()
	a.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/probe", nil))
	require.Equal(t, http.StatusOK, w.Code)
	require.True(t, ran)
}

// R10-5: the preferences save must use the keys that actually exist and
// keep both flash branches reachable.
func TestPreferenceSaveFlashKeysExist(t *testing.T) {
	for _, loc := range []string{"en-us", "fr", "de", "nl"} {
		yaml := readTemplate(t, "../locales/all."+loc+".yaml")
		require.Contains(t, yaml, `id: "preferences.ok"`, loc)
		require.Contains(t, yaml, `id: "preferences.err"`, loc)
	}
	src := readTemplate(t, "../actions/preferences.go")
	require.NotContains(t, src, "preferences.update.ok")
	require.NotContains(t, src, "preferences.update.error")
	// The dead-code shape that ate the success flash: a redirect followed
	// by more statements inside the same block.
	require.NotContains(t, src, "preferences.err\")\n\t\treturn c.Redirect(http.StatusSeeOther, \"/preferences\")\n\t}\n\tc.Flash().Add(\"success\"", src)
	require.Contains(t, src, `T.Translate(c, "preferences.ok")`)
}

var _ = strings.TrimSpace // keep strings imported for future assertions
