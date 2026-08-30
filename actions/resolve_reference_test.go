package actions

import (
	"testing"
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
