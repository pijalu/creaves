package models

import (
	"testing"
)

func TestOuttaketypeString(t *testing.T) {
	o := Outtaketype{Name: "Released"}
	if s := o.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestOuttaketypesString(t *testing.T) {
	os := Outtaketypes{{Name: "a"}, {Name: "b"}}
	if s := os.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
