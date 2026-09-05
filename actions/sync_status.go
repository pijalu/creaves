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
	// StateConfirmed counts animals whose CURRENT state hash was ACKNOWLEDGED
	// by the console (echoed back as stored on an animal_state event).
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

	ackedHashes, pendingHashes, err := loadAckedPendingHashes(tx, instanceID)
	if err != nil {
		return nil, err
	}

	// Stream animals in keyset-paginated chunks (the same bounded LEFT JOIN
	// as the resync) instead of one full-table EagerPreload: every
	// /webhook_resync page load recomputes the expected set, so neither
	// memory nor IN() sizes may scale with the animal count. Hashes are
	// identical to the old path — expectedStateHashes runs the unchanged
	// payload builder over the same association graph, just per chunk.
	var lines []string
	yearIndex := map[int]int{}
	afterID := 0
	for {
		animals, nextID, err := loadResyncAnimalChunk(tx, afterID)
		if err != nil {
			return nil, fmt.Errorf("failed to load animals: %w", err)
		}
		if len(*animals) == 0 {
			break
		}
		afterID = nextID
		yearByAnimal := make(map[int]int, len(*animals))
		for i := range *animals {
			yearByAnimal[(*animals)[i].ID] = (*animals)[i].Year
		}
		pre := newTranslationPreloader(tx, animals)
		// Animals whose payload could not be built are NOT part of the
		// expected set (same as the resync: they surface as resync errors).
		for _, hl := range expectedStateHashes(tx, animals, pre, instanceID) {
			lines = append(lines, fmt.Sprintf("%d|%s", hl.AnimalID, hl.Hash))
			status.recordHash(hl, yearByAnimal[hl.AnimalID], ackedHashes, pendingHashes, yearIndex)
		}
	}

	sort.Slice(status.Years, func(a, b int) bool { return status.Years[a].Year < status.Years[b].Year })
	status.ExpectedChecksum = StateSetChecksum(lines)
	return status, nil
}

// recordHash folds one expected-state hash into the totals and the per-year
// bucket: confirmed only when the console acknowledged THIS hash; otherwise
// unconfirmed, and never-synced when no event with the hash exists at all.
func (status *SyncStatus) recordHash(hl stateHashLine, year int, acked, pending map[int]map[string]bool, yearIndex map[int]int) {
	status.ExpectedTotal++
	confirmed := acked[hl.AnimalID][hl.Hash]
	if confirmed {
		status.StateConfirmed++
	} else {
		status.StateUnconfirmed++
		if !pending[hl.AnimalID][hl.Hash] {
			status.NeverSynced++
		}
	}
	bumpYearBucket(status, yearIndex, year, confirmed)
}

// loadAckedPendingHashes returns the acknowledged / pending state-event
// content hashes per animal: (animal_id, content_hash). An event counts as
// confirmed only when the console echoed its state hash back (acknowledged_at
// set) — an HTTP delivery alone does not prove the console stored the state.
func loadAckedPendingHashes(tx *pop.Connection, instanceID string) (acked, pending map[int]map[string]bool, err error) {
	acked = map[int]map[string]bool{}
	pending = map[int]map[string]bool{}
	type stateEventRow struct {
		AnimalID       int        `db:"animal_id"`
		ContentHash    string     `db:"content_hash"`
		AcknowledgedAt *time.Time `db:"acknowledged_at"`
	}
	eventRows := []stateEventRow{}
	if err := tx.RawQuery(
		"SELECT animal_id, content_hash, acknowledged_at FROM event_streams WHERE instance_id = ? AND event_type = 'animal_state' AND content_hash IS NOT NULL",
		instanceID,
	).All(&eventRows); err != nil {
		return nil, nil, fmt.Errorf("failed to load state events: %w", err)
	}
	for i := range eventRows {
		row := &eventRows[i]
		target := &pending
		if row.AcknowledgedAt != nil {
			target = &acked
		}
		if (*target)[row.AnimalID] == nil {
			(*target)[row.AnimalID] = map[string]bool{}
		}
		(*target)[row.AnimalID][row.ContentHash] = true
	}
	return acked, pending, nil
}

// bumpYearBucket appends/updates the per-year Confirmed/Unconfirmed bucket.
func bumpYearBucket(status *SyncStatus, yearIndex map[int]int, year int, confirmed bool) {
	idx, ok := yearIndex[year]
	if !ok {
		idx = len(status.Years)
		yearIndex[year] = idx
		status.Years = append(status.Years, SyncStatusYear{Year: year})
	}
	status.Years[idx].Total++
	if confirmed {
		status.Years[idx].Confirmed++
	} else {
		status.Years[idx].Unconfirmed++
	}
}

// stateHashLine is one animal's expected (current) state hash.
type stateHashLine struct {
	AnimalID int
	Hash     string
}

// expectedStateHashes computes the per-animal current state hash for a set
// of PRELOADED animals (same payload builder as the resync). It issues no
// per-animal SELECTs — reference lookups come from the shared preloader —
// so callers may run it inside the resync loop without breaking the
// query-count bounds. Animals whose payload cannot be built are skipped
// (same handling as the resync loop: they surface as resync errors).
func expectedStateHashes(tx *pop.Connection, animals *models.Animals, pre *translationPreloader, instanceID string) []stateHashLine {
	lines := make([]stateHashLine, 0, len(*animals))
	for i := range *animals {
		animal := &(*animals)[i]
		payload := buildEventPayloadInto(tx, pre, animal)
		if payload == nil {
			continue
		}
		// Hash exactly what the resync/update paths would send: without the
		// shared CurrentStatus derivation the computed hash never matches any
		// stored content_hash and the whole set shows as unconfirmed.
		applyCurrentStatus(payload, animal)
		lines = append(lines, stateHashLine{
			AnimalID: animal.ID,
			Hash:     StateContentHashPayload(instanceID, *payload),
		})
	}
	return lines
}
