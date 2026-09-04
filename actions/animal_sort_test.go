package actions

import (
	"net/http/httptest"
	"testing"

	"github.com/gobuffalo/plush/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestAnimalSortClausesPinsDefaultOrdering pins the default (no/unknown sort
// param) ordering: the historical register order animals.id desc. A bad link
// must never be able to alter — let alone inject — ordering SQL.
func TestAnimalSortClausesPinsDefaultOrdering(t *testing.T) {
	for _, sortKey := range []string{"", "bogus", "species; DROP TABLE animals"} {
		assert.Equal(t,
			[]string{"animals.id desc"},
			animalSortClauses(sortKey, ""), "sort=%q", sortKey)
	}
}

// TestAnimalSortClausesPinsWhitelistAndDirection pins whitelist mapping,
// direction handling and the stable secondary keys.
func TestAnimalSortClausesPinsWhitelistAndDirection(t *testing.T) {
	assert.Equal(t,
		[]string{"animals.species desc", "animals.id desc"},
		animalSortClauses("species", "desc"))

	// invalid dir falls back to asc
	assert.Equal(t,
		[]string{"animals.species asc", "animals.id desc"},
		animalSortClauses("species", "DROP"))

	// number sorts yearNumber first, newest year second
	assert.Equal(t,
		[]string{"animals.yearNumber asc", "animals.year desc", "animals.id desc"},
		animalSortClauses("number", "asc"))

	// intake date sorts via scalar subquery (no join, no DISTINCT duplication)
	assert.Equal(t,
		[]string{"(SELECT i.date FROM intakes i WHERE i.id = animals.intake_id) desc", "animals.id desc"},
		animalSortClauses("intake_date", "desc"))

	// every key listed for the templates must exist in the whitelist
	for _, field := range animalSortFields {
		assert.Contains(t, animalSortColumns, field)
	}
}

// TestAnimalSortLinkPinsToggleAndPreservation pins the header-link helper:
// direction toggles on the active column, filters are preserved, page resets.
func TestAnimalSortLinkPinsToggleAndPreservation(t *testing.T) {
	build := func(target string) plush.HelperContext {
		req := httptest.NewRequest("GET", target, nil)
		return plush.HelperContext{Context: plush.NewContextWith(map[string]interface{}{"request": req})}
	}

	// inactive column: ascending link, filters + page kept, sort/dir replaced
	href, err := sortLink("cage", build("/animals?species=Testsp&page=3"))
	require.NoError(t, err)
	assert.Equal(t, "/animals?dir=asc&sort=cage&species=Testsp", string(href))

	// active column asc: toggles to desc
	href, err = sortLink("species", build("/animals?sort=species&dir=asc&page=3"))
	require.NoError(t, err)
	assert.Equal(t, "/animals?dir=desc&sort=species", string(href))

	// active column desc: toggles back to asc
	href, err = sortLink("species", build("/animals?sort=species&dir=desc"))
	require.NoError(t, err)
	assert.Equal(t, "/animals?dir=asc&sort=species", string(href))
}

// TestAnimalSortIconPinsIndicators pins the ▲/▼ indicator helper.
func TestAnimalSortIconPinsIndicators(t *testing.T) {
	build := func(target string) plush.HelperContext {
		req := httptest.NewRequest("GET", target, nil)
		return plush.HelperContext{Context: plush.NewContextWith(map[string]interface{}{"request": req})}
	}

	icon, err := sortIcon("species", build("/animals?sort=species&dir=asc"))
	require.NoError(t, err)
	assert.Equal(t, "▲", string(icon))

	icon, err = sortIcon("species", build("/animals?sort=species&dir=desc"))
	require.NoError(t, err)
	assert.Equal(t, "▼", string(icon))

	icon, err = sortIcon("cage", build("/animals?sort=species&dir=asc"))
	require.NoError(t, err)
	assert.Equal(t, "", string(icon))
}
