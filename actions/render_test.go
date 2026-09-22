package actions

import "testing"

// TestBoolLabel pins the yes/no label selection used by the user show view
// (issue #202 bug 2): raw Go booleans must never reach the template output.
func TestBoolLabel(t *testing.T) {
	cases := []struct {
		v       bool
		yes, no string
		want    string
	}{
		{true, "Oui", "Non", "Oui"},
		{false, "Oui", "Non", "Non"},
		{true, "Yes", "No", "Yes"},
		{false, "Ja", "Nein", "Nein"},
		{true, "Ja", "Nee", "Ja"},
	}
	for _, c := range cases {
		if got := boolLabel(c.v, c.yes, c.no); got != c.want {
			t.Errorf("boolLabel(%v, %q, %q) = %q, want %q", c.v, c.yes, c.no, got, c.want)
		}
	}
}
