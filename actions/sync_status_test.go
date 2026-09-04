//go:build sqlite
// +build sqlite

package actions

import (
	crypto_sha256 "crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const syncStatusInstance = "syncstatus-test"

// TestStateSetChecksumGolden pins the shared checksum formula on the producer
// side. The identical formula lives in creaves-console/actions/sync_checksum.go;
// the two admin UIs compare these strings, so any divergence breaks the feature.
func TestStateSetChecksumGolden(t *testing.T) {
	// Empty set → empty-input SHA-256 (must match the console's empty-set value).
	assert.Equal(t,
		"sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StateSetChecksum(nil))

	// Two lines: sorted lexicographically, joined with "\n", sha256-hex prefixed.
	sum := crypto_sha256.Sum256([]byte("42|abc\n7|bcd"))
	assert.Equal(t, "sha256:"+hex.EncodeToString(sum[:]), StateSetChecksum([]string{"7|bcd", "42|abc"}))
}

// seedSyncStatusFixture creates four animals (two 2025, two 2026) with the
// association chain the payload builder reads. No outtakes: all in_care.
func seedSyncStatusFixture(t *testing.T) {
	t.Helper()
	now := "CURRENT_TIMESTAMP"

	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
			t.Fatalf("fixture insert failed: %v\nquery: %s", err, q)
		}
	}

	cleanup := func() {
		for _, q := range []string{
			"DELETE FROM event_streams WHERE instance_id = '" + syncStatusInstance + "'",
			"DELETE FROM animals WHERE id BETWEEN 985100 AND 985103",
			"DELETE FROM intakes WHERE id LIKE '55555556-0000-0000-0000-0000000000%'",
			"DELETE FROM discoveries WHERE id LIKE '66666667-0000-0000-0000-0000000000%'",
			"DELETE FROM discoverers WHERE id = 'dddddddd-4444-4444-4444-4444444444f2'",
			"DELETE FROM entry_causes WHERE id = 'SYNCST_EC1'",
			"DELETE FROM animaltypes WHERE id = 'bbbbbbbb-2222-2222-2222-2222222222f2'",
			"DELETE FROM animalages WHERE id = 'aaaaaaaa-1111-1111-1111-1111111111f2'",
		} {
			_ = models.DB.RawQuery(q).Exec()
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	exec("INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('aaaaaaaa-1111-1111-1111-1111111111f2', 'SYNCST Young', 0, " + now + ", " + now + ")")
	exec("INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('bbbbbbbb-2222-2222-2222-2222222222f2', 'SYNCST Type', 0, " + now + ", " + now + ")")
	exec("INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('SYNCST_EC1', 'SYNCST_C1', 'SYNCST_D1', 'SYNCST_N1', 'x', " + now + ", " + now + ", 1)")
	exec("INSERT INTO discoverers (id, created_at, updated_at) VALUES ('dddddddd-4444-4444-4444-4444444444f2', " + now + ", " + now + ")")

	type spec struct {
		id         int
		seq        int
		year       int
		yearNumber int
	}
	specs := []spec{
		{id: 985100, seq: 1, year: 2025, yearNumber: 1}, // A: delivered, current hash → confirmed
		{id: 985101, seq: 2, year: 2026, yearNumber: 2}, // B: pending state event → unconfirmed
		{id: 985102, seq: 3, year: 2026, yearNumber: 3}, // C: never synced
		{id: 985103, seq: 4, year: 2025, yearNumber: 4}, // D: delivered but stale hash → unconfirmed
	}
	for _, s := range specs {
		intakeID := fmt.Sprintf("55555556-0000-0000-0000-0000000000%02d", s.seq)
		discID := fmt.Sprintf("66666667-0000-0000-0000-0000000000%02d", s.seq)
		exec("INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intakeID + "', '2025-06-01 10:00:00', " + now + ", " + now + ")")
		exec("INSERT INTO discoveries (id, date, discoverer_id, entry_cause_id, created_at, updated_at) VALUES ('" + discID + "', '2025-06-01 10:00:00', 'dddddddd-4444-4444-4444-4444444444f2', 'SYNCST_EC1', " + now + ", " + now + ")")
		exec(fmt.Sprintf(
			"INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (%d, 'SYNCST_Hedgehog', 'aaaaaaaa-1111-1111-1111-1111111111f2', 'bbbbbbbb-2222-2222-2222-2222222222f2', '%s', '%s', %s, %s, %d, %d, '2025-06-01 10:00:00')",
			s.id, discID, intakeID, now, now, s.year, s.yearNumber))
	}
}

// syncStatusLiveHash computes the animal's current state hash exactly the way
// the resync producer does (same eager loading, same payload builder).
func syncStatusLiveHash(t *testing.T, animalID int) string {
	t.Helper()
	animal := &models.Animal{}
	require.NoError(t, models.DB.Eager(
		"Animalage", "Animaltype", "Intake",
		"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
		"Outtake", "Outtake.Type",
	).Find(animal, animalID))
	payload := buildEventPayloadWithTranslations(models.DB, animal)
	require.NotNil(t, payload)
	return StateContentHashPayload(syncStatusInstance, *payload)
}

func TestComputeSyncStatusCountsPerYearAndChecksum(t *testing.T) {
	seedSyncStatusFixture(t)
	now := time.Now()

	hashes := map[int]string{}
	for _, id := range []int{985100, 985101, 985102, 985103} {
		hashes[id] = syncStatusLiveHash(t, id)
	}

	mkEvent := func(t *testing.T, animalID int, hash string, delivered bool) {
		t.Helper()
		e := &models.EventStream{
			ID: uuid.Must(uuid.NewV4()), InstanceID: syncStatusInstance, AnimalID: animalID,
			EventType: string(models.EventTypeAnimalState), ContentHash: &hash,
		}
		if delivered {
			e.DeliveredAt = &now
		}
		require.NoError(t, models.DB.Create(e))
	}
	// A: delivered with current hash → confirmed.
	mkEvent(t, 985100, hashes[985100], true)
	// B: state event exists but not yet delivered → unconfirmed (pending).
	mkEvent(t, 985101, hashes[985101], false)
	// C: no event at all → never-synced (still part of the expected set).
	// D: delivered but with an outdated hash → unconfirmed (stale). Its
	// CURRENT state has never been sent, so it also counts as never-synced.
	mkEvent(t, 985103, "0000000000000000000000000000000000000000000000000000000000000000", true)

	status, err := ComputeSyncStatus(models.DB, syncStatusInstance)
	require.NoError(t, err)

	assert.Equal(t, 4, status.ExpectedTotal)
	assert.Equal(t, 1, status.StateConfirmed)
	assert.Equal(t, 3, status.StateUnconfirmed, "pending + stale + never-synced")
	assert.Equal(t, 2, status.NeverSynced, "C (no event) + D (current state never sent)")

	// Per-year breakdown: 2025 = A (confirmed) + D (stale); 2026 = B + C (both unconfirmed).
	require.Len(t, status.Years, 2)
	assert.Equal(t, 2025, status.Years[0].Year)
	assert.Equal(t, 2, status.Years[0].Total)
	assert.Equal(t, 1, status.Years[0].Confirmed)
	assert.Equal(t, 2026, status.Years[1].Year)
	assert.Equal(t, 2, status.Years[1].Total)
	assert.Equal(t, 0, status.Years[1].Confirmed)

	// Expected checksum covers ALL current animals with their LIVE hashes —
	// including animals without any event yet and the stale one (not "stalehash").
	lines := make([]string, 0, 4)
	for _, id := range []int{985100, 985101, 985102, 985103} {
		lines = append(lines, fmt.Sprintf("%d|%s", id, hashes[id]))
	}
	assert.Equal(t, StateSetChecksum(lines), status.ExpectedChecksum)
	assert.True(t, len(status.ExpectedChecksum) == len("sha256:")+64)
}
