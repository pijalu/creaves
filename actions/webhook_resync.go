package actions

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"creaves/models"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

var resyncStartMu sync.Mutex

// StartResync creates one run and schedules its work on a background goroutine.
func StartResync(tx *pop.Connection, instanceID string, total int) (*models.ResyncRun, error) {
	resyncStartMu.Lock()
	defer resyncStartMu.Unlock()
	if !IsWebhookEnabled() {
		return nil, fmt.Errorf("webhook forwarding is disabled")
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
	if err := tx.Create(run); err != nil {
		return nil, err
	}
	workerTx := models.DB
	if workerTx == nil {
		workerTx = tx
	}
	go func() { _ = RunResync(context.Background(), workerTx, run.ID) }()
	return run, nil
}

// RunResync enqueues deterministic full-state events and persists progress after each animal.
func RunResync(ctx context.Context, tx *pop.Connection, runID uuid.UUID) error {
	run := &models.ResyncRun{}
	if err := tx.Find(run, runID); err != nil {
		return err
	}
	animals := &models.Animals{}
	if err := tx.Eager().All(animals); err != nil {
		return finishResync(tx, run, err)
	}
	for i := range *animals {
		if err := ctx.Err(); err != nil {
			run.Cancel(time.Now())
			_ = tx.Update(run)
			return err
		}
		if err := tx.Where("id = ?", run.ID).First(run); err != nil {
			return err
		}
		if run.Status == "cancelled" {
			return nil
		}
		animal := &(*animals)[i]
		payload := buildEventPayloadWithTranslations(tx, animal)
		if payload == nil {
			appendResyncError(run, animal.ID, "failed to build payload")
		} else {
			payload.CurrentStatus = "in_care"
			if animal.Outtake != nil {
				payload.CurrentStatus = "released"
			}
			hash := StateContentHashPayload(run.InstanceID, *payload)
			var existing models.EventStream
			exists, err := tx.Where("instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?", run.InstanceID, animal.ID, string(models.EventTypeAnimalState), hash).Exists(&existing)
			if err != nil {
				return finishResync(tx, run, err)
			}
			if exists {
				run.EventsSkippedUnchanged++
			} else {
				id := StateEventUUID(run.InstanceID, animal.ID, hash)
				event := &models.EventStream{ID: id, InstanceID: run.InstanceID, AnimalID: animal.ID, EventType: string(models.EventTypeAnimalState), ContentHash: &hash, ResyncRunID: &run.ID}
				if err := event.SetPayload(*payload); err != nil {
					appendResyncError(run, animal.ID, err.Error())
				} else if err := tx.Create(event); err != nil {
					return finishResync(tx, run, err)
				} else {
					run.EventsCreated++
				}
			}
		}
		run.AnimalsProcessed++
		if err := tx.Update(run); err != nil {
			return err
		}
	}
	run.Complete(time.Now())
	if err := tx.Update(run); err != nil {
		return err
	}
	EnsureWebhookWorkerRunning()
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
