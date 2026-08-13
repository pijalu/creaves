package models

import (
	"testing"
	"time"
)

func TestDiscoveryDateFormated(t *testing.T) {
	d := Discovery{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := d.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}

func TestDiscoveryString(t *testing.T) {
	d := Discovery{}
	if s := d.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDiscoveriesString(t *testing.T) {
	ds := Discoveries{{}, {}}
	if s := ds.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
