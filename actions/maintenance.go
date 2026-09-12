package actions

import (
	"creaves/models"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// SnapshotTaskStatus tracks the status of async snapshot operations
type SnapshotTaskStatus struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // "snapshot" or "cleanup"
	Status      string    `json:"status"` // "running", "completed", "failed"
	StartedAt   time.Time `json:"started_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
	Message     string    `json:"message,omitempty"`
	Processed   int       `json:"processed,omitempty"`
	Errors      int       `json:"errors,omitempty"`
}

var (
	snapshotTasks     = make(map[string]*SnapshotTaskStatus)
	snapshotTasksMu   sync.RWMutex
	snapshotTaskQueue = make(chan *SnapshotTaskStatus, 10)
)

func init() {
	// Start the background worker
	go snapshotWorker()
}

// snapshotWorker processes snapshot tasks asynchronously
func snapshotWorker() {
	for task := range snapshotTaskQueue {
		switch task.Type {
		case "snapshot":
			runSnapshotTask(task)
		case "cleanup":
			runCleanupTask(task)
		}
	}
}

// runSnapshotTask creates events for all animals asynchronously.
//
// Batched: the set of animals that already have events is pre-fetched once
// (instead of one EXISTS query per animal), animals are walked in id-keyset
// chunks without eager preloading (Publish*Event re-reads each animal with
// the associations it needs itself — see reloadAnimalForEvent), and the
// outtake dead/released classification is resolved with one eager batch
// query per chunk instead of per-animal Find+Load round trips.
func runSnapshotTask(task *SnapshotTaskStatus) {
	const chunkSize = 500

	task.Status = "running"

	// Use the existing DB connection - it's safe for background goroutines
	tx := models.DB

	fail := func(msg string) {
		task.Status = "failed"
		task.Message = msg
		task.CompletedAt = time.Now()
	}

	// Load config
	if CurrentConfigGet() == nil {
		if _, err := LoadConfig(tx); err != nil {
			fail(fmt.Sprintf("Failed to load config: %v", err))
			return
		}
	}

	if !IsEventStreamEnabled() {
		fail("Event stream is disabled. Enable it in Configuration.")
		return
	}

	// Pre-fetch every animal that already has any event for this instance:
	// one query replaces one EXISTS query per animal.
	hasEvent := map[int]bool{}
	type eventAnimalRow struct {
		AnimalID int `db:"animal_id"`
	}
	eventAnimals := []eventAnimalRow{}
	if err := tx.RawQuery(
		"SELECT DISTINCT animal_id FROM event_streams WHERE instance_id = ?", GetInstanceID(),
	).All(&eventAnimals); err != nil {
		fail(fmt.Sprintf("Failed to load existing event animals: %v", err))
		return
	}
	for _, r := range eventAnimals {
		hasEvent[r.AnimalID] = true
	}

	var total int
	if err := tx.RawQuery("SELECT COUNT(*) FROM animals").First(&total); err != nil {
		// Non-fatal: progress messages just show absolute counts.
		total = 0
	}

	created := 0
	skipped := 0
	errors := 0
	processed := 0
	lastID := 0

	for {
		animals := &models.Animals{}
		if err := tx.Where("id > ?", lastID).Order("id asc").Limit(chunkSize).All(animals); err != nil {
			fail(fmt.Sprintf("Failed to load animals: %v", err))
			return
		}
		if len(*animals) == 0 {
			break
		}
		lastID = (*animals)[len(*animals)-1].ID

		// Batch-load the outtakes (with their types) referenced by this
		// chunk: two queries per chunk instead of two per animal.
		outtakeIDs := make([]uuid.UUID, 0, len(*animals))
		for i := range *animals {
			if (*animals)[i].OuttakeID.Valid {
				outtakeIDs = append(outtakeIDs, (*animals)[i].OuttakeID.UUID)
			}
		}
		outtakes := map[uuid.UUID]*models.Outtake{}
		if len(outtakeIDs) > 0 {
			var ots models.Outtakes
			if err := tx.Eager("Type").Where("id IN (?)", outtakeIDs).All(&ots); err != nil {
				fail(fmt.Sprintf("Failed to load outtakes: %v", err))
				return
			}
			for i := range ots {
				outtakes[ots[i].ID] = &ots[i]
			}
		}

		for i := range *animals {
			animal := &(*animals)[i]
			processed++
			if processed%100 == 0 {
				task.Processed = processed
				if total > 0 {
					task.Message = fmt.Sprintf("Processing %d/%d animals...", processed, total)
				} else {
					task.Message = fmt.Sprintf("Processing %d animals...", processed)
				}
			}

			if hasEvent[animal.ID] {
				skipped++
				continue
			}

			// Create discovery event (re-reads the animal with full
			// associations itself).
			if err := PublishAnimalDiscoveredEvent(tx, animal, nil); err != nil {
				errors++
				continue
			}

			// If animal has outtake, create appropriate outtake event.
			// Missing type behaves like the original Find/Load path: it
			// falls through to the released classification.
			if animal.OuttakeID.Valid {
				outtake := outtakes[animal.OuttakeID.UUID]
				if outtake == nil {
					errors++
					continue
				}
				if outtake.Type.Dead {
					if err := PublishAnimalDiedEvent(tx, animal, nil); err != nil {
						errors++
						continue
					}
				} else {
					if err := PublishAnimalReleasedEvent(tx, animal, nil); err != nil {
						errors++
						continue
					}
				}
			}

			created++
		}
	}

	task.Status = "completed"
	task.Processed = created
	task.Errors = errors
	task.Message = fmt.Sprintf("Created %d events, skipped %d, errors %d", created, skipped, errors)
	task.CompletedAt = time.Now()
}

// runCleanupTask removes delivered events from the event stream
func runCleanupTask(task *SnapshotTaskStatus) {
	task.Status = "running"

	// Use the existing DB connection
	tx := models.DB

	// Count events to be deleted
	var count int
	if err := tx.RawQuery("SELECT COUNT(*) FROM event_streams WHERE delivered_at IS NOT NULL").First(&count); err != nil {
		task.Status = "failed"
		task.Message = fmt.Sprintf("Failed to count events: %v", err)
		task.CompletedAt = time.Now()
		return
	}

	if count == 0 {
		task.Status = "completed"
		task.Message = "No delivered events to clean up"
		task.CompletedAt = time.Now()
		return
	}

	// Delete delivered events
	if err := tx.RawQuery("DELETE FROM event_streams WHERE delivered_at IS NOT NULL").Exec(); err != nil {
		task.Status = "failed"
		task.Message = fmt.Sprintf("Failed to delete events: %v", err)
		task.CompletedAt = time.Now()
		return
	}

	task.Status = "completed"
	task.Processed = count
	task.Message = fmt.Sprintf("Cleaned up %d delivered events", count)
	task.CompletedAt = time.Now()
}

// enqueueSnapshotTask adds a task to the queue and returns its ID
func enqueueSnapshotTask(taskType string) string {
	task := &SnapshotTaskStatus{
		ID:        fmt.Sprintf("%s-%d", taskType, time.Now().UnixNano()),
		Type:      taskType,
		Status:    "queued",
		StartedAt: time.Now(),
	}

	snapshotTasksMu.Lock()
	snapshotTasks[task.ID] = task
	snapshotTasksMu.Unlock()

	// Non-blocking send
	select {
	case snapshotTaskQueue <- task:
	default:
		task.Status = "failed"
		task.Message = "Task queue is full, try again later"
		task.CompletedAt = time.Now()
	}

	return task.ID
}

// getSnapshotTask retrieves a task by ID
func getSnapshotTask(id string) (*SnapshotTaskStatus, bool) {
	snapshotTasksMu.RLock()
	defer snapshotTasksMu.RUnlock()
	task, ok := snapshotTasks[id]
	return task, ok
}

// MaintenanceIndex default implementation.
func MaintenanceIndex(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	// Get event stream stats for display
	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var totalEvents int
	tx.RawQuery("SELECT COUNT(*) FROM event_streams").First(&totalEvents)

	var undeliveredEvents int
	tx.RawQuery("SELECT COUNT(*) FROM event_streams WHERE delivered_at IS NULL").First(&undeliveredEvents)

	var deliveredEvents int
	tx.RawQuery("SELECT COUNT(*) FROM event_streams WHERE delivered_at IS NOT NULL").First(&deliveredEvents)

	// Diagnostic: species in use that have no approved animal type mapping.
	// These animals are saved with their submitted (possibly blank) type and
	// logged as warnings; this list is the follow-up worklist.
	unmappedSpecies, err := unmappedSpeciesDiagnostics(tx)
	if err != nil {
		return err
	}

	c.Set("totalEvents", totalEvents)
	c.Set("undeliveredEvents", undeliveredEvents)
	c.Set("deliveredEvents", deliveredEvents)
	c.Set("eventStreamEnabled", IsEventStreamEnabled())
	c.Set("unmappedSpecies", unmappedSpecies)

	return c.Render(http.StatusOK, r.HTML("maintenance/index.plush.html"))
}

// unmappedSpeciesRow is one entry of the unmapped-species diagnostic: a
// species string used by animals that has no approved animal type mapping,
// with the number of animals referencing it.
type unmappedSpeciesRow struct {
	Species     string `db:"species"`
	AnimalCount int    `db:"animal_count"`
}

// unmappedSpeciesDiagnostics lists species in use that have no approved
// animal type mapping (no species row with a non-null animaltype_id).
func unmappedSpeciesDiagnostics(tx *pop.Connection) ([]unmappedSpeciesRow, error) {
	rows := []unmappedSpeciesRow{}
	if err := tx.RawQuery(`
		SELECT a.species AS species, COUNT(*) AS animal_count
		FROM animals a
		WHERE a.species <> ''
		  AND NOT EXISTS (
			SELECT 1 FROM species s
			WHERE s.creaves_species = a.species AND s.animaltype_id IS NOT NULL
		  )
		GROUP BY a.species
		ORDER BY animal_count DESC, a.species ASC`).All(&rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// MaintenanceRenumber default implementation.
func MaintenanceRenumber(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	if err := tx.RawQuery("update animals set year = null, yearNumber = null").Exec(); err != nil {
		return err
	}
	if err := tx.RawQuery("update animals set year = YEAR(IntakeDate);").Exec(); err != nil {
		return err
	}

	years := []int{}
	if err := tx.RawQuery("SELECT DISTINCT year FROM animals ORDER BY year asc").All(&years); err != nil {
		return err
	}

	for _, year := range years {
		if err := tx.RawQuery("SELECT @i:=0").Exec(); err != nil {
			return err
		}
		if err := tx.RawQuery(`
			UPDATE animals a
			set a.yearNumber = @i:=@i+1 
			where a.year=? 
			order by a.intakeDate asc;`, year).Exec(); err != nil {
			return err
		}

	}

	return c.Render(http.StatusOK, r.HTML("maintenance/renumber.plush.html"))
}

// MaintenanceSnapshot triggers an async snapshot creation
func MaintenanceSnapshot(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	taskID := enqueueSnapshotTask("snapshot")

	c.Flash().Add("info", "Snapshot creation started in background. Check status below.")
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/maintenance?task=%s", taskID))
}

// MaintenanceCleanup triggers cleanup of delivered events
func MaintenanceCleanup(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	taskID := enqueueSnapshotTask("cleanup")

	c.Flash().Add("info", "Event cleanup started in background. Check status below.")
	return c.Redirect(http.StatusSeeOther, fmt.Sprintf("/maintenance?task=%s", taskID))
}

// MaintenanceDeleteAllEvents deletes ALL events from the event stream
func MaintenanceDeleteAllEvents(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	tx, ok := c.Value("tx").(*pop.Connection)
	if !ok {
		return fmt.Errorf("no transaction found")
	}

	var count int
	tx.RawQuery("SELECT COUNT(*) FROM event_streams").First(&count)

	if err := tx.RawQuery("DELETE FROM event_streams").Exec(); err != nil {
		c.Flash().Add("danger", fmt.Sprintf("Failed to delete events: %v", err))
		return c.Redirect(http.StatusSeeOther, "/maintenance")
	}

	c.Flash().Add("success", fmt.Sprintf("Deleted all %d events from the event stream", count))
	return c.Redirect(http.StatusSeeOther, "/maintenance")
}

// MaintenanceTaskStatus returns the status of a background task
func MaintenanceTaskStatus(c buffalo.Context) error {
	cu := GetCurrentUser(c)
	if !cu.Admin {
		return c.Error(http.StatusForbidden, fmt.Errorf("admin rights required for this action"))
	}

	taskID := c.Param("task_id")
	task, ok := getSnapshotTask(taskID)
	if !ok {
		return c.Error(http.StatusNotFound, fmt.Errorf("task not found"))
	}

	return c.Render(http.StatusOK, r.JSON(task))
}
