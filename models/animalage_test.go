package models

import (
	"testing"
)

func TestAnimalageString(t *testing.T) {
	a := Animalage{Name: "Juvenile"}
	if s := a.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimalagesString(t *testing.T) {
	as := Animalages{{Name: "a"}, {Name: "b"}}
	if s := as.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
