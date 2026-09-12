//go:build sqlite
// +build sqlite

package actions

import (
	"context"
	"fmt"
	stdlog "log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/pop/v6/logging"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loadedPreloader is the sqlite-tag test helper replacing the deleted
// production constructor: builds a translation preloader and eagerly loads
// the reference data the given animals need.
func loadedPreloader(tx *pop.Connection, animals *models.Animals) *translationPreloader {
	p := newTranslationPreloader()
	p.ensure(tx, animals)
	return p
}

// These tests pin the N+1 fix for the webhook full resync: per-animal
// translation/species SELECTs and the per-animal EXISTS check were replaced
// by run-wide batches (translationPreloader + resyncStateIndex).

const preloaderInstance = "rsync-n1-test"

const (
	preloaderAgeID    = "aaaaaaaa-1111-1111-1111-1111111111f4"
	preloaderTypeID   = "bbbbbbbb-2222-2222-2222-2222222222f4"
	preloaderOutID    = "cccccccc-3333-3333-3333-3333333333f4"
	preloaderDiscID   = "dddddddd-4444-4444-4444-4444444444f4"
	preloaderCauseID  = "PRE_EC1"
	preloaderSpecies  = "PRE_Hedgehog"
	preloaderTrPrefix = "22222222-0000-0000-0000-0000000000"
)

// seedPreloaderAnimal inserts the shared reference rows (idempotent) plus one
// fully-associated animal (species taxonomy, entry cause, outtake type, zone)
// and translations in en-US/de/nl. Cleanup is registered once per animal id.
func seedPreloaderAnimal(t *testing.T, animalID int) {
	t.Helper()
	now := "CURRENT_TIMESTAMP"

	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := models.DB.RawQuery(q, args...).Exec(); err != nil {
			t.Fatalf("fixture insert failed: %v\nquery: %s", err, q)
		}
	}
	// Reference rows: shared by all seeded animals → ignore duplicates.
	ref := func(q string) {
		t.Helper()
		if err := models.DB.RawQuery(q).Exec(); err != nil {
			t.Fatalf("reference insert failed: %v\nquery: %s", err, q)
		}
	}

	intakeSuffix := fmt.Sprintf("%02x", animalID%256)
	intakeID := "55555555-0000-0000-0000-0000000000" + intakeSuffix
	discRowID := "66666666-0000-0000-0000-0000000000" + intakeSuffix
	outRowID := "77777777-0000-0000-0000-0000000000" + intakeSuffix

	cleanup := func() {
		for _, q := range []string{
			fmt.Sprintf("DELETE FROM animals WHERE id = %d", animalID),
			"DELETE FROM event_streams WHERE instance_id = '" + preloaderInstance + "'",
			"DELETE FROM resync_runs WHERE instance_id = '" + preloaderInstance + "'",
			"DELETE FROM outtakes WHERE id = '" + outRowID + "'",
			"DELETE FROM intakes WHERE id = '" + intakeID + "'",
			"DELETE FROM discoveries WHERE id = '" + discRowID + "'",
			"DELETE FROM translations WHERE record_id IN ('" +
				preloaderSpecies + "','" + preloaderCauseID + "','" + preloaderTypeID + "','" + preloaderOutID + "')",
		} {
			_ = models.DB.RawQuery(q).Exec()
		}
	}
	cleanup()
	t.Cleanup(cleanup)

	ref("INSERT OR IGNORE INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, game, agw_group, native_status, huntable) VALUES ('PRE_SP1', 'PRE Hedgehog', 'PRE_Mammalia', 'PRE_Erinaceidae', '" + preloaderSpecies + "', 'PRE_SUB', " + now + ", " + now + ", 'PRE_Order', 0, 'PRE_AGW', 'PRE_Ind', 0)")
	ref("INSERT OR IGNORE INTO animalages (id, name, `def`, created_at, updated_at) VALUES ('" + preloaderAgeID + "', 'PRE Young', 0, " + now + ", " + now + ")")
	ref("INSERT OR IGNORE INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES ('" + preloaderTypeID + "', 'PRE Type', 0, " + now + ", " + now + ")")
	ref("INSERT OR IGNORE INTO outtaketypes (id, name, `def`, created_at, updated_at, dead, rating, error) VALUES ('" + preloaderOutID + "', 'PRE_REL', 0, " + now + ", " + now + ", 0, 1, 0)")
	ref("INSERT OR IGNORE INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES ('" + preloaderCauseID + "', 'PRE_C1', 'PRE_D1', 'PRE_N1', 'x', " + now + ", " + now + ", 1)")
	ref("INSERT OR IGNORE INTO discoverers (id, created_at, updated_at) VALUES ('" + preloaderDiscID + "', " + now + ", " + now + ")")

	exec("INSERT INTO intakes (id, date, created_at, updated_at) VALUES ('" + intakeID + "', '2024-06-01 10:00:00', " + now + ", " + now + ")")
	exec("INSERT INTO discoveries (id, date, discoverer_id, entry_cause_id, created_at, updated_at) VALUES ('" + discRowID + "', '2024-06-01 10:00:00', '" + preloaderDiscID + "', '" + preloaderCauseID + "', " + now + ", " + now + ")")
	exec("INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES ('" + outRowID + "', '2024-08-01 09:00:00', '" + preloaderOutID + "', " + now + ", " + now + ")")
	exec(fmt.Sprintf("INSERT INTO animals (id, species, zone, animalage_id, animaltype_id, discovery_id, intake_id, outtake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (%d, '%s', 'PRE_Zone', '%s', '%s', '%s', '%s', '%s', %s, %s, 2024, 7, '2024-06-01 10:00:00')",
		animalID, preloaderSpecies, preloaderAgeID, preloaderTypeID, discRowID, intakeID, outRowID, now, now))

	// Translations in three locales (fr intentionally missing → base fallback).
	insertTranslation := func(seq int, table, record, field, locale, value string) {
		exec(fmt.Sprintf("INSERT OR IGNORE INTO translations (id, table_name, record_id, field, locale, value, created_at, updated_at) VALUES ('%s%02d', '%s', '%s', '%s', '%s', '%s', %s, %s)",
			preloaderTrPrefix, seq, table, record, field, locale, value, now, now))
	}
	insertTranslation(1, "species", "PRE_SP1", "class", "en-US", "PRE_Mammalia EN")
	insertTranslation(2, "species", "PRE_SP1", "agw_group", "de", "PRE_AGW DE")
	insertTranslation(3, "animaltypes", preloaderTypeID, "name", "de", "PRE Typ DE")
	insertTranslation(4, "entry_causes", preloaderCauseID, "cause", "en-US", "PRE Cause EN")
	insertTranslation(5, "outtaketypes", preloaderOutID, "name", "nl", "PRE Vrij nl")
	insertTranslation(6, "species", "PRE_SP1", "creaves_species", "en-US", "PRE Herisson EN")
}

