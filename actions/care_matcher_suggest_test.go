package actions

// Context-aware value suggestions (bugs.md U27 R5-1c, D-f): POST
// /care_matchers/suggest evaluates the expression prefix (the clauses ABOVE
// the edited one) over the in-care sampling and returns the distinct values
// of the edited field among the still-matching animals, with counts.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/require"
)

func TestCareMatcherSuggest(t *testing.T) {
	f := setupPlanFixture(t)
	admin, adminURL := planAdminClient(t)
	adminToken := planToken(t, admin, adminURL)
	reg, regURL := planRegularClient(t)
	regToken := planToken(t, reg, regURL)

	created := map[int]bool{}
	mk := func(species, cage string) int {
		id := f.mkAnimal(t, models.DB, species)
		require.NoError(t, models.DB.RawQuery("UPDATE animals SET cage = ? WHERE id = ?", cage, id).Exec())
		created[id] = true
		return id
	}
	mk("Renard roux", "Enclos Renards")
	mk("Renard roux", "enclos renards")
	mk("Renard roux", "VE5")
	mk("Renard roux", "VE5")
	mk("Hérisson d'Europe", f.cage)
	t.Cleanup(func() {
		for id := range created {
			models.DB.RawQuery("DELETE FROM animals WHERE id = ?", id).Exec()
		}
	})

	var out struct {
		Values []struct {
			Value string `json:"value"`
			Count int    `json:"count"`
		} `json:"values"`
		MatchedTotal int `json:"matched_total"`
	}

	// regular users are refused like every plan admin surface
	code, _ := planDoJSON(t, reg, regURL, "POST", "/care_matchers/suggest", regToken, map[string]interface{}{
		"expression_prefix": `species IN ("Renard roux")`, "field": "cage",
	})
	require.Equal(t, http.StatusForbidden, code)

	// context-aware: only the foxes remain after the species prefix —
	// case-variant cages come back with their counts, the hedgehog cage
	// does not (U26's exact scenario in miniature).
	code, raw := planDoJSON(t, admin, adminURL, "POST", "/care_matchers/suggest", adminToken, map[string]interface{}{
		"expression_prefix": `species IN ("Renard roux")`,
		"field":             "cage",
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, decodeSuggest(t, raw, &out))
	require.Equal(t, 4, out.MatchedTotal)
	byValue := map[string]int{}
	for _, v := range out.Values {
		byValue[v.Value] = v.Count
	}
	require.Len(t, out.Values, 3)
	require.Equal(t, 1, byValue["Enclos Renards"])
	require.Equal(t, 1, byValue["enclos renards"])
	require.Equal(t, 2, byValue["VE5"])
	require.NotContains(t, byValue, f.cage)

	// empty prefix → first clause sees every in-care animal in the DB
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_matchers/suggest", adminToken, map[string]interface{}{
		"expression_prefix": "",
		"field":             "cage",
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, decodeSuggest(t, raw, &out))
	require.GreaterOrEqual(t, out.MatchedTotal, 5)
	byValue = map[string]int{}
	for _, v := range out.Values {
		byValue[v.Value] = v.Count
	}
	require.GreaterOrEqual(t, byValue[f.cage], 1) // fixture cage: CP-A1 + CP-A2 + the hedgehog

	// unknown field → an empty answer, not an error
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_matchers/suggest", adminToken, map[string]interface{}{
		"expression_prefix": `species = "Renard roux"`,
		"field":             "nope",
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, decodeSuggest(t, raw, &out))
	require.Empty(t, out.Values)
	require.Equal(t, 0, out.MatchedTotal)

	// broken prefix → the parser's 422 (same shape as /preview)
	code, _ = planDoJSON(t, admin, adminURL, "POST", "/care_matchers/suggest", adminToken, map[string]interface{}{
		"expression_prefix": `species ~ "("`,
		"field":             "cage",
	})
	require.Equal(t, http.StatusUnprocessableEntity, code)

	// the ≤50 value cap is respected
	for i := 0; i < 55; i++ {
		mk("Renard roux", fmt.Sprintf("CAP-%s-%02d", f.marker, i))
	}
	code, raw = planDoJSON(t, admin, adminURL, "POST", "/care_matchers/suggest", adminToken, map[string]interface{}{
		"expression_prefix": `species IN ("Renard roux")`,
		"field":             "cage",
	})
	require.Equal(t, http.StatusOK, code, "body: %s", raw)
	require.NoError(t, decodeSuggest(t, raw, &out))
	require.Len(t, out.Values, 50)
	require.Equal(t, 59, out.MatchedTotal)
}

func decodeSuggest(t *testing.T, raw []byte, out interface{}) error {
	t.Helper()
	return json.Unmarshal(raw, out)
}
