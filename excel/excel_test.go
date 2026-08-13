package excel

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSheetPosition(t *testing.T) {
	cases := []struct {
		name     string
		line     int
		col      int
		expected string
	}{
		// Single-letter columns (1-26 -> A-Z)
		{"first cell", 1, 1, "A1"},
		{"second column", 2, 2, "B2"},
		{"column Z boundary", 1, 26, "Z1"},
		{"middle single letter", 10, 13, "M10"},

		// Two-letter columns (27+)
		{"column AA", 1, 27, "AA1"},
		{"column AB", 1, 28, "AB1"},
		{"column AZ boundary", 1, 52, "AZ1"},
		{"column BA", 1, 53, "BA1"},
		{"column BD", 1, 56, "BD1"},
		{"column ZZ boundary", 1, 702, "ZZ1"},

		// Three-letter columns (703+)
		{"column AAA", 1, 703, "AAA1"},

		// Row/line values exercised independently of column
		{"large line", 999, 1, "A999"},
		{"line 132 col 3", 132, 3, "C132"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, sheetPosition(tc.line, tc.col))
		})
	}
}

func TestSheetPosition_ColZero(t *testing.T) {
	// col == 0 never enters the loop, so the column prefix is empty and the
	// result is just the line number rendered as a string.
	assert.Equal(t, "1", sheetPosition(1, 0))
	assert.Equal(t, "5", sheetPosition(5, 0))
}

// --- getQuery on an explicitly constructed Config (pure logic) ---

func TestGetQuery_Found(t *testing.T) {
	cfg := &Config{
		Queries: []Queries{
			{Name: "registre_detail", Template: "registre.xlsx", Sheet: "animals", Query: "SELECT 1"},
			{Name: "stat_communes", Template: "stats_communes.xlsx", Sheet: "bdd", Query: "SELECT 2"},
		},
	}

	q, err := cfg.getQuery("stat_communes")
	assert.NoError(t, err)
	if assert.NotNil(t, q) {
		assert.Equal(t, "stats_communes.xlsx", q.Template)
		assert.Equal(t, "bdd", q.Sheet)
		assert.Equal(t, "SELECT 2", q.Query)
	}
}

func TestGetQuery_NotFound(t *testing.T) {
	cfg := &Config{
		Queries: []Queries{
			{Name: "registre_detail"},
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
			{Name: "Registre_Detail"},
		},
	}

	// Lookup is case-insensitive via ToLower on both sides.
	for _, id := range []string{"registre_detail", "REGISTRE_DETAIL", "Registre_Detail"} {
		q, err := cfg.getQuery(id)
		assert.NoError(t, err, "id=%s", id)
		if assert.NotNil(t, q, "id=%s", id) {
			assert.Equal(t, "Registre_Detail", q.Name, "id=%s", id)
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
	// getConfig() decodes the embedded config/config.yaml. A non-panic return
	// with a populated Queries slice proves the embed + YAML path works.
	cfg := getConfig()
	if assert.NotNil(t, cfg) {
		assert.NotEmpty(t, cfg.Queries)
	}
}

func TestGetQueries_ReturnsLoadedQueries(t *testing.T) {
	// The package-level `config` var is initialised at load via getConfig().
	queries := GetQueries()
	assert.NotEmpty(t, queries)

	// The embedded config defines exactly these two queries.
	names := make(map[string]bool, len(queries))
	for _, q := range queries {
		names[q.Name] = true
	}
	assert.True(t, names["registre_detail"], "expected registre_detail query in config")
	assert.True(t, names["stat_communes"], "expected stat_communes query in config")
}

func TestGetQuery_OnLoadedConfig(t *testing.T) {
	// getQuery against the real loaded config should resolve known ids.
	q, err := config.getQuery("registre_detail")
	assert.NoError(t, err)
	if assert.NotNil(t, q) {
		assert.Equal(t, "registre_detail", q.Name)
		assert.Equal(t, "animals", q.Sheet)
		assert.Equal(t, "registre.xlsx", q.Template)
	}
}
