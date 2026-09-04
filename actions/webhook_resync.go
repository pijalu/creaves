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

// resyncPersistEvery controls how often the resync loop persists run progress
// and re-checks for user cancellation. Per-animal persistence used to issue an
// UPDATE plus a run re-read SELECT per animal — log noise and avoidable
// round-trips; 25 gives imperceptible status.json granularity at resync scale.
const resyncPersistEvery = 25

// resyncStateIndex is the run-wide set of (animal_id, content_hash) state
// events already in event_streams for one instance. One query replaces the
// per-animal EXISTS check; entries created by the run itself are added in
// memory. Single-writer invariant: StartResync refuses concurrent runs for
// the same instance, and no other code path creates animal_state events.
type resyncStateIndex struct {
	entries map[int]map[string]bool
}

func loadResyncStateIndex(tx *pop.Connection, instanceID string) (*resyncStateIndex, error) {
	rows := []struct {
		AnimalID    int    `db:"animal_id"`
		ContentHash string `db:"content_hash"`
	}{}
	if err := tx.RawQuery(
		"SELECT animal_id, content_hash FROM event_streams WHERE instance_id = ? AND event_type = ? AND content_hash IS NOT NULL",
		instanceID, string(models.EventTypeAnimalState),
	).All(&rows); err != nil {
		return nil, err
	}
	ix := &resyncStateIndex{entries: map[int]map[string]bool{}}
	for i := range rows {
		ix.add(rows[i].AnimalID, rows[i].ContentHash)
	}
	return ix, nil
}

func (ix *resyncStateIndex) has(animalID int, hash string) bool {
	return ix.entries[animalID][hash]
}

func (ix *resyncStateIndex) add(animalID int, hash string) {
	if ix.entries[animalID] == nil {
		ix.entries[animalID] = map[string]bool{}
	}
	ix.entries[animalID][hash] = true
}

// resyncCheckpoint persists run progress and re-checks for user cancellation
// every resyncPersistEvery animals; context cancellation is honoured on every
// call. stop=true means the loop must return immediately.
func resyncCheckpoint(ctx context.Context, tx *pop.Connection, run *models.ResyncRun, processed int) (stop bool, err error) {
	if err := ctx.Err(); err != nil {
		run.Cancel(time.Now())
		_ = tx.Update(run)
		return true, err
	}
	if processed%resyncPersistEvery != 0 {
		return false, nil
	}
	if err := tx.Update(run); err != nil {
		return true, err
	}
	cancelled, err := tx.Where("id = ? AND status = ?", run.ID, "cancelled").Exists(&models.ResyncRun{})
	if err != nil {
		return true, err
	}
	return cancelled, nil
}

