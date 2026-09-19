package models

import (
	"strings"
	"testing"
	"time"
)

// TestTreatmentValidateWoundCareSkipsDosage covers issue #96: wound-care
// treatments (Drug == WoundCareDrugName) require no posology.
func TestTreatmentValidateWoundCareSkipsDosage(t *testing.T) {
	base := Treatment{
		Date:       time.Now(),
		AnimalID:   1,
		Timebitmap: Treatement_MORNING,
	}

	// wound care: empty dosage is valid
	wc := base
	wc.Drug = WoundCareDrugName
	wc.Dosage = ""
	if verrs, _ := wc.Validate(nil); verrs.HasAny() {
		t.Fatalf("wound-care treatment should validate without dosage, got %v", verrs)
	}

	// regular drug: empty dosage is invalid
	med := base
	med.Drug = "Peni-Kel"
	med.Dosage = ""
	verrs, _ := med.Validate(nil)
	if !verrs.HasAny() {
		t.Fatal("regular treatment without dosage should not validate")
	}
	if !strings.Contains(verrs.String(), "Dosage") {
		t.Fatalf("expected a Dosage error, got %v", verrs)
	}

	// regular drug with dosage: valid
	med.Dosage = "0.5ml"
	if verrs, _ := med.Validate(nil); verrs.HasAny() {
		t.Fatalf("regular treatment with dosage should validate, got %v", verrs)
	}
}
