package actions

import (
	"testing"
	"time"

	"creaves/models"
	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

func TestUnmappedSpeciesDiagnostics(t *testing.T) {
	tx := searchTestDB(t)
	var column string
	if err := tx.RawQuery("SELECT column_name FROM information_schema.columns WHERE table_schema = DATABASE() AND table_name = 'species' AND column_name = 'animaltype_id'").First(&column); err != nil || column == "" {
		t.Skip("species animaltype_id migration not applied to test database")
	}
	marker := uuid.Must(uuid.NewV4()).String()
	creavesSpecies := "DIAG-UNMAPPED-" + marker
	insertUnmappedSpecies(t, tx, "diag-"+marker, creavesSpecies)

	// Animal referencing the unmapped species (submitted type is kept by the
	// soft-block path, so any type id works here).
	at := models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "Diag type " + marker}
	if err := tx.Create(&at); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM animaltypes WHERE id = ?", at.ID).Exec() })
	aa := models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "Diag age " + marker}
	if err := tx.Create(&aa); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM animalages WHERE id = ?", aa.ID).Exec() })
	disc := models.Discoverer{ID: uuid.Must(uuid.NewV4()), Lastname: nulls.NewString("Diag-" + marker)}
	if err := tx.Create(&disc); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM discoverers WHERE id = ?", disc.ID).Exec() })
	d := models.Discovery{ID: uuid.Must(uuid.NewV4()), Date: time.Now(), DiscovererID: disc.ID, EntryCauseID: "DIAG-" + marker}
	if err := tx.Create(&d); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM discoveries WHERE id = ?", d.ID).Exec() })
	in := models.Intake{ID: uuid.Must(uuid.NewV4()), Date: time.Now()}
	if err := tx.Create(&in); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM intakes WHERE id = ?", in.ID).Exec() })
	a := models.Animal{
		Species:      creavesSpecies,
		AnimaltypeID: at.ID,
		AnimalageID:  aa.ID,
		DiscoveryID:  d.ID,
		IntakeID:     in.ID,
		IntakeDate:   time.Now(),
	}
	if err := tx.Create(&a); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM animals WHERE id = ?", a.ID).Exec() })

	rows, err := unmappedSpeciesDiagnostics(tx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, row := range rows {
		if row.Species == creavesSpecies {
			found = true
			if row.AnimalCount < 1 {
				t.Fatalf("unmapped species %q listed with %d animals, want >= 1", row.Species, row.AnimalCount)
			}
		}
	}
	if !found {
		t.Fatalf("unmapped species %q missing from diagnostic (got %d rows)", creavesSpecies, len(rows))
	}

	// Mapped species stay out of the list: the fixture above used a unique
	// marker, so a mapped control species must not appear.
	mappedSpecies := "DIAG-MAPPED-" + marker
	sp := models.Species{ID: "diag-mapped-" + marker, Species: "Diag mapped " + marker, Class: "class", Order: "order", Family: "family", CreavesSpecies: mappedSpecies, SubsideGroup: "group", AgwGroup: "agw", NativeStatus: "native", AnimaltypeID: at.ID}
	if err := tx.Create(&sp); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { tx.RawQuery("DELETE FROM species WHERE ID = ?", sp.ID).Exec() })

	for _, row := range rows {
		if row.Species == mappedSpecies {
			t.Fatalf("mapped species %q must not be listed as unmapped", mappedSpecies)
		}
	}
}
