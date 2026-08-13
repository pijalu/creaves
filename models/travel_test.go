package models

import (
	"testing"
	"time"
)

func TestTravelString(t *testing.T) {
	tr := Travel{Distance: 100}
	if s := tr.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTravelsString(t *testing.T) {
	ts := Travels{{Distance: 1}, {Distance: 2}}
	if s := ts.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestTravelDateFormated(t *testing.T) {
	tr := &Travel{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := tr.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}
