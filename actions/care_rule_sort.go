package actions

import (
	"strings"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// careRuleSortColumns maps the public `sort` parameter to a fixed ORDER BY
// SQL expression on the care_rules table. Only keys present in this whitelist
// ever reach the database — user input is used as a map lookup, never inside
// SQL. (Defect D6 — TM-7.)
var careRuleSortColumns = map[string]string{
	"name":     "care_rules.name",
	"kind":     "care_rules.action_kind",
	"priority": "care_rules.priority",
	"active":   "care_rules.active",
}

// careRuleSortClauses is the pure core of applyCareRuleSort: it maps a sort
// key and direction to ORDER BY SQL fragments. `care_rules.name asc` is
// always appended as the secondary key so ties fall back to alphabetical
// order (and the order is stable across pages). Unknown/absent keys fall
// back to the default name-ascending library order.
func careRuleSortClauses(sortKey, dir string) []string {
	col, ok := careRuleSortColumns[sortKey]
	if !ok {
		return []string{"care_rules.name asc"}
	}
	dir = strings.ToLower(dir)
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	if sortKey == "name" {
		return []string{col + " " + dir}
	}
	return []string{col + " " + dir, "care_rules.name asc"}
}

// applyCareRuleSort applies the `sort`/`dir` request parameters to q, falling
// back to the default name-ascending order when they are absent or invalid.
func applyCareRuleSort(q *pop.Query, c buffalo.Context) *pop.Query {
	for _, clause := range careRuleSortClauses(c.Param("sort"), c.Param("dir")) {
		q = q.Order(clause)
	}
	return q
}

// applyCareRuleListFilters applies the D6 list filters (AND-combined):
//   - kind=        exact action_kind match
//   - active=      "true"/"false" (anything else ignored)
//   - matcher_id=  exact matcher match (invalid UUID ignored)
//   - q=           case-insensitive substring over name + description
func applyCareRuleListFilters(q *pop.Query, c buffalo.Context) *pop.Query {
	if k := strings.TrimSpace(c.Param("kind")); k != "" {
		q = q.Where("care_rules.action_kind = ?", k)
	}
	switch strings.ToLower(strings.TrimSpace(c.Param("active"))) {
	case "true":
		q = q.Where("care_rules.active = ?", true)
	case "false":
		q = q.Where("care_rules.active = ?", false)
	}
	if m := strings.TrimSpace(c.Param("matcher_id")); m != "" {
		if id, err := uuid.FromString(m); err == nil {
			q = q.Where("care_rules.matcher_id = ?", id)
		}
	}
	if s := strings.TrimSpace(c.Param("q")); s != "" {
		like := "%" + s + "%"
		q = q.Where(
			"(LOWER(care_rules.name) LIKE LOWER(?) OR LOWER(care_rules.description) LIKE LOWER(?))",
			like, like,
		)
	}
	return q
}
