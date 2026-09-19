package actions

import (
	"strings"

	"github.com/gobuffalo/pop/v6"
)

// userSortClauses is the pure core of the users listing sort: it maps a sort
// key and direction to ORDER BY SQL fragments. `first_name asc, last_name
// asc` (volunteer name, issue #107) is the default; `login asc` is always
// appended last so the order is stable. The direction applies to every
// column of the key (name = first + last name).
func userSortClauses(sortKey, dir string) []string {
	dir = strings.ToLower(dir)
	if dir != "asc" && dir != "desc" {
		dir = "asc"
	}
	var cols []string
	switch sortKey {
	case "login":
		cols = []string{"login"}
	case "city":
		cols = []string{"city"}
	case "email":
		cols = []string{"email"}
	case "name":
		cols = []string{"first_name", "last_name"}
	default:
		return []string{"first_name asc, last_name asc, login asc"}
	}
	clauses := make([]string, 0, len(cols)+1)
	for _, c := range cols {
		clauses = append(clauses, c+" "+dir)
	}
	return append(clauses, "login asc")
}

// usersListQuery applies the users-listing search filter and sort to q.
// The `search` term is matched against login, first/last name, email and
// city (issue #107).
func usersListQuery(q *pop.Query, search, sortKey, dir string) *pop.Query {
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where(
			"(login LIKE ? OR first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR city LIKE ?)",
			like, like, like, like, like,
		)
	}
	for _, clause := range userSortClauses(sortKey, dir) {
		q = q.Order(clause)
	}
	return q
}
