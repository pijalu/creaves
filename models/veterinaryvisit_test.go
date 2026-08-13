package models

import (
	"testing"
	"time"
)

func TestVeterinaryvisitString(t *testing.T) {
	v := Veterinaryvisit{Veterinary: "Dr. Vet"}
	if s := v.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestVeterinaryvisitsString(t *testing.T) {
	vs := Veterinaryvisits{{Veterinary: "A"}, {Veterinary: "B"}}
	if s := vs.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestVeterinaryvisitDateFormated(t *testing.T) {
	v := Veterinaryvisit{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := v.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}