// processResyncAnimal builds and enqueues the state event for one animal,
// incrementing the processed counter. Progress persistence is the caller's
// (resyncCheckpoint) job.
func processResyncAnimal(tx *pop.Connection, run *models.ResyncRun, pre *translationPreloader, ix *resyncStateIndex, force bool, animal *models.Animal) error {
	payload := buildEventPayloadInto(tx, pre, animal)
	if payload == nil {
		appendResyncError(run, animal.ID, "failed to build payload")
	} else {
		payload.CurrentStatus = "in_care"
		if animal.Outtake != nil {
			payload.CurrentStatus = "released"
		}
		if err := enqueueResyncStateEvent(tx, run, ix, force, animal.ID, *payload); err != nil {
			return err
		}
	}
	run.AnimalsProcessed++
	return nil
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
	// The payload builder needs nested associations (Outtake.Type,
	// Discovery.EntryCause, ...) or the emitted state events would miss
	// outtake type/rating and entry-cause fields (they were silently NULL on
	// the console side).
	// NB: an explicit list loads ONLY those associations, so every
	// association the payload builder reads must be listed.
	// EagerPreload() (not Eager()): Eager loads per record — 8 extra SELECTs
	// per animal (480 per 60 animals), flooding the SQL log; EagerPreload
	// batches each association into a single `id IN (...)` query.
	if err := tx.EagerPreload(
		"Animalage", "Animaltype", "Intake",
		"Discovery", "Discovery.EntryCause", "Discovery.Discoverer",
		"Outtake", "Outtake.Type",
	).All(animals); err != nil {
		return finishResync(tx, run, err)
	}
	// Run-wide batches: reference translations/species and the existing
	// state-event index replace per-animal SELECTs (the old N+1 flood).
	pre := newTranslationPreloader(tx, animals)
	ix, err := loadResyncStateIndex(tx, run.InstanceID)
	if err != nil {
		return finishResync(tx, run, err)
	}
	for i := range *animals {
		stop, err := resyncCheckpoint(ctx, tx, run, i)
		if err != nil {
			return err
		}
		if stop {
			return nil
		}
		if err := processResyncAnimal(tx, run, pre, ix, force, &(*animals)[i]); err != nil {
			return finishResync(tx, run, err)
		}
		// Keep delivery moving while large resyncs are still producing events.
		signalWebhookWake()
	}
	// Production is only half the job: the run may only report "completed"
	// once every created event was accepted by the console (bug #1: the old
	// code marked the run completed right after enqueuing, so partial
	// webhook accepts silently lost events while the run looked green).
	return completeResyncDelivery(ctx, tx, run)
}

// resyncDeliveryPollDelay and resyncDeliveryMaxStalled bound the delivery
// wait: after MaxStalled consecutive attempts without delivery progress the
// run is marked failed with a per-run diagnostic instead of blocking forever.
// Package vars so tests can tighten them.
var (
	resyncDeliveryPollDelay  = 100 * time.Millisecond
	resyncDeliveryMaxStalled = 5
)

// countResyncRunEvents returns (total, delivered) for the events attributed
// to this run (created or re-queued by it, see enqueueResyncStateEvent).
func countResyncRunEvents(tx *pop.Connection, runID uuid.UUID) (int, int, error) {
	row := struct {
		Total     int `db:"total"`
		Delivered int `db:"delivered"`
	}{}
	if err := tx.RawQuery(
		"SELECT COUNT(*) AS total, "+
			"COALESCE(SUM(CASE WHEN delivered_at IS NOT NULL THEN 1 ELSE 0 END), 0) AS delivered "+
			"FROM event_streams WHERE resync_run_id = ?",
		runID,
	).First(&row); err != nil {
		return 0, 0, err
	}
	return row.Total, row.Delivered, nil
}

// failResyncDelivery marks the run failed and appends the diagnostic to the
// run's structured error list without discarding production errors.
func failResyncDelivery(tx *pop.Connection, run *models.ResyncRun, diagnostic string) error {
	now := time.Now()
	run.Status = "failed"
	run.FinishedAt = &now
	appendResyncError(run, 0, diagnostic)
	return tx.Update(run)
}

// resyncDeliveryCancelled reports whether delivery must stop (context done
// or user cancellation) and persists the cancelled state when so.
func resyncDeliveryCancelled(ctx context.Context, tx *pop.Connection, run *models.ResyncRun) (bool, error) {
	if err := ctx.Err(); err != nil {
		run.Cancel(time.Now())
		_ = tx.Update(run)
		return true, nil
	}
	cancelled, err := tx.Where("id = ? AND status = ?", run.ID, "cancelled").Exists(&models.ResyncRun{})
	if err != nil {
		return false, err
	}
	if cancelled {
		run.Cancel(time.Now())
		_ = tx.Update(run)
	}
	return cancelled, nil
}

