package models

import (
	"testing"
	"time"
)

func TestCareWithAnimalNumberYearNumberFormatted(t *testing.T) {
	c := CareWithAnimalNumber{Year: 2023, YearNumber: 42}
	got := c.YearNumberFormatted()
	want := "42/23"
	if got != want {
		t.Errorf("YearNumberFormatted() = %q, want %q", got, want)
	}
}

func TestCareString(t *testing.T) {
	c := Care{}
	if s := c.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestCareDateFormated(t *testing.T) {
	c := Care{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := c.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}

func TestCaresString(t *testing.T) {
	cs := Cares{{}, {}}
	if s := cs.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
