package models

import (
	"testing"
)

func TestZoneKeyZoneEscape(t *testing.T) {
	z := ZoneKey{Zone: "Zone A/B"}
	got := z.ZoneEscape()
	want := "Zone%20A%2FB"
	if got != want {
		t.Errorf("ZoneEscape() = %q, want %q", got, want)
	}
}

func TestZoneKeyZoneEscapePlain(t *testing.T) {
	z := ZoneKey{Zone: "Aviary"}
	got := z.ZoneEscape()
	// A plain name with no special chars should be unchanged.
	if got != "Aviary" {
		t.Errorf("ZoneEscape() = %q, want %q", got, "Aviary")
	}
}

func TestZoneString(t *testing.T) {
	z := Zone{Zone: "Aviary", Type: "Indoor"}
	if s := z.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestZonesString(t *testing.T) {
	zs := Zones{{Zone: "A"}, {Zone: "B"}}
	if s := zs.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
