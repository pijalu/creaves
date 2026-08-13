package models

import (
	"testing"
	"time"
)

func TestLogentryString(t *testing.T) {
	l := Logentry{Description: "something happened"}
	if s := l.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestLogentriesString(t *testing.T) {
	ls := Logentries{{Description: "a"}, {Description: "b"}}
	if s := ls.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestLogentryCreatedAtFormated(t *testing.T) {
	l := Logentry{CreatedAt: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := l.CreatedAtFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("CreatedAtFormated() = %q, want %q", got, want)
	}
}

func TestLogentryUpdatedAtFormated(t *testing.T) {
	l := Logentry{UpdatedAt: time.Date(2023, 6, 15, 10, 30, 0, 0, time.UTC)}
	got := l.UpdatedAtFormated()
	want := "2023/06/15 10:30"
	if got != want {
		t.Errorf("UpdatedAtFormated() = %q, want %q", got, want)
	}
}