// loadPreloaderAnimal loads animals exactly like the resync/sync-status
// loops do (same explicit eager list).
func loadPreloaderAnimals(t *testing.T, where string, args ...interface{}) *models.Animals {
	t.Helper()
	animals := &models.Animals{}
	require.NoError(t, models.DB.EagerPreload(
		"Animalage", "Animaltype", "Intake",
		"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
		"Outtake", "Outtake.Type",
	).Where(where, args...).All(animals))
	return animals
}

// TestTranslationPreloaderEquivalence proves the batched builder produces a
// payload identical to the per-animal builder (translations + species
// taxonomy) — the resync now uses the batched path.
func TestTranslationPreloaderEquivalence(t *testing.T) {
	seedPreloaderAnimal(t, 985060)
	animals := loadPreloaderAnimals(t, "animals.id = ?", 985060)
	require.Equal(t, 1, len(*animals))
	animal := &(*animals)[0]

	perAnimal := buildEventPayloadWithTranslations(models.DB, animal)
	pre := loadedPreloader(models.DB, animals)
	batched := buildEventPayloadInto(models.DB, pre, animal)

	perAnimal.Timestamp = ""
	batched.Timestamp = ""
	assert.Equal(t, perAnimal, batched)

	// Spot-check that translations really flowed in via the batched path.
	require.NotNil(t, batched.Translations)
	assert.Equal(t, "PRE_Mammalia EN", batched.Translations["en-US"]["species_class"])
	assert.Equal(t, "PRE_AGW DE", batched.Translations["de"]["species_agw_group"])
	assert.Equal(t, "PRE Typ DE", batched.Translations["de"]["animal_type"])
	assert.Equal(t, "PRE Cause EN", batched.Translations["en-US"]["entry_cause"])
	assert.Equal(t, "PRE Vrij nl", batched.Translations["nl"]["outtake_type"])
	// fr has no translation rows → canonical base values win (base is the
	// creaves_species name stored on animals.species).
	assert.Equal(t, preloaderSpecies, batched.Translations["fr"]["species"])
	// Taxonomy from the batched species map. Both paths resolve the species
	// row via `SELECT * FROM species WHERE creaves_species = ?` (RawQuery),
	// which avoids pop's unquoted `order` column and works on every dialect.
	assert.Equal(t, "PRE_Mammalia", batched.Animal.SpeciesClass)
	assert.Equal(t, "PRE_Ind", batched.Animal.SpeciesNativeStatus)
}