// resyncDeliveryPump drives one delivery attempt and re-counts the run's
// delivered events, returning the updated stall counter (0 on progress).
func resyncDeliveryPump(tx *pop.Connection, run *models.ResyncRun, deliveredBefore, stalled int) (int, error) {
	// The background worker shares this duty; driving deliverBatch here keeps
	// the run responsive instead of waiting for the 60s fallback tick.
	// Deliveries are idempotent (console-side upsert), so overlap with the
	// worker is harmless.
	if _, err := deliverBatch(); err != nil {
		log.Printf("resync run %s delivery attempt failed: %v", run.ID, err)
	}
	_, deliveredNow, err := countResyncRunEvents(tx, run.ID)
	if err != nil {
		return stalled, err
	}
	if deliveredNow > deliveredBefore {
		return 0, nil
	}
	return stalled + 1, nil
}

// resyncDeliveryFinished records the delivered/failed counters and reports
// whether the run is done (nothing created or everything delivered), marking
// it completed when so.
func resyncDeliveryFinished(tx *pop.Connection, run *models.ResyncRun, total, delivered int) (bool, error) {
	run.EventsDelivered = delivered
	run.EventsFailed = total - delivered
	if total == 0 || delivered == total {
		run.Complete(time.Now())
		return true, tx.Update(run)
	}
	return false, nil
}

// completeResyncDelivery drives the webhook deliverer until every event of
// the run is accepted by the console (run -> completed) or a bounded number
// of stalled attempts proves delivery impossible (run -> failed with
// diagnostics counting delivered vs failed events). Partial batch accepts
// (e.g. 97/100) are retried automatically: rejected events stay
// delivered_at IS NULL and the next deliverBatch picks them up again.
func completeResyncDelivery(ctx context.Context, tx *pop.Connection, run *models.ResyncRun) error {
	rr := &resyncDeliveryRunner{tx: tx, run: run}
	for {
		done, err := rr.iterate(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}
		time.Sleep(resyncDeliveryPollDelay)
	}
}

// resyncDeliveryRunner carries the mutable state of one run's delivery wait
// (the stall counter) across loop iterations.
type resyncDeliveryRunner struct {
	tx      *pop.Connection
	run     *models.ResyncRun
	stalled int
}

// iterate performs one delivery-wait step (re-check cancellation, count,
// complete, persist progress, pump one batch, enforce the stall bound).
// done=true means the loop must stop; err is a terminal error for the caller.
func (r *resyncDeliveryRunner) iterate(ctx context.Context) (bool, error) {
	cancelled, err := resyncDeliveryCancelled(ctx, r.tx, r.run)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if cancelled {
		return true, nil
	}
	total, delivered, err := countResyncRunEvents(r.tx, r.run.ID)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	finished, err := resyncDeliveryFinished(r.tx, r.run, total, delivered)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if finished {
		return true, nil
	}
	// Persist the live delivered/failed counters so the status.json endpoint
	// and the resync UI show delivery progress.
	if err := r.tx.Update(r.run); err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	r.stalled, err = resyncDeliveryPump(r.tx, r.run, delivered, r.stalled)
	if err != nil {
		return true, finishResync(r.tx, r.run, err)
	}
	if r.stalled >= resyncDeliveryMaxStalled {
		return true, failResyncDelivery(r.tx, r.run, fmt.Sprintf(
			"delivery incomplete: %d of %d events accepted; %d events not delivered after %d stalled attempts",
			delivered, total, total-delivered, r.stalled))
	}
	return false, nil
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
func enqueueResyncStateEvent(tx *pop.Connection, run *models.ResyncRun, ix *resyncStateIndex, force bool, animalID int, payload models.EventPayload) error {
	hash := StateContentHashPayload(run.InstanceID, payload)
	exists := ix.has(animalID, hash)
	if exists && !force {
		run.EventsSkippedUnchanged++
		return nil
	}
	if exists {
		if err := tx.RawQuery(
			// resync_run_id is re-pointed at the current run so the
			// delivery-accounting loop below sees the re-queued event as
			// this run's responsibility (created/failed counting).
			"UPDATE event_streams SET delivered_at = NULL, resync_run_id = ? WHERE instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?",
			run.ID, run.InstanceID, animalID, string(models.EventTypeAnimalState), hash,
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
	ix.add(animalID, hash)
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
