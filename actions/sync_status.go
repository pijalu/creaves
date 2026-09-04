package actions

import (
	crypto_sha256 "crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
)

// StateSetChecksum computes the shared sync fingerprint over a set of
// "<animal_id>|<state_hash>" lines: lines are sorted lexicographically, joined
// with "\n" and SHA-256 hashed, with a "sha256:" prefix. The identical formula
// is implemented on the creaves-console side (receiver), so the two admin UIs
// can verify sync by comparing the printed checksums.
func StateSetChecksum(lines []string) string {
	sorted := make([]string, len(lines))
	copy(sorted, lines)
	sort.Strings(sorted)
	sum := crypto_sha256.Sum256([]byte(strings.Join(sorted, "\n")))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// SyncStatusYear is one year bucket of the expected-set breakdown.
type SyncStatusYear struct {
	Year        int `json:"year"`
	Total       int `json:"total"`
	Confirmed   int `json:"confirmed"`
	Unconfirmed int `json:"unconfirmed"`
}

// SyncStatus is the producer-side (expected) sync state of this instance,
// shown on /webhook_resync. The console computes the complementary
// confirmed/unconfirmed view from its own data; the two ExpectedChecksum /
// console event-log checksums must be equal for a verified sync.
type SyncStatus struct {
	// ExpectedTotal is the number of current animals in this database —
	// what the console should eventually hold for this instance.
	ExpectedTotal int `json:"expected_total"`
	// StateConfirmed counts animals whose CURRENT state hash is already on a
	// DELIVERED animal_state event.
	StateConfirmed int `json:"state_confirmed"`
	// StateUnconfirmed = ExpectedTotal - StateConfirmed: animals with only a
	// pending state event, a stale one, or none at all.
	StateUnconfirmed int `json:"state_unconfirmed"`
	// NeverSynced counts animals that have no animal_state event with their
	// current hash at all (subset of StateUnconfirmed).
	NeverSynced int `json:"never_synced"`
	// ExpectedChecksum fingerprints the expected set (live-computed hashes).
	ExpectedChecksum string `json:"expected_checksum"`
	// Years breaks the expected set down per year, oldest first — this is the
	// visibility that would have exposed the "no animals of 2026" incident.
	Years []SyncStatusYear `json:"years"`
}

// ComputeSyncStatus derives the producer-side expected set for one instance.
// The per-animal state hash is recomputed live (same payload builder as the
// resync), so the result reflects what a fresh full extract WOULD send.
func ComputeSyncStatus(tx *pop.Connection, instanceID string) (*SyncStatus, error) {
	status := &SyncStatus{Years: []SyncStatusYear{}}

	animals := &models.Animals{}
	// EagerPreload() (not Eager()) batches each association into one
	// `id IN (...)` query; Eager issues per-record SELECTs — 8 per animal on
	// every /webhook_resync page load.
	if err := tx.EagerPreload(
		"Animalage", "Animaltype", "Intake",
		"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
		"Outtake", "Outtake.Type",
	).All(animals); err != nil {
		return nil, fmt.Errorf("failed to load animals: %w", err)
	}

	// Delivered/pending state events per animal: (animal_id, content_hash).
	type stateEventRow struct {
		AnimalID    int        `db:"animal_id"`
		ContentHash string     `db:"content_hash"`
		DeliveredAt *time.Time `db:"delivered_at"`
	}
	eventRows := []stateEventRow{}
	if err := tx.RawQuery(
		"SELECT animal_id, content_hash, delivered_at FROM event_streams WHERE instance_id = ? AND event_type = 'animal_state' AND content_hash IS NOT NULL",
		instanceID,
	).All(&eventRows); err != nil {
		return nil, fmt.Errorf("failed to load state events: %w", err)
	}
	// deliveredHashes: current-hash events confirmed by delivery.
	deliveredHashes := map[int]map[string]bool{}
	pendingHashes := map[int]map[string]bool{}
	for i := range eventRows {
		row := &eventRows[i]
		if row.DeliveredAt != nil {
			if deliveredHashes[row.AnimalID] == nil {
				deliveredHashes[row.AnimalID] = map[string]bool{}
			}
			deliveredHashes[row.AnimalID][row.ContentHash] = true
		} else {
			if pendingHashes[row.AnimalID] == nil {
				pendingHashes[row.AnimalID] = map[string]bool{}
			}
			pendingHashes[row.AnimalID][row.ContentHash] = true
		}
	}

	lines := make([]string, 0, len(*animals))
	yearIndex := map[int]int{}
	// Batch the reference lookups (translations + species taxonomy) once for
	// the whole set — the per-animal queries used to flood the SQL log on
	// every /webhook_resync page load.
	pre := newTranslationPreloader(tx, animals)
	for i := range *animals {
		animal := &(*animals)[i]
		payload := buildEventPayloadInto(tx, pre, animal)
		if payload == nil {
			// Same handling as the resync loop: skip unusable animals in the
			// fingerprint, they will surface as resync errors.
			continue
		}
		hash := StateContentHashPayload(instanceID, *payload)
		lines = append(lines, fmt.Sprintf("%d|%s", animal.ID, hash))
		status.ExpectedTotal++

		confirmed := deliveredHashes[animal.ID][hash]
		if confirmed {
			status.StateConfirmed++
		} else {
			status.StateUnconfirmed++
			if !pendingHashes[animal.ID][hash] {
				status.NeverSynced++
			}
		}

		idx, ok := yearIndex[animal.Year]
		if !ok {
			idx = len(status.Years)
			yearIndex[animal.Year] = idx
			status.Years = append(status.Years, SyncStatusYear{Year: animal.Year})
		}
		status.Years[idx].Total++
		if confirmed {
			status.Years[idx].Confirmed++
		} else {
			status.Years[idx].Unconfirmed++
		}
	}

	sort.Slice(status.Years, func(a, b int) bool { return status.Years[a].Year < status.Years[b].Year })
	status.ExpectedChecksum = StateSetChecksum(lines)
	return status, nil
}
