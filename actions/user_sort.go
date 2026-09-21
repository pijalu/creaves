package actions

import (
	"strings"

	"creaves/models"

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

// usersListFilters applies the users-listing role and status filters
// (issue #199-14). `role` is one of the single "Account role" selector values
// (admin/maintainer/shared/lecteur/scientifique/spw/user); unknown values are
// ignored. `status` is "active" (approved) or "pending" (awaiting approval).
func usersListFilters(q *pop.Query, role, status string) *pop.Query {
	switch role {
	case "admin":
		q = q.Where("admin = ?", true)
	case "maintainer":
		q = q.Where("maintainer = ?", true)
	case "shared":
		q = q.Where("shared = ?", true)
	case "user":
		q = q.Where("admin = ? AND maintainer = ? AND shared = ? AND (role IS NULL OR role = '')",
			false, false, false)
	case models.UserRoleReader, models.UserRoleScientist, models.UserRoleSPW:
		q = q.Where("role = ?", role)
	}
	switch status {
	case "active":
		q = q.Where("approved = ?", true)
	case "pending":
		q = q.Where("approved = ?", false)
	}
	return q
}

// usersListQuery applies the users-listing search filter and sort to q.
// The `search` term is matched against login, first/last name, email and
// city (issue #107); role/status filter the account type (issue #199-14).
func usersListQuery(q *pop.Query, search, role, status, sortKey, dir string) *pop.Query {
	if s := strings.TrimSpace(search); s != "" {
		like := "%" + s + "%"
		q = q.Where(
			"(login LIKE ? OR first_name LIKE ? OR last_name LIKE ? OR email LIKE ? OR city LIKE ?)",
			like, like, like, like, like,
		)
	}
	q = usersListFilters(q, role, status)
	for _, clause := range userSortClauses(sortKey, dir) {
		q = q.Order(clause)
	}
	return q
}
