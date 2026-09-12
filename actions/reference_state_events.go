package actions

import (
	"fmt"
	"sort"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// maxReferenceAnimalEvents bounds how many animal_state events a single
// reference remap/delete may re-emit. A reference row (age, type, drug, ...)
// can be linked from a very large number of animals; without a cap one admin
// request could enqueue tens of thousands of payload builds and webhook
// events. Animals beyond the cap keep their stale payload until the next
// full resync reconciles them.
const maxReferenceAnimalEvents = 500

// referenceAnimalIDQueries maps each dependent table that carries an animal
// linkage to the bounded query collecting the animal ids affected by an FK
// change on the given column (%s = column, %d = limit). Tables without any
// animal linkage (species, dosages) are absent on purpose: they cannot change
// an animal_state payload, so no event would ever survive the content-hash
// dedupe.
var referenceAnimalIDQueries = map[string]string{
	"animals":     "SELECT id FROM `animals` WHERE `%s` = ? ORDER BY id LIMIT %d",
	"cares":       "SELECT animal_id FROM `cares` WHERE `%s` = ? ORDER BY animal_id LIMIT %d",
	"discoveries": "SELECT id FROM `animals` WHERE discovery_id IN (SELECT id FROM `discoveries` WHERE `%s` = ?) ORDER BY id LIMIT %d",
	"outtakes":    "SELECT id FROM `animals` WHERE outtake_id IN (SELECT id FROM `outtakes` WHERE `%s` = ?) ORDER BY id LIMIT %d",
	"travels":     "SELECT animal_id FROM `travels` WHERE `%s` = ? ORDER BY animal_id LIMIT %d",
}

// referenceAffectedAnimalIDs collects the distinct ids of animals whose event
// payload depends on reference rows pointing at sourceID. It must run BEFORE
// the FK updates so the affected rows can still be found by their old
// reference id. The union is deduplicated, sorted (deterministic emission
// order) and capped at maxReferenceAnimalEvents.
func referenceAffectedAnimalIDs(tx *pop.Connection, dependents map[string]string, sourceID uuid.UUID) ([]int, error) {
	seen := map[int]bool{}
	ids := []int{}
	for table, column := range dependents {
		tmpl, ok := referenceAnimalIDQueries[table]
		if !ok {
			continue
		}
		q := fmt.Sprintf(tmpl, column, maxReferenceAnimalEvents)
		var tableIDs []int
		if err := tx.RawQuery(q, sourceID).All(&tableIDs); err != nil {
			return nil, fmt.Errorf("failed to collect affected animals from %s: %w", table, err)
		}
		for _, id := range tableIDs {
			if !seen[id] {
				seen[id] = true
				ids = append(ids, id)
			}
		}
	}
	sort.Ints(ids)
	if len(ids) > maxReferenceAnimalEvents {
		ids = ids[:maxReferenceAnimalEvents]
	}
	return ids, nil
}

// publishReferenceStateEvents re-emits one animal_state event per affected
// animal after a reference remap/delete succeeded. Publishing happens on the
// request transaction; failures are logged, not fatal — the reference change
// itself already succeeded and the next resync reconciles any missed event.
func publishReferenceStateEvents(c buffalo.Context, tx *pop.Connection, animalIDs []int) {
	for _, animalID := range animalIDs {
		publishAnimalStateEventWarn(c, tx, animalID)
	}
}
