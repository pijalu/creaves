package actions

import (
	"fmt"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// completeAndValidateSpeciesType resolves a blank type from the canonical
// species link and rejects an explicit mismatch. Unmapped species remain
// allowed during the nullable migration rollout and are handled by seed/report
// diagnostics rather than guessed at request time.
func completeAndValidateSpeciesType(tx *pop.Connection, animal *models.Animal) error {
	if animal.Species == "" {
		return nil
	}
	var ids []uuid.UUID
	if err := tx.RawQuery("SELECT DISTINCT animaltype_id FROM species WHERE creaves_species = ? AND animaltype_id IS NOT NULL", animal.Species).All(&ids); err != nil {
		return err
	}
	if animal.AnimaltypeID == uuid.Nil {
		if len(ids) == 1 {
			animal.AnimaltypeID = ids[0]
			return nil
		}
		if len(ids) > 1 {
			return fmt.Errorf("species %q has multiple animal type mappings", animal.Species)
		}
		return fmt.Errorf("species %q has no animal type mapping", animal.Species)
	}
	for _, id := range ids {
		if animal.AnimaltypeID == id {
			return nil
		}
	}
	if len(ids) == 0 || animal.AnimaltypeID != uuid.Nil {
		return fmt.Errorf("species %q does not match selected animal type", animal.Species)
	}
	return nil
}

func animalSpeciesTypeMismatch(tx *pop.Connection, animal *models.Animal) bool {
	if animal == nil || animal.Species == "" || animal.AnimaltypeID == uuid.Nil {
		return false
	}
	var matches []uuid.UUID
	if err := tx.RawQuery("SELECT animaltype_id FROM species WHERE creaves_species = ? AND animaltype_id = ? LIMIT 1", animal.Species, animal.AnimaltypeID).All(&matches); err != nil {
		return false
	}
	return len(matches) == 0
}
