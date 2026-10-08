//go:build sqlite
// +build sqlite

package actions

import (
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"
)

// ensureLandingCaresTable creates the cares table in the shared SQLite test
// database if a previous test run has not already done so. Column set mirrors
// migrations/schema.sql for cares; the landing clean-cage query only touches
// animal_id, date and clean.
func ensureLandingCaresTable(t *testing.T, tx *pop.Connection) {
	t.Helper()
	q := `CREATE TABLE IF NOT EXISTS cares (
  "id" char(36) PRIMARY KEY,
  "date" datetime NOT NULL,
  "animal_id" int NOT NULL,
  "type_id" char(36) NOT NULL,
  "weight" varchar(255) DEFAULT NULL,
  "note" text,
  "clean" tinyint(1) DEFAULT NULL,
  "in_warning" tinyint(1) DEFAULT NULL,
  "heat_source" varchar(255) DEFAULT NULL,
  "oxygen" tinyint(1) NOT NULL DEFAULT '0',
  "link_to_id" char(36) DEFAULT NULL,
  "created_at" datetime NOT NULL,
  "updated_at" datetime NOT NULL
)`
	require.NoError(t, tx.RawQuery(q).Exec())
}

// TestAnimalIDsCleanCageSince_Boundary pins the landing "cleaned cage" green
// marker window (BUG-2): it must be a rolling last-24h window. The previous
// predicate `c.date >= CURDATE()+3h` silently dropped clean cares dated
// 00:00–03:00 of the current calendar day the moment the wall clock passed
// midnight, and yesterday-evening cares after midnight — night-shift work
// became invisible. Deterministic: the cutoff is passed to
// animalIDsCleanCageSince explicitly instead of being read from the wall
// clock inside the query.
func TestAnimalIDsCleanCageSince_Boundary(t *testing.T) {
	tx := pusherTestDB
	ensureLandingCaresTable(t, tx)

	base := time.Now()

	// Dedicated animal IDs so the fixtures stay isolated from the other
	// tests sharing pusher_test.db.
	type fixture struct {
		animalID int
		age      time.Duration
		clean    bool
		want     bool
	}
	fixtures := []fixture{
		// Inside the window: a night-shift care recorded 2h ago must be
		// included (this is the BUG-2 case, e.g. dated 00:12).
		{animalID: 990101, age: -2 * time.Hour, clean: true, want: true},
		// Exactly on the boundary: >= semantics, included.
		{animalID: 990102, age: -24 * time.Hour, clean: true, want: true},
		// Outside the window: a care from 25h ago must be excluded.
		{animalID: 990103, age: -25 * time.Hour, clean: true, want: false},
		// Negative control: only clean=1 cares count.
		{animalID: 990104, age: -2 * time.Hour, clean: false, want: false},
	}

	for _, f := range fixtures {
		f := f
		care := &models.Care{
			ID:       uuid.Must(uuid.NewV4()),
			Date:     base.Add(f.age),
			AnimalID: f.animalID,
			TypeID:   uuid.Must(uuid.NewV4()),
			Clean:    nulls.NewBool(f.clean),
		}
		require.NoError(t, tx.Create(care), "fixture care for animal %d", f.animalID)
		t.Cleanup(func() {
			tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", f.animalID).Exec()
		})
	}

	got, err := animalIDsCleanCageSince(tx, base.Add(-24*time.Hour))
	require.NoError(t, err)

	for _, f := range fixtures {
		require.Equal(t, f.want, got[f.animalID],
			"animal %d (clean care %v ago, clean=%v) included=%v, want %v",
			f.animalID, f.age, f.clean, got[f.animalID], f.want)
	}
}

// TestListAnimalWithCleanCage_MapsIDsToTrue checks the public entry point's
// contract on a representative row: every returned animal id maps to true
// (the template does animalsWithCleanCage[animal.ID] lookups).
func TestListAnimalWithCleanCage_MapsIDsToTrue(t *testing.T) {
	tx := pusherTestDB
	ensureLandingCaresTable(t, tx)

	care := &models.Care{
		ID:       uuid.Must(uuid.NewV4()),
		Date:     time.Now().Add(-1 * time.Hour),
		AnimalID: 990201,
		TypeID:   uuid.Must(uuid.NewV4()),
		Clean:    nulls.NewBool(true),
	}
	require.NoError(t, tx.Create(care))
	t.Cleanup(func() { tx.RawQuery("DELETE FROM cares WHERE animal_id = ?", 990201).Exec() })

	got, err := animalIDsCleanCageSince(tx, time.Now().Add(-24*time.Hour))
	require.NoError(t, err)
	require.Equal(t, true, got[990201])
}
