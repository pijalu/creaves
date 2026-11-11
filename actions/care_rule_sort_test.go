package actions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestCareRuleSortClausesPinsDefaultOrdering pins the default (no/unknown
// sort param) ordering: name ascending. A bad link must never be able to
// alter — let alone inject — ordering SQL.
func TestCareRuleSortClausesPinsDefaultOrdering(t *testing.T) {
	for _, sortKey := range []string{"", "bogus", "name; DROP TABLE care_rules", "id desc"} {
		assert.Equal(t,
			[]string{"care_rules.name asc"},
			careRuleSortClauses(sortKey, "desc"), "sort=%q", sortKey)
	}
}

// TestCareRuleSortClausesPinsWhitelistAndDirection pins whitelist mapping,
// direction handling and the name-ascending tiebreak.
func TestCareRuleSortClausesPinsWhitelistAndDirection(t *testing.T) {
	assert.Equal(t,
		[]string{"care_rules.action_kind desc", "care_rules.name asc"},
		careRuleSortClauses("kind", "desc"))

	// invalid dir falls back to asc
	assert.Equal(t,
		[]string{"care_rules.priority asc", "care_rules.name asc"},
		careRuleSortClauses("priority", "DROP"))

	assert.Equal(t,
		[]string{"care_rules.active asc", "care_rules.name asc"},
		careRuleSortClauses("active", "ASC"))

	// name is the primary AND the tiebreak: single clause.
	assert.Equal(t,
		[]string{"care_rules.name desc"},
		careRuleSortClauses("name", "desc"))

	// every key in the TM-7 whitelist must be mapped
	for _, field := range []string{"name", "kind", "priority", "active"} {
		assert.Contains(t, careRuleSortColumns, field)
	}
}
