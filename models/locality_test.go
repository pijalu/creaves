package models

import (
	"testing"
)

func TestLocalityString(t *testing.T) {
	l := Locality{ID: "L1", Country: "BE", Locality: "Brussels"}
	if s := l.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestLocalitiesString(t *testing.T) {
	ls := Localities{{ID: "1"}, {ID: "2"}}
	if s := ls.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
