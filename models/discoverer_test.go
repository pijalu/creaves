package models

import (
	"testing"

	"github.com/gobuffalo/nulls"
)

// TestNormalizeBelgianPhone (#208.10): phone numbers must be unified to the
// local Belgian format 0470112233 in reception/new and /guest/ paths.
func TestNormalizeBelgianPhone(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"+32470112233", "0470112233"},
		{"+32 470 11 22 33", "0470112233"},
		{"+32/470.11.22.33", "0470112233"},
		{"0032470112233", "0470112233"},
		{"0470112233", "0470112233"},   // already local: unchanged
		{"0470 11 22 33", "0470 11 22 33"}, // local with spaces: unchanged
		{"+33612345678", "+33612345678"},   // French: unchanged
		{"", ""},
		{"+32", "+32"}, // no digits left: unchanged
	}
	for _, tc := range cases {
		if got := normalizeBelgianPhone(tc.in); got != tc.want {
			t.Errorf("normalizeBelgianPhone(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestNormalizeLocalPhoneNull(t *testing.T) {
	if got := normalizeLocalPhone(nulls.String{}); got.Valid {
		t.Errorf("invalid nulls.String must stay invalid, got %+v", got)
	}
	if got := normalizeLocalPhone(nulls.NewString("+32470112233")); !got.Valid || got.String != "0470112233" {
		t.Errorf("got %+v, want valid 0470112233", got)
	}
}
