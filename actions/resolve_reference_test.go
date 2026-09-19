package actions

import (
	"fmt"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// TestResolveReferenceInputPassthrough covers the no-DB guard paths: nil tx,
// base language (fr), empty input and unknown tables all pass the input
// through unchanged. DB-backed canonical/translation resolution is exercised
// by the E2E suite (needs MySQL).
func TestResolveReferenceInputPassthrough(t *testing.T) {
	cases := []struct {
		name  string
		tx    interface{}
		lang  string
		table string
		input string
		want  string
	}{
		{"nil tx", nil, "en-US", "species", "Hérisson", "Hérisson"},
		{"base lang", nil, "", "species", "Hedgehog", "Hedgehog"},
		{"empty input", nil, "en-US", "species", "", ""},
		{"unknown table", nil, "en-US", "discoverers", "Foo", "Foo"},
		{"whitespace trimmed", nil, "", "species", "  Hérisson  ", "Hérisson"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveReferenceInputTx(nil, tc.lang, tc.table, tc.input)
			if got != tc.want {
				t.Fatalf("resolveReferenceInputTx(nil, %q, %q, %q) = %q, want %q", tc.lang, tc.table, tc.input, got, tc.want)
			}
		})
	}
}

// TestResolveReferenceExistsKeepsPopColumnCacheIntact pins the pop column-cache
// regression: Query.Exists must be given a model, not a bare table name. With
// a string, pop's internal reflection panics and silently caches
// "SELECT <table>.*" as the column list for the whole process; MySQL then
// returns reference columns in their defined case (species.ID) and every
// later model scan for that table fails with "missing destination name ID"
// (observed as /species/ 500s after an animal save).
func TestResolveReferenceExistsKeepsPopColumnCacheIntact(t *testing.T) {
	requireMySQLTestDB(t)
	tx := models.DB

	// The resolution itself (canonical input → passthrough).
	marker := fmt.Sprintf("cacheprobe-%s", uuid.Must(uuid.NewV4()).String()[:8])
	sp := models.Species{
		ID:             "crp-" + marker,
		Species:        "CacheProbe " + marker,
		CreavesSpecies: "CACHEPROBE-" + marker,
		Class:          "class",
		Order:          "order",
		Family:         "family",
		NativeStatus:   "NS1",
	}
	require.NoError(t, tx.Create(&sp))
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE id = ?", sp.ID).Exec()
	})

	require.Equal(t, sp.CreavesSpecies, resolveReferenceInputTx(tx, "en-US", "species", sp.CreavesSpecies))

	// Whatever the resolution did, a plain species scan must still work —
	// this is exactly what breaks when the column cache got poisoned.
	probe := &[]models.Species{}
	require.NoError(t, tx.Where("creaves_species = ?", sp.CreavesSpecies).All(probe))
	require.Len(t, *probe, 1)
	require.Equal(t, sp.ID, (*probe)[0].ID)
}
