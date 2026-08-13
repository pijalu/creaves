package models

import (
	"testing"
)

func TestSpeciesString(t *testing.T) {
	s := Species{ID: "SP1", Species: "Heron", Class: "Aves"}
	if str := s.String(); str == "" {
		t.Error("Expected non-empty string representation")
	}
}
