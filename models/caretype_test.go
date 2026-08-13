package models

import (
	"testing"
)

func TestCaretypeString(t *testing.T) {
	c := Caretype{Name: "Feeding"}
	if s := c.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestCaretypesString(t *testing.T) {
	cs := Caretypes{{Name: "a"}, {Name: "b"}}
	if s := cs.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
