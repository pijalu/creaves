package models

import (
	"testing"

	"github.com/gofrs/uuid"
)

func TestAnimaltypeString(t *testing.T) {
	a := Animaltype{Name: "Bird"}
	if s := a.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimaltypesString(t *testing.T) {
	as := Animaltypes{{Name: "a"}, {Name: "b"}}
	if s := as.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestAnimaltypesAsMap(t *testing.T) {
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	at1 := Animaltype{ID: id1, Name: "Bird"}
	at2 := Animaltype{ID: id2, Name: "Mammal"}
	as := Animaltypes{at1, at2}

	m := as.AsMap()
	if len(m) != 2 {
		t.Fatalf("Expected 2 entries in map, got %d", len(m))
	}
	if got, ok := m[id1]; !ok || got.Name != "Bird" {
		t.Errorf("Expected Bird for id1, got %+v (present=%v)", got, ok)
	}
	if got, ok := m[id2]; !ok || got.Name != "Mammal" {
		t.Errorf("Expected Mammal for id2, got %+v (present=%v)", got, ok)
	}
}

func TestAnimaltypesAsMapEmpty(t *testing.T) {
	as := Animaltypes{}
	m := as.AsMap()
	if len(m) != 0 {
		t.Errorf("Expected 0 entries for empty slice, got %d", len(m))
	}
}
