package actions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// BUG-R5: user-controlled redirect targets must be local paths only.
func TestSafeRedirectTarget(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"", "/"},                      // empty → root
		{"/animals/42", "/animals/42"}, // local path ok
		{"/animals/42#nav-media", "/animals/42#nav-media"},
		{"https://evil.example", "/"},  // absolute URL blocked
		{"http://evil.example/x", "/"}, //
		{"//evil.example", "/"},        // scheme-relative blocked
		{"//evil.example/path", "/"},   //
		{"/\\evil.example", "/"},       // backslash trick blocked
		{"javascript:alert(1)", "/"},   // scheme blocked
		{"evil.example", "/"},          // bare host blocked
		{"/", "/"},                     // root ok (caller treats as no-back)
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, safeRedirectTarget(tc.in), "input %q", tc.in)
	}
}