// TestTranslationPreloaderIncremental pins the run-wide reuse: a second
// ensure() with the SAME animals must issue ZERO queries (chunk 2..N of a
// resync/sync-status re-use chunk 1's reference data). On a low-end box the
// per-chunk full species/localities scans and per-chunk translation queries
// were pure repetition.
func TestTranslationPreloaderIncremental(t *testing.T) {
	seedPreloaderAnimal(t, 985061)
	animals := loadPreloaderAnimals(t, "animals.id = ?", 985061)
	require.Equal(t, 1, len(*animals))

	pre := newTranslationPreloader()
	pre.ensure(models.DB, animals) // chunk 1: loads everything

	var mu sync.Mutex
	selectCount := 0
	prevDebug := pop.Debug
	pop.Debug = true
	pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
		if level != logging.SQL {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(strings.ToUpper(s), "SELECT") {
			selectCount++
		}
	})
	defer func() {
		pop.Debug = prevDebug
		pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
			if !pop.Debug && level <= logging.Debug {
				return
			}
			if level == logging.SQL {
				stdlog.Printf("[POP] sql - %s", s)
				return
			}
			stdlog.Printf("[POP] "+s, args...)
		})
	}()

	pre.ensure(models.DB, animals) // chunk 2 with identical references
	pre.ensure(models.DB, animals) // chunk 3, same

	mu.Lock()
	count := selectCount
	mu.Unlock()
	assert.Equal(t, 0, count, "repeated ensure() with already-covered references must issue no queries")

	// Values must still resolve exactly like a freshly built preloader.
	fresh := loadedPreloader(models.DB, animals)
	animal := &(*animals)[0]
	payload := buildEventPayloadInto(models.DB, pre, animal)
	freshPayload := buildEventPayloadInto(models.DB, fresh, animal)
	require.NotNil(t, payload)
	payload.Timestamp = ""
	freshPayload.Timestamp = ""
	assert.Equal(t, freshPayload, payload)
}

