package actions

import (
	"fmt"
	"log"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// completeAndValidateSpeciesType resolves a blank type from the canonical
// species link and rejects an explicit contradiction with an existing mapping.
// Species with zero approved mappings are soft-blocked: the submitted type
// (or a blank one) is accepted and a warning is logged instead of failing the
// request; the maintenance page lists these unmapped species for follow-up.
func completeAndValidateSpeciesType(tx *pop.Connection, animal *models.Animal) error {
	if animal.Species == "" {
		return nil
	}
	var ids []uuid.UUID
	if err := tx.RawQuery("SELECT DISTINCT animaltype_id FROM species WHERE creaves_species = ? AND animaltype_id IS NOT NULL", animal.Species).All(&ids); err != nil {
		return err
	}
	if animal.AnimaltypeID == uuid.Nil {
		switch len(ids) {
		case 0:
			// Soft-block: no approved mapping exists, keep the blank type.
			log.Printf("WARNING: species %q has no animal type mapping; saving animal with blank animal type", animal.Species)
			return nil
		case 1:
			animal.AnimaltypeID = ids[0]
			return nil
		default:
			return fmt.Errorf("species %q has multiple animal type mappings", animal.Species)
		}
	}
	if len(ids) == 0 {
		// Soft-block: nothing mapped, accept the submitted type.
		log.Printf("WARNING: species %q has no animal type mapping; accepting submitted animal type %s", animal.Species, animal.AnimaltypeID)
		return nil
	}
	for _, id := range ids {
		if animal.AnimaltypeID == id {
			return nil
		}
	}
	return fmt.Errorf("species %q does not match selected animal type", animal.Species)
}

func animalSpeciesTypeMismatch(tx *pop.Connection, animal *models.Animal) bool {
	if animal == nil || animal.Species == "" || animal.AnimaltypeID == uuid.Nil {
		return false
	}
	// Unmapped species have nothing to contradict: they are accepted with a
	// warning (see completeAndValidateSpeciesType) and surfaced on the
	// maintenance page instead of being flagged as a mismatch here.
	var mapped []uuid.UUID
	if err := tx.RawQuery("SELECT DISTINCT animaltype_id FROM species WHERE creaves_species = ? AND animaltype_id IS NOT NULL", animal.Species).All(&mapped); err != nil || len(mapped) == 0 {
		return false
	}
	var matches []uuid.UUID
	if err := tx.RawQuery("SELECT animaltype_id FROM species WHERE creaves_species = ? AND animaltype_id = ? LIMIT 1", animal.Species, animal.AnimaltypeID).All(&matches); err != nil {
		return false
	}
	return len(matches) == 0
}
