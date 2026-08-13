package models

import (
	"testing"
)

func TestTraveltypeString(t *testing.T) {
	tt := Traveltype{Name: "Transport"}
	if s := tt.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTraveltypesString(t *testing.T) {
	ts := Traveltypes{{Name: "a"}, {Name: "b"}}
	if s := ts.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