// TestRunResyncQueryCountBounded is the regression test for the N+1 flood:
// a 60-animal resync must issue far fewer than one SELECT per animal
// (previously ≈ 51 SELECTs per animal: 48 translation lookups, 1 species
// lookup, 1 EXISTS, 1 run re-read).
func TestRunResyncQueryCountBounded(t *testing.T) {
	const animalCount = 60
	for i := 0; i < animalCount; i++ {
		seedPreloaderAnimal(t, 986000+i)
	}
	animals := loadPreloaderAnimals(t, "animals.id BETWEEN ? AND ?", 986000, 986000+animalCount-1)
	require.Equal(t, animalCount, len(*animals))

	run := &models.ResyncRun{
		ID:           uuid.Must(uuid.NewV4()),
		InstanceID:   preloaderInstance,
		Status:       "running",
		StartedAt:    time.Now(),
		TotalAnimals: animalCount,
	}
	require.NoError(t, models.DB.Create(run))

	var mu sync.Mutex
	selectCount := 0
	shapes := map[string]int{}
	prevDebug := pop.Debug
	pop.Debug = true // txlog suppresses SQL unless Debug; restore below.
	pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
		if level != logging.SQL {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(strings.ToUpper(s), "SELECT") {
			selectCount++
			shape := strings.ToUpper(s)
			if i := strings.Index(shape, " FROM "); i > 0 {
				shape = shape[:i]
			} else if i := strings.Index(shape, " AS "); i > 0 && strings.Contains(shape[:i], "SELECT") {
				shape = shape[:i]
			}
			if len(shape) > 80 {
				shape = shape[:80]
			}
			shapes[shape]++
		}
	})
	// Restore a plain stdlog-based pop tx logger afterwards (the original
	// default is not readable back); format fidelity is irrelevant for the
	// remaining tests.
	defer func() {
		pop.Debug = prevDebug
		pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
			if !pop.Debug && level <= logging.Debug {
				return
			}
			if level == logging.SQL {
				stdlog.Printf("[POP] sql - %s", s)
				return
			}
			stdlog.Printf("[POP] "+s, args...)
		})
	}()

	// A run may only report "completed" once every event was delivered:
	// point webhook delivery at a stub receiver that accepts every batch
	// (empty 200 body = legacy full acceptance in deliverBatch).
	acceptAll := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer acceptAll.Close()
	savedCfg := CurrentConfigGet()
	cfg := &models.Config{ID: uuid.Must(uuid.NewV4()), InstanceID: preloaderInstance, Name: "resync-n1-stub", Active: true}
	require.NoError(t, cfg.SetSettings(models.ConfigSettings{
		EnableEventStream: true, WebhookEnabled: true, WebhookURL: acceptAll.URL,
		WebhookAPIKey: "k", WebhookBatchSize: 100, WebhookMaxPerMin: 10000,
	}))
	CurrentConfigSet(cfg)
	defer func() { CurrentConfigSet(savedCfg) }()

	require.NoError(t, RunResync(context.Background(), models.DB, run.ID, false))

	mu.Lock()
	count := selectCount
	for shape, n := range shapes {
		t.Logf("%4d × %s", n, shape)
	}
	mu.Unlock()

	finished := &models.ResyncRun{}
	require.NoError(t, models.DB.Find(finished, run.ID))
	assert.Equal(t, "completed", finished.Status)
	assert.Equal(t, animalCount, finished.AnimalsProcessed)
	assert.Equal(t, animalCount, finished.EventsCreated)
	assert.Equal(t, animalCount, finished.EventsDelivered, "completed run must have delivered every created event")
	assert.Equal(t, 0, finished.EventsFailed)

	// Batched cost is a fixed budget (~10 association/animal preloads + 48
	// translation group queries + index/checkpoints) and must not scale with
	// the animal count. Old behaviour: ~59 SELECTs PER animal (~3540 here).
	// The delivery-accounting loop adds a small fixed number of COUNT/EXISTS
	// queries on top, hence the +80 headroom (bugs.md #1).
	assert.Less(t, count, animalCount+80,
		"resync issued %d SELECTs for %d animals — per-record SELECT N+1 is back", count, animalCount)
	t.Logf("resync over %d animals issued %d SELECTs", animalCount, count)
}

