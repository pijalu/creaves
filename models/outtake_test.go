package models

import (
	"strings"
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

func TestOuttakeComputeStayDuration(t *testing.T) {
	intake := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	cases := []struct {
		name    string
		outtake time.Time
		want    int
	}{
		{"exactly 24h", intake.Add(24 * time.Hour), 24},
		{"exactly 48h", intake.Add(48 * time.Hour), 48},
		{"partial hour floors down", intake.Add(90 * time.Minute), 1},
		{"same instant", intake, 0},
		{"negative clamped to 0", intake.Add(-2 * time.Hour), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := Outtake{Date: tc.outtake}
			o.ComputeStayDuration(intake)
			if !o.StayDuration.Valid {
				t.Fatalf("expected StayDuration set, got NULL")
			}
			if o.StayDuration.Int != tc.want {
				t.Errorf("StayDuration = %d, want %d", o.StayDuration.Int, tc.want)
			}
		})
	}

	// Missing dates leave the value untouched.
	o := Outtake{Date: intake}
	o.ComputeStayDuration(time.Time{})
	if o.StayDuration.Valid {
		t.Errorf("zero intake must leave StayDuration untouched")
	}
	o = Outtake{}
	o.ComputeStayDuration(intake)
	if o.StayDuration.Valid {
		t.Errorf("zero outtake date must leave StayDuration untouched")
	}
}

// TestDeaccent: the indigénat guard must match accented spellings.
func TestDeaccent(t *testing.T) {
	cases := map[string]string{
		"ID d'indigénat":   "ID d'indigenat",
		"Mort à l'arrivée": "Mort a l'arrivee",
		"Relacher":         "Relacher",
		"":                 "",
		"déjà vu Noël":     "deja vu Noel",
	}
	for in, want := range cases {
		if got := deaccent(in); got != want {
			t.Errorf("deaccent(%q) = %q, want %q", in, got, want)
		}
	}
	if !strings.Contains(strings.ToLower(deaccent("ID d'indigénat")), "indigen") {
		t.Error("accented indigénat must match the indigen guard")
	}
}

// TestOuttakeValidateRejectsFutureDate: an outtake date in the future must
// fail validation (issue #175). TypeID is Nil so no DB connection is used.
// Dates are built in the form/storage frame (naive local wall clock as UTC,
// see FormWallClockNow): a real future wall clock must be rejected, a past
// wall clock accepted.
func TestOuttakeValidateRejectsFutureDate(t *testing.T) {
	future := FormWallClockNow().Add(2 * time.Hour)
	o := Outtake{Date: future}
	errs, err := o.Validate(nil)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if !errs.HasAny() {
		t.Fatal("expected validation errors for a future date")
	}
	if _, ok := errs.Errors["Date"]; !ok {
		t.Errorf("expected a Date error, got %v", errs.Errors)
	}

	past := Outtake{Date: time.Now().Add(-24 * time.Hour)}
	errs, err = past.Validate(nil)
	if err != nil {
		t.Fatalf("Validate returned error: %v", err)
	}
	if errs.HasAny() {
		t.Errorf("past date must validate, got %v", errs.Errors)
	}
}
