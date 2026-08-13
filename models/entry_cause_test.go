package models

import (
	"strings"
	"testing"
)

func TestEntryCauseString(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "Fall"}
	if s := e.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestEntryCausesString(t *testing.T) {
	es := EntryCauses{{ID: "1"}, {ID: "2"}}
	if s := es.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestEntryCauseFmtWithoutIDDifferent(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "Fall", Detail: "From nest"}
	got := e.Fmt(false)
	want := "Fall ⇨ From nest"
	if got != want {
		t.Errorf("Fmt(false) = %q, want %q", got, want)
	}
}

func TestEntryCauseFmtWithIDDifferent(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "Fall", Detail: "From nest"}
	got := e.Fmt(true)
	want := "EC1 - Fall ⇨ From nest"
	if got != want {
		t.Errorf("Fmt(true) = %q, want %q", got, want)
	}
}

func TestEntryCauseFmtEqualCauseDetail(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "Fall", Detail: "Fall"}
	got := e.Fmt(false)
	want := "Fall"
	if got != want {
		t.Errorf("Fmt(false) with equal cause/detail = %q, want %q", got, want)
	}
}

func TestEntryCauseFmtWithIDEqualCauseDetail(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "Fall", Detail: "Fall"}
	got := e.Fmt(true)
	want := "EC1 - Fall"
	if got != want {
		t.Errorf("Fmt(true) with equal cause/detail = %q, want %q", got, want)
	}
}

func TestEntryCauseFmtContainsArrow(t *testing.T) {
	e := EntryCause{ID: "EC1", Cause: "A", Detail: "B"}
	got := e.Fmt(false)
	if !strings.Contains(got, "⇨") {
		t.Errorf("Expected arrow in formatted output, got %q", got)
	}
}
