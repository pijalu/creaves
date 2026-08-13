package models

import (
	"math"
	"testing"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

func TestDosageString(t *testing.T) {
	d := Dosage{Description: nulls.NewString("test")}
	if s := d.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDosagesString(t *testing.T) {
	ds := Dosages{{Description: nulls.NewString("a")}}
	if s := ds.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDosagePerKilo(t *testing.T) {
	// 0.005 per gram → 5.0 per kilo.
	d := Dosage{DosagePerGrams: nulls.NewFloat64(0.005)}
	got := d.PerKilo()
	if !got.Valid {
		t.Fatal("Expected valid PerKilo")
	}
	if math.Abs(got.Float64-5.0) > 1e-9 {
		t.Errorf("PerKilo() = %v, want 5.0", got.Float64)
	}
}

func TestDosagePerKiloRounding(t *testing.T) {
	// 0.00012345 per gram → 0.12345 per kilo, formatted to 4 decimals → 0.1234
	// (float64 repr of the product lands below the rounding midpoint).
	d := Dosage{DosagePerGrams: nulls.NewFloat64(0.00012345)}
	got := d.PerKilo()
	if !got.Valid {
		t.Fatal("Expected valid PerKilo")
	}
	if math.Abs(got.Float64-0.1234) > 1e-9 {
		t.Errorf("PerKilo() = %v, want 0.1234", got.Float64)
	}
}

func TestDosagePerKiloInvalid(t *testing.T) {
	d := Dosage{DosagePerGrams: nulls.Float64{}}
	got := d.PerKilo()
	if got.Valid {
		t.Errorf("Expected invalid PerKilo when DosagePerGrams invalid, got %+v", got)
	}
}

func TestDrugString(t *testing.T) {
	d := Drug{Name: "Aspirin"}
	if s := d.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDrugsString(t *testing.T) {
	ds := Drugs{{Name: "A"}, {Name: "B"}}
	if s := ds.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestDrugDosagePerAnimalTypeID(t *testing.T) {
	at1 := uuid.Must(uuid.NewV4())
	at2 := uuid.Must(uuid.NewV4())
	d := Drug{
		Name: "Drug X",
		Dosages: []Dosage{
			{AnimaltypeID: at1, Description: nulls.NewString("for type1")},
			{AnimaltypeID: at2, Description: nulls.NewString("for type2")},
		},
	}
	m := d.DosagePerAnimalTypeID()
	if len(m) != 2 {
		t.Fatalf("Expected 2 dosages in map, got %d", len(m))
	}
	if _, ok := m[at1]; !ok {
		t.Error("Expected dosage for animal type 1")
	}
	if _, ok := m[at2]; !ok {
		t.Error("Expected dosage for animal type 2")
	}
}

func TestDrugDosagePerAnimalTypeIDEmpty(t *testing.T) {
	d := Drug{Name: "Drug X"}
	m := d.DosagePerAnimalTypeID()
	if len(m) != 0 {
		t.Errorf("Expected 0 dosages for drug with no dosages, got %d", len(m))
	}
}
