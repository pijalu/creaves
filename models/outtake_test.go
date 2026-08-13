package models

import (
	"testing"
	"time"

	"github.com/gofrs/uuid"
)

func TestOuttakeIsSelectedTrue(t *testing.T) {
	id := uuid.Must(uuid.NewV4())
	o := Outtake{TypeID: id}
	if !o.IsSelected(id) {
		t.Error("Expected IsSelected to return true for matching UUID")
	}
}

func TestOuttakeIsSelectedFalse(t *testing.T) {
	id1 := uuid.Must(uuid.NewV4())
	id2 := uuid.Must(uuid.NewV4())
	o := Outtake{TypeID: id1}
	if o.IsSelected(id2) {
		t.Error("Expected IsSelected to return false for non-matching UUID")
	}
}

func TestOuttakeDateFormated(t *testing.T) {
	o := Outtake{Date: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := o.DateFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("DateFormated() = %q, want %q", got, want)
	}
}

func TestOuttakeString(t *testing.T) {
	o := Outtake{}
	if s := o.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestOuttakesString(t *testing.T) {
	os := Outtakes{{}, {}}
	if s := os.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}
