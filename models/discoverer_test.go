package models

import (
	"testing"
)

func TestDiscovererString(t *testing.T) {
	d := Discoverer{}
	if s := d.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDiscoverersString(t *testing.T) {
	ds := Discoverers{{}, {}}
	if s := ds.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
