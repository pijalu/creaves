package actions

import (
	"fmt"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestListDiscovererRowsFilters covers the optional name/city/postal_code
// filters of the discoverers register (issue #202 bug 4). The dev/test
// database holds real production data, so fixture rows carry marker values
// derived from their UUIDs — guaranteed absent from production rows — and
// every assertion counts only fixture IDs. Fixture discoverers have no linked
// animals, so they are listed for every year selection (a.id IS NULL branch)
// and the year predicate never interferes.
func TestListDiscovererRowsFilters(t *testing.T) {
	requireMySQLTestDB(t)

	ids := []uuid.UUID{uuid.Must(uuid.NewV4()), uuid.Must(uuid.NewV4()), uuid.Must(uuid.NewV4())}
	t.Cleanup(func() {
		for _, id := range ids {
			models.DB.RawQuery("DELETE FROM discoverers WHERE id = ?", id).Exec()
		}
	})

	// marker derives a filter token unique to this run from the fixture UUID.
	marker := func(i int, suffix string) string {
		return ids[i].String()[:8] + suffix
	}
	stmts := []string{
		fmt.Sprintf("INSERT INTO discoverers (id, firstname, lastname, address, postal_code, city, created_at, updated_at) VALUES ('%s', '%s', '%s', '1 rue A', '%s', '%s', NOW(), NOW())",
			ids[0], marker(0, "_first"), marker(0, "_last"), marker(0, "_pc"), marker(0, "_city")),
		fmt.Sprintf("INSERT INTO discoverers (id, firstname, lastname, address, postal_code, city, created_at, updated_at) VALUES ('%s', '%s', '%s', '2 rue B', '%s', '%s', NOW(), NOW())",
			ids[1], marker(1, "_first"), marker(1, "_last"), marker(1, "_pc"), marker(1, "_city")),
		fmt.Sprintf("INSERT INTO discoverers (id, firstname, lastname, address, postal_code, city, created_at, updated_at) VALUES ('%s', '%s', '%s', '3 rue C', '%s', '%s', NOW(), NOW())",
			ids[2], marker(2, "_first"), marker(2, "_last"), marker(2, "_pc"), marker(2, "_city")),
	}
	for _, s := range stmts {
		require.NoError(t, models.DB.RawQuery(s).Exec(), "seed discoverer")
	}

	fixtureRows := func(rows []discovererRow) []discovererRow {
		want := map[uuid.UUID]bool{ids[0]: true, ids[1]: true, ids[2]: true}
		out := []discovererRow{}
		for _, r := range rows {
			if want[r.DiscovererID] {
				out = append(out, r)
			}
		}
		return out
	}

	// Empty filters list every fixture row.
	rows, err := listDiscovererRows(models.DB, "", discovererFilters{})
	require.NoError(t, err)
	rows = fixtureRows(rows)
	require.Len(t, rows, 3)

	// Name substring matches firstname or lastname, case-insensitively.
	rows, err = listDiscovererRows(models.DB, "", discovererFilters{Name: marker(2, "_LAST")})
	require.NoError(t, err)
	rows = fixtureRows(rows)
	require.Len(t, rows, 1)
	require.Equal(t, ids[2], rows[0].DiscovererID)

	rows, err = listDiscovererRows(models.DB, "", discovererFilters{Name: marker(0, "_first")})
	require.NoError(t, err)
	rows = fixtureRows(rows)
	require.Len(t, rows, 1)
	require.Equal(t, ids[0], rows[0].DiscovererID)

	// City substring.
	rows, err = listDiscovererRows(models.DB, "", discovererFilters{City: marker(0, "_ci")})
	require.NoError(t, err)
	rows = fixtureRows(rows)
	require.Len(t, rows, 1)
	require.Equal(t, ids[0], rows[0].DiscovererID)

	// Postal code substring.
	rows, err = listDiscovererRows(models.DB, "", discovererFilters{PostalCode: marker(1, "_p")})
	require.NoError(t, err)
	rows = fixtureRows(rows)
	require.Len(t, rows, 1)
	require.Equal(t, ids[1], rows[0].DiscovererID)

	// No match for any fixture row.
	rows, err = listDiscovererRows(models.DB, "", discovererFilters{Name: "nosuchmarker"})
	require.NoError(t, err)
	require.Empty(t, fixtureRows(rows))

	// Combined filters narrow down (AND): city of fixture 1 does not combine
	// with the postal code of fixture 0.
	rows, err = listDiscovererRows(models.DB, "", discovererFilters{
		City:       marker(1, "_city"),
		PostalCode: marker(0, "_pc"),
	})
	require.NoError(t, err)
	require.Empty(t, fixtureRows(rows))
}

// TestDiscovererFiltersQuery pins the CSV export query-string building: only
// non-empty filters are carried over, values stay URL-encoded.
func TestDiscovererFiltersQuery(t *testing.T) {
	require.Equal(t, "year=2024", discovererFilters{}.query("2024"))
	require.Equal(t,
		"name=Al+Bert&postal_code=6700&year=2024",
		discovererFilters{Name: "Al Bert", PostalCode: "6700"}.query("2024"))
	require.Equal(t, "city=Paris", discovererFilters{City: "Paris"}.query(""))
}
