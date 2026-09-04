package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

var resyncStartMu sync.Mutex

// ErrWebhookDisabled is returned by StartResync when webhook forwarding is
// off. Handlers must map it to a clear user-facing message (flash + 200
// with an enable-on-confirm offer) instead of the raw buffalo error trace.
var ErrWebhookDisabled = fmt.Errorf("webhook forwarding is disabled: enable webhook forwarding to run resync")

// StartResync creates one run and schedules its work on a background goroutine.
// With force=true, state events that already exist (same instance/animal/
// content hash) are re-queued for delivery instead of being skipped: this is
// the "full rebuild" path used after the console side purged the instance
// (cleanup). Re-delivered events keep their deterministic UUIDs, so the
// console upsert/dedup keeps the operation idempotent.
func StartResync(tx *pop.Connection, instanceID string, total int, force bool) (*models.ResyncRun, error) {
	resyncStartMu.Lock()
	defer resyncStartMu.Unlock()
	if !IsWebhookEnabled() {
		return nil, ErrWebhookDisabled
	}
	active, err := tx.Where("instance_id = ? AND status = ?", instanceID, "running").Exists(&models.ResyncRun{})
	if err != nil {
		return nil, err
	}
	if active {
		return nil, fmt.Errorf("resync already running")
	}
	animals := &models.Animals{}
	if total == 0 {
		if err := tx.Where("1 = 1").All(animals); err != nil {
			return nil, err
		}
		total = len(*animals)
	}
	now := time.Now()
	run := &models.ResyncRun{ID: uuid.Must(uuid.NewV4()), InstanceID: instanceID, Status: "running", StartedAt: now, TotalAnimals: total}
	// The worker goroutine below reads this row back through models.DB on a
	// DIFFERENT connection. Creating it on the caller's tx would race the
	// request transaction's commit (the goroutine's Find can execute before
	// the row is visible -> "no rows" -> worker exits and the run stays
	// 'running' forever). Persist on models.DB (autocommit) so the row is
	// committed before the goroutine is launched.
	createTx := models.DB
	if createTx == nil {
		createTx = tx
	}
	if err := createTx.Create(run); err != nil {
		return nil, err
	}
	// A resync creates events asynchronously; ensure delivery is available and
	// wake it immediately rather than waiting for the fallback poll interval.
	EnsureWebhookWorkerRunning()
	signalWebhookWake()
	// The worker shares models.DB like any other code path. NOTE: with
	// pop v6.1.0 + pop.Debug (development), every Create/Update leaked one
	// pooled connection via the SQL logger (logger.go store.Transaction()
	// without close) — resync loops then wedged the app (pool deadlock) or
	// exhausted MySQL (Error 1040). Worked around by the SetTxLogger
	// override in models/models.go (fixed upstream in pop v6.1.2); dev
	// pool limits in database.yml guard against any future burst.
	workerTx := models.DB
	if workerTx == nil {
		workerTx = tx
	}
	go func() {
		if err := RunResync(context.Background(), workerTx, run.ID, force); err != nil {
			log.Printf("resync run %s failed: %v", run.ID, err)
		}
	}()
	return run, nil
}

// RunResync enqueues deterministic full-state events and persists progress after each animal.
// With force=true, already-known (instance, animal, hash) events are re-queued
// (delivered_at reset to NULL) so the webhook worker delivers them again;
// the console rebuilds from them. Without force they are counted as skipped.
// resyncAbortCheck re-reads the run row and reports whether the loop must
// stop: either the context was cancelled or the run was cancelled by the
// user (status flips to 'cancelled' from the web UI).
func resyncAbortCheck(ctx context.Context, tx *pop.Connection, run *models.ResyncRun) (stop bool, err error) {
	if err := ctx.Err(); err != nil {
		run.Cancel(time.Now())
		_ = tx.Update(run)
		return true, err
	}
	if err := tx.Where("id = ?", run.ID).First(run); err != nil {
		return true, err
	}
	if run.Status == "cancelled" {
		return true, nil
	}
	return false, nil
}

// processResyncAnimal builds and enqueues the state event for one animal,
// incrementing the processed counter and persisting the run.
func processResyncAnimal(tx *pop.Connection, run *models.ResyncRun, force bool, animal *models.Animal) error {
	payload := buildEventPayloadWithTranslations(tx, animal)
	if payload == nil {
		appendResyncError(run, animal.ID, "failed to build payload")
	} else {
		payload.CurrentStatus = "in_care"
		if animal.Outtake != nil {
			payload.CurrentStatus = "released"
		}
		if err := enqueueResyncStateEvent(tx, run, force, animal.ID, *payload); err != nil {
			return err
		}
	}
	run.AnimalsProcessed++
	return tx.Update(run)
}

