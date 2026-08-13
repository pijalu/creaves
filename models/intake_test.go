package models

import (
	"testing"
	"time"
)

func TestIntakeDateFormated(t *testing.T) {
	i := Intake{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := i.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}

func TestIntakeString(t *testing.T) {
	i := Intake{}
	if s := i.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestIntakesString(t *testing.T) {
	is := Intakes{{}, {}}
	if s := is.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
