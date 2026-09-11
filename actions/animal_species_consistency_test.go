package actions

import (
	"testing"

	"creaves/models"
	"github.com/gofrs/uuid"
)

func TestCompleteAndValidateSpeciesType(t *testing.T) {
	tx := searchTestDB(t)
	var column string
	if err := tx.RawQuery("SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'species' AND column_name = 'animaltype_id'").First(&column); err != nil || column == "" {
		t.Skip("species animaltype_id migration not applied to test database")
	}
	marker := uuid.Must(uuid.NewV4()).String()
	atID := uuid.Must(uuid.NewV4())
	otherID := uuid.Must(uuid.NewV4())
	spID := "consistency-" + marker
	sp := models.Species{ID: spID, Species: "Consistency species " + marker, Class: "class", Order: "order", Family: "family", CreavesSpecies: "CONSISTENCY-" + marker, SubsideGroup: "group", AgwGroup: "agw", NativeStatus: "native"}
	at := models.Animaltype{ID: atID, Name: "Consistency type " + marker}
	other := models.Animaltype{ID: otherID, Name: "Other type " + marker}
	if err := tx.Create(&at); err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&other); err != nil {
		t.Fatal(err)
	}
	sp.AnimaltypeID = atID
	if err := tx.Create(&sp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE id = ?", spID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE id IN (?, ?)", atID, otherID).Exec()
	})

	inferred := &models.Animal{Species: sp.CreavesSpecies}
	if err := completeAndValidateSpeciesType(tx, inferred); err != nil {
		t.Fatal(err)
	}
	if inferred.AnimaltypeID != atID {
		t.Fatalf("inferred type = %s, want %s", inferred.AnimaltypeID, atID)
	}

	matching := &models.Animal{Species: sp.CreavesSpecies, AnimaltypeID: atID}
	if err := completeAndValidateSpeciesType(tx, matching); err != nil {
		t.Fatal(err)
	}

	mismatching := &models.Animal{Species: sp.CreavesSpecies, AnimaltypeID: otherID}
	if err := completeAndValidateSpeciesType(tx, mismatching); err == nil {
		t.Fatal("expected mismatch validation error")
	}
}
