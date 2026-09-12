//go:build sqlite
// +build sqlite

package actions

import (
	"testing"

	"creaves/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResyncExcludesDestroyedAnimals pins the producer half of "Delete show
// as deceased in console": animals destroyed via the error outtake type are
// deletions, so neither the resync chunk loader nor the announced expected
// total may include them — otherwise the console (which removes them) could
// never match the producer's expected sync set.
func TestResyncExcludesDestroyedAnimals(t *testing.T) {
	now := "CURRENT_TIMESTAMP"
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
			t.Fatalf("fixture insert failed: %v\nquery: %s", err, q)
		}
	}

	const (
		ageID     = "aaaaaaaa-1111-1111-1111-1111111111e1"
		typeID    = "bbbbbbbb-2222-2222-2222-2222222222e1"
		errTypeID = "cccccccc-3333-3333-3333-3333333333e1"
		okTypeID  = "cccccccc-3333-3333-3333-3333333333e2"
		// intakes/discoveries are NOT NULL on animals: minimal rows.
		intakeA = "55555556-0000-0000-0000-e00000000001"
		intakeB = "55555556-0000-0000-0000-e00000000002"
		intakeC = "55555556-0000-0000-0000-e00000000003"
		dvrID   = "dddddddd-4444-4444-4444-e44444444441"
		discA   = "66666667-0000-0000-0000-e00000000001"
		discB   = "66666667-0000-0000-0000-e00000000002"
		discC   = "66666667-0000-0000-0000-e00000000003"
		outB    = "77777778-0000-0000-0000-e00000000002"
		outC    = "77777778-0000-0000-0000-e00000000003"
	)

	cleanup := func() {
		for _, q := range []string{
			"DELETE FROM animals WHERE id BETWEEN 985110 AND 985112",
			"DELETE FROM outtakes WHERE id IN ('" + outB + "', '" + outC + "')",
			"DELETE FROM intakes WHERE id IN ('" + intakeA + "', '" + intakeB + "', '" + intakeC + "')",
			"DELETE FROM discoveries WHERE id IN ('" + discA + "', '" + discB + "', '" + discC + "')",
			"DELETE FROM discoverers WHERE id = '" + dvrID + "'",
			"DELETE FROM animaltypes WHERE id IN ('" + typeID + "', '" + errTypeID + "', '" + okTypeID + "')",
			"DELETE FROM animalages WHERE id = '" + ageID + "'",
		} {
			_ = models.DB.RawQuery(q).Exec()
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	exec("INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + ageID + "', 'SYNCDEL Age', 0, " + now + ", " + now + ")")
	exec("INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + typeID + "', 'SYNCDEL Type', 0, " + now + ", " + now + ")")
	exec("INSERT INTO discoverers (id, created_at, updated_at) VALUES ('" + dvrID + "', " + now + ", " + now + ")")
	// Doublon-shaped error type vs a normal release-shaped type.
	exec("INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('" + errTypeID + "', 'SYNCDEL Doublon', 0, " + now + ", " + now + ", 0, -1, 1)")
	exec("INSERT INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('" + okTypeID + "', 'SYNCDEL Released', 0, " + now + ", " + now + ", 0, 1, 0)")

	seedAnimal := func(id int, intakeID, discID string) {
		exec("INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intakeID + "', '2025-06-01 10:00:00', " + now + ", " + now + ")")
		exec("INSERT INTO discoveries (id, date, discoverer_id, created_at, updated_at) VALUES ('" + discID + "', '2025-06-01 10:00:00', '" + dvrID + "', " + now + ", " + now + ")")
		exec("INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (" +
			itoa(id) + ", 'SYNCDEL Fox', '" + ageID + "', '" + typeID + "', '" + discID + "', '" + intakeID + "', " + now + ", " + now + ", 2025, " + itoa(id) + ", '2025-06-01 10:00:00')")
	}
	// A: plain in-care (no outtake) → must be synced.
	seedAnimal(985110, intakeA, discA)
	// B: destroyed (error outtake) → must be EXCLUDED.
	seedAnimal(985111, intakeB, discB)
	exec("INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + outB + "', '2025-07-01 10:00:00', '" + errTypeID + "', " + now + ", " + now + ")")
	exec("UPDATE animals SET outtake_id = '" + outB + "' WHERE id = 985111")
	// C: normally released (non-error outtake) → must be synced.
	seedAnimal(985112, intakeC, discC)
	exec("INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + outC + "', '2025-07-01 10:00:00', '" + okTypeID + "', " + now + ", " + now + ")")
	exec("UPDATE animals SET outtake_id = '" + outC + "' WHERE id = 985112")

	// Chunk loader: returns A and C, skips B.
	seen := map[int]bool{}
	afterID := 0
	for {
		chunk, nextID, err := loadResyncAnimalChunk(models.DB, afterID)
		require.NoError(t, err)
		if len(*chunk) == 0 {
			break
		}
		afterID = nextID
		for i := range *chunk {
			seen[(*chunk)[i].ID] = true
		}
	}
	assert.True(t, seen[985110], "plain animal must be synced")
	assert.True(t, seen[985112], "normally released animal must be synced")
	assert.False(t, seen[985111], "destroyed (error outtake) animal must NOT be re-synced")

	// Announced expected total matches the chunk loader.
	total, err := countAnimals(models.DB)
	require.NoError(t, err)
	assert.Equal(t, 2, total, "expected total must exclude destroyed animals")
}
