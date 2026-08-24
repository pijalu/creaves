package actions

import (
	"testing"

	"github.com/gobuffalo/nulls"
)

func TestBaseStringNullsString(t *testing.T) {
	valid := nulls.NewString("translated description")
	if got := baseString(valid); got != "translated description" {
		t.Fatalf("expected nullable string value, got %q", got)
	}

	invalid := nulls.String{}
	if got := baseString(invalid); got != "" {
		t.Fatalf("expected invalid nullable string to render empty, got %q", got)
	}
}

func TestBaseStringStringAndFallbacks(t *testing.T) {
	if got := baseString("plain"); got != "plain" {
		t.Fatalf("expected plain string, got %q", got)
	}
}