func RunResync(ctx context.Context, tx *pop.Connection, runID uuid.UUID, force bool) error {
	run := &models.ResyncRun{}
	if err := tx.Find(run, runID); err != nil {
		// Never leave a run stranded in 'running': mark it failed
		// best-effort (the row may be invisible only to this connection).
		_ = tx.RawQuery(
			"UPDATE resync_runs SET status = 'failed', finished_at = ?, errors = ? WHERE id = ? AND status = 'running'",
			time.Now(), fmt.Sprintf("worker start: %v", err), runID,
		).Exec()
		return err
	}
	animals := &models.Animals{}
	// Eager() alone loads only direct associations; the payload builder also
	// needs nested ones (Outtake.Type, Discovery.EntryCause, ...) or the
	// emitted state events would miss outtake type/rating and entry-cause
	// fields (they were silently NULL on the console side).
	// NB: Eager with an explicit list loads ONLY those associations, so every
	// association the payload builder reads must be listed.
	if err := tx.Eager(
		"Animalage", "Animaltype", "Intake",
		"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
		"Outtake", "Outtake.Type",
	).All(animals); err != nil {
		return finishResync(tx, run, err)
	}
	for i := range *animals {
		stop, err := resyncAbortCheck(ctx, tx, run)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		if err := processResyncAnimal(tx, run, force, &(*animals)[i]); err != nil {
			return finishResync(tx, run, err)
		}
		// Keep delivery moving while large resyncs are still producing events.
		signalWebhookWake()
	}
	run.Complete(time.Now())
	if err := tx.Update(run); err != nil {
		return err
	}
	EnsureWebhookWorkerRunning()
	signalWebhookWake()
	return nil
}

func appendResyncError(run *models.ResyncRun, animalID int, message string) {
	var errors []map[string]interface{}
	if run.Errors != "" {
		_ = json.Unmarshal([]byte(run.Errors), &errors)
	}
	errors = append(errors, map[string]interface{}{"animal_id": animalID, "error": message})
	data, _ := json.Marshal(errors)
	run.Errors = string(data)
}

// enqueueResyncStateEvent creates (or, in force mode, re-queues) the
// deterministic full-state event for one animal.
//   - Unknown (instance, animal, hash): create the event.
//   - Known, force=false: counted as skipped unchanged.
//   - Known, force=true: re-queue by resetting delivered_at so the webhook
//     worker delivers it again; the deterministic UUID keeps the console
//     side idempotent.
func enqueueResyncStateEvent(tx *pop.Connection, run *models.ResyncRun, force bool, animalID int, payload models.EventPayload) error {
	hash := StateContentHashPayload(run.InstanceID, payload)
	var existing models.EventStream
	exists, err := tx.Where("instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?", run.InstanceID, animalID, string(models.EventTypeAnimalState), hash).Exists(&existing)
	if err != nil {
		return err
	}
	if exists && !force {
		run.EventsSkippedUnchanged++
		return nil
	}
	if exists {
		if err := tx.RawQuery(
			"UPDATE event_streams SET delivered_at = NULL WHERE instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?",
			run.InstanceID, animalID, string(models.EventTypeAnimalState), hash,
		).Exec(); err != nil {
			return err
		}
		run.EventsCreated++
		return nil
	}
	id := StateEventUUID(run.InstanceID, animalID, hash)
	event := &models.EventStream{ID: id, InstanceID: run.InstanceID, AnimalID: animalID, EventType: string(models.EventTypeAnimalState), ContentHash: &hash, ResyncRunID: &run.ID}
	if err := event.SetPayload(payload); err != nil {
		appendResyncError(run, animalID, err.Error())
		return nil
	}
	if err := tx.Create(event); err != nil {
		return err
	}
	run.EventsCreated++
	return nil
}

func finishResync(tx *pop.Connection, run *models.ResyncRun, err error) error {
	run.Fail(time.Now(), err)
	_ = tx.Update(run)
	return err
}

func RecoverInterruptedRuns(tx *pop.Connection) error {
	runs := &[]models.ResyncRun{}
	if err := tx.Where("status = ?", "running").All(runs); err != nil {
		return err
	}
	for i := range *runs {
		r := &(*runs)[i]
		r.Fail(time.Now(), fmt.Errorf("interrupted by restart"))
		if err := tx.Update(r); err != nil {
			return err
		}
	}
	return nil
}

func CancelResync(tx *pop.Connection, runID uuid.UUID) error {
	run := &models.ResyncRun{}
	if err := tx.Find(run, runID); err != nil {
		return err
	}
	if run.Status == "running" {
		run.Cancel(time.Now())
		return tx.Update(run)
	}
	return nil
}
