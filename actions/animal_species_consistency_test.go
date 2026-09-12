package actions

import (
	"testing"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
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

// insertUnmappedSpecies creates a species row with a NULL animaltype_id.
func insertUnmappedSpecies(t *testing.T, tx *pop.Connection, id, creavesSpecies string) {
	t.Helper()
	if err := tx.RawQuery(
		"INSERT INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, game, agw_group, native_status, huntable, animaltype_id) VALUES (?, ?, 'class', 'family', ?, 'group', NOW(), NOW(), 'order', 0, 'agw', 'native', 0, NULL)",
		id, "Unmapped species "+id, creavesSpecies,
	).Exec(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE ID = ?", id).Exec()
	})
}

func TestCompleteAndValidateSpeciesTypeUnmappedSoftBlock(t *testing.T) {
	tx := searchTestDB(t)
	var column string
	if err := tx.RawQuery("SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'species' AND column_name = 'animaltype_id'").First(&column); err != nil || column == "" {
		t.Skip("species animaltype_id migration not applied to test database")
	}
	marker := uuid.Must(uuid.NewV4()).String()
	spID := "unmapped-" + marker
	creavesSpecies := "UNMAPPED-" + marker
	insertUnmappedSpecies(t, tx, spID, creavesSpecies)

	// Blank type + zero mappings: accepted with a warning, type stays blank.
	blank := &models.Animal{Species: creavesSpecies}
	if err := completeAndValidateSpeciesType(tx, blank); err != nil {
		t.Fatalf("unmapped species with blank type should be accepted, got %v", err)
	}
	if blank.AnimaltypeID != uuid.Nil {
		t.Fatalf("unmapped species should keep blank type, got %s", blank.AnimaltypeID)
	}

	// Submitted type + zero mappings: accepted (soft-block), type kept.
	atID := uuid.Must(uuid.NewV4())
	submitted := &models.Animal{Species: creavesSpecies, AnimaltypeID: atID}
	if err := completeAndValidateSpeciesType(tx, submitted); err != nil {
		t.Fatalf("unmapped species with submitted type should be accepted, got %v", err)
	}
	if submitted.AnimaltypeID != atID {
		t.Fatalf("submitted type should be kept, got %s", submitted.AnimaltypeID)
	}

	// No mapping exists, so the show-page mismatch banner must stay off.
	if animalSpeciesTypeMismatch(tx, submitted) {
		t.Fatal("unmapped species should not be flagged as type mismatch")
	}
}

func TestCompleteAndValidateSpeciesTypeMultipleMappingsRejected(t *testing.T) {
	tx := searchTestDB(t)
	var column string
	if err := tx.RawQuery("SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'species' AND column_name = 'animaltype_id'").First(&column); err != nil || column == "" {
		t.Skip("species animaltype_id migration not applied to test database")
	}
	marker := uuid.Must(uuid.NewV4()).String()
	atID := uuid.Must(uuid.NewV4())
	otherID := uuid.Must(uuid.NewV4())
	creavesSpecies := "MULTI-" + marker
	at := models.Animaltype{ID: atID, Name: "Multi type " + marker}
	other := models.Animaltype{ID: otherID, Name: "Multi other type " + marker}
	if err := tx.Create(&at); err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&other); err != nil {
		t.Fatal(err)
	}
	sp1 := models.Species{ID: "multi-1-" + marker, Species: "Multi species " + marker, Class: "class", Order: "order", Family: "family", CreavesSpecies: creavesSpecies, SubsideGroup: "group", AgwGroup: "agw", NativeStatus: "native", AnimaltypeID: atID}
	sp2 := models.Species{ID: "multi-2-" + marker, Species: "Multi species " + marker, Class: "class", Order: "order", Family: "family", CreavesSpecies: creavesSpecies, SubsideGroup: "group", AgwGroup: "agw", NativeStatus: "native", AnimaltypeID: otherID}
	if err := tx.Create(&sp1); err != nil {
		t.Fatal(err)
	}
	if err := tx.Create(&sp2); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		tx.RawQuery("DELETE FROM species WHERE ID IN (?, ?)", sp1.ID, sp2.ID).Exec()
		tx.RawQuery("DELETE FROM animaltypes WHERE ID IN (?, ?)", atID, otherID).Exec()
	})

	// Blank type + several mappings is ambiguous: still rejected.
	blank := &models.Animal{Species: creavesSpecies}
	if err := completeAndValidateSpeciesType(tx, blank); err == nil {
		t.Fatal("blank type with multiple mappings should be rejected")
	}
}