// TestComputeSyncStatusQueryCountBounded pins the same bound for the
// /webhook_resync page load: ComputeSyncStatus recomputes the expected set
// on every render and must stream animals in keyset-paginated chunks with a
// per-chunk preloader — a fixed number of SELECTs per chunk, never per
// animal, and no IN() list larger than the chunk size.
func TestComputeSyncStatusQueryCountBounded(t *testing.T) {
	const animalCount = 60
	for i := 0; i < animalCount; i++ {
		seedPreloaderAnimal(t, 987000+i)
	}

	var mu sync.Mutex
	selectCount := 0
	maxIN := 0
	prevDebug := pop.Debug
	pop.Debug = true
	pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
		if level != logging.SQL {
			return
		}
		mu.Lock()
		defer mu.Unlock()
		if strings.Contains(strings.ToUpper(s), "SELECT") {
			selectCount++
			if n := strings.Count(s, "?"); n > maxIN {
				maxIN = n
			}
		}
	})
	defer func() {
		pop.Debug = prevDebug
		pop.SetTxLogger(func(level logging.Level, anon interface{}, s string, args ...interface{}) {
			if !pop.Debug && level <= logging.Debug {
				return
			}
			if level == logging.SQL {
				stdlog.Printf("[POP] sql - %s", s)
				return
			}
			stdlog.Printf("[POP] "+s, args...)
		})
	}()

	status, err := ComputeSyncStatus(models.DB, preloaderInstance)
	require.NoError(t, err)
	require.Equal(t, animalCount, status.ExpectedTotal)

	mu.Lock()
	count := selectCount
	in := maxIN
	mu.Unlock()

	// 60 animals = 1 chunk: 1 state-event scan + 1 chunk SELECT + a fixed
	// preloader batch. Old behaviour: one full-table EagerPreload with 8
	// association queries whose IN() listed every animal id, then per-animal
	// lookups. The generous headroom absorbs reference-table lookups; the
	// point is no per-animal scaling.
	assert.Less(t, count, animalCount+80,
		"sync status issued %d SELECTs for %d animals — per-record SELECT N+1 is back", count, animalCount)
	assert.Less(t, in, 2*resyncChunkSize,
		"largest placeholder list (%d) exceeds 2× chunk size — unbounded IN() is back", in)
	t.Logf("sync status over %d animals issued %d SELECTs, max placeholder list %d", animalCount, count, in)
}

// TestResyncChunkLoaderEquivalence proves the keyset-paginated LEFT JOIN
// chunk loader rebuilds the exact same models.Animal graph as the old
// EagerPreload path — with AND without an outtake (LEFT JOIN NULLs must
// scan as zero associations, not fail). Payload hashes built from both
// graphs must be identical, or resync/sync-status checksums would diverge
// from the events already delivered.
func TestResyncChunkLoaderEquivalence(t *testing.T) {
	seedPreloaderAnimal(t, 988001) // full association graph incl. outtake
	seedSyncStatusFixture(t)       // 985100-985103: in_care, no outtake
	ids := []int{988001, 985100, 985101, 985102, 985103}

	chunked := map[int]*models.Animal{}
	afterID := 0
	for {
		chunk, nextID, err := loadResyncAnimalChunk(models.DB, afterID)
		require.NoError(t, err)
		if len(*chunk) == 0 {
			break
		}
		afterID = nextID
		for i := range *chunk {
			a := &(*chunk)[i]
			for _, id := range ids {
				if a.ID == id {
					chunked[id] = a
				}
			}
		}
	}
	require.Len(t, chunked, len(ids), "chunk loader must return every seeded animal")

	for _, id := range ids {
		reference := &models.Animal{}
		require.NoError(t, models.DB.EagerPreload(
			"Animalage", "Animaltype", "Intake",
			"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
			"Outtake", "Outtake.Type",
		).Find(reference, id))

		chunkAnimal := chunked[id]
		// Normalize non-payload noise: pop sets no CreatedAt/UpdatedAt layout
		// differences here, but the payload Timestamp always differs.
		preRef := loadedPreloader(models.DB, &models.Animals{*reference})
		preChunk := loadedPreloader(models.DB, &models.Animals{*chunkAnimal})
		pRef := buildEventPayloadInto(models.DB, preRef, reference)
		pChunk := buildEventPayloadInto(models.DB, preChunk, chunkAnimal)
		require.NotNil(t, pRef, "reference payload for animal %d", id)
		require.NotNil(t, pChunk, "chunk payload for animal %d", id)
		pRef.Timestamp = ""
		pChunk.Timestamp = ""
		assert.Equal(t, pRef, pChunk, "payload diverges for animal %d (outtake=%q)", id, pRef.Outtake.Type)
		assert.Equal(t,
			StateContentHashPayload(preloaderInstance, *pRef),
			StateContentHashPayload(preloaderInstance, *pChunk),
			"content hash diverges for animal %d", id)
	}
}
