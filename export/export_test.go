package export

import (
	"bytes"
	"encoding/csv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- getQuery on an explicitly constructed Config (pure logic) ---

func TestGetQuery_Found(t *testing.T) {
	cfg := &Config{
		Queries: []Queries{
			{Name: "register", Description: "Registre", Query: "SELECT 1"},
			{Name: "Nombre", Description: "Nombre d'animaux", Query: "SELECT 2"},
		},
	}

	q, err := cfg.getQuery("register")
	assert.NoError(t, err)
	if assert.NotNil(t, q) {
		assert.Equal(t, "Registre", q.Description)
		assert.Equal(t, "SELECT 1", q.Query)
	}
}

func TestGetQuery_NotFound(t *testing.T) {
	cfg := &Config{
		Queries: []Queries{
			{Name: "register"},
		},
	}

	q, err := cfg.getQuery("does_not_exist")
	assert.Error(t, err)
	assert.Nil(t, q)
	assert.Contains(t, err.Error(), "does_not_exist")
}

func TestGetQuery_CaseInsensitive(t *testing.T) {
	cfg := &Config{
		Queries: []Queries{
			{Name: "Register"},
		},
	}

	// Lookup is case-insensitive via ToLower on both sides.
	for _, id := range []string{"register", "REGISTER", "Register"} {
		q, err := cfg.getQuery(id)
		assert.NoError(t, err, "id=%s", id)
		if assert.NotNil(t, q, "id=%s", id) {
			assert.Equal(t, "Register", q.Name, "id=%s", id)
		}
	}
}

func TestGetQuery_EmptyConfig(t *testing.T) {
	cfg := &Config{}

	q, err := cfg.getQuery("anything")
	assert.Error(t, err)
	assert.Nil(t, q)
}

// --- package-level config loaded from embedded YAML ---

func TestGetConfig_LoadsEmbeddedYAML(t *testing.T) {
	// getConfig() decodes the embedded config.yaml. A non-panic return with a
	// populated Queries slice proves the embed + YAML path works.
	cfg := getConfig()
	if assert.NotNil(t, cfg) {
		assert.NotEmpty(t, cfg.Queries)
	}
}

func TestGetQueries_ReturnsLoadedQueries(t *testing.T) {
	// The package-level `config` var is initialised at load via getConfig().
	queries := GetQueries()
	assert.NotEmpty(t, queries)

	// The embedded config defines a well-known set; assert a representative subset.
	names := make(map[string]bool, len(queries))
	for _, q := range queries {
		names[q.Name] = true
	}
	assert.True(t, names["register"], "expected register query in config")
	assert.True(t, names["detail_register"], "expected detail_register query in config")
}

// --- writeCSV encoding ---

// TestWriteCSV_UTF8BOM proves the CSV output starts with a UTF-8 BOM so Excel
// detects UTF-8 instead of the local ANSI codepage (accented characters would
// render as mojibake without it).
func TestWriteCSV_UTF8BOM(t *testing.T) {
	var buf bytes.Buffer

	err := writeCSV(&buf, []string{"année", "Espèce"}, [][]string{{"2024", "Hérisson"}})
	require.NoError(t, err)

	bom := []byte{0xEF, 0xBB, 0xBF}
	if !bytes.HasPrefix(buf.Bytes(), bom) {
		t.Fatalf("output must start with UTF-8 BOM EF BB BF, got % x", buf.Bytes()[:3])
	}
}

// TestWriteCSV_AccentsRoundTrip proves accented values (animal ages like
// "Juvénile", causes like "Mort à l'arrivée") survive a write/read cycle
// unchanged: the bytes on the wire are valid UTF-8.
func TestWriteCSV_AccentsRoundTrip(t *testing.T) {
	var buf bytes.Buffer

	cols := []string{"âge", "cause"}
	rows := [][]string{
		{"Juvénile", "Mort à l'arrivée avant l'encodage"},
		{"Adulte", "Relâché"},
	}
	require.NoError(t, writeCSV(&buf, cols, rows))

	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(buf.String(), "\ufeff")))
	recs, err := r.ReadAll()
	require.NoError(t, err)
	require.Len(t, recs, len(rows)+1)
	assert.Equal(t, cols, recs[0])
	for i, row := range rows {
		assert.Equal(t, row, recs[i+1])
	}
}

func TestGetQuery_OnLoadedConfig(t *testing.T) {
	// getQuery against the real loaded config should resolve known ids.
	q, err := config.getQuery("register")
	assert.NoError(t, err)
	if assert.NotNil(t, q) {
		assert.Equal(t, "register", q.Name)
		assert.Equal(t, "Registre", q.Description)
	}
}
