package actions

import (
	"creaves/models"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
)

// EventProcessor handles processing events into the consolidated view
type EventProcessor struct {
	tx *pop.Connection
}

// NewEventProcessor creates a new event processor
func NewEventProcessor(tx *pop.Connection) *EventProcessor {
	return &EventProcessor{tx: tx}
}

// ProcessUnprocessedEvents processes all unprocessed events in order
func (ep *EventProcessor) ProcessUnprocessedEvents() (int, error) {
	events := &models.EventStreams{}

	// Fetch all unprocessed events ordered by created_at
	if err := ep.tx.Where("processed_at IS NULL").Order("created_at asc").All(events); err != nil {
		return 0, errors.WithStack(err)
	}

	processedCount := 0
	for _, event := range *events {
		if err := ep.processEvent(&event); err != nil {
			return processedCount, errors.Wrapf(err, "failed to process event %s", event.ID)
		}
		processedCount++
	}

	return processedCount, nil
}

// ProcessAllEvents reprocesses all events (for rebuilding the consolidated view)
func (ep *EventProcessor) ProcessAllEvents() (int, error) {
	// Clear the consolidated view
	if err := ep.tx.RawQuery("DELETE FROM consolidated_animals").Exec(); err != nil {
		return 0, errors.WithStack(err)
	}

	// Reset all processed_at timestamps
	if err := ep.tx.RawQuery("UPDATE event_streams SET processed_at = NULL").Exec(); err != nil {
		return 0, errors.WithStack(err)
	}

	// Process all events
	return ep.ProcessUnprocessedEvents()
}

// ProcessEventsForInstance processes events for a specific instance
func (ep *EventProcessor) ProcessEventsForInstance(instanceID string) (int, error) {
	events := &models.EventStreams{}

	if err := ep.tx.Where("processed_at IS NULL AND instance_id = ?", instanceID).Order("created_at asc").All(events); err != nil {
		return 0, errors.WithStack(err)
	}

	processedCount := 0
	for _, event := range *events {
		if err := ep.processEvent(&event); err != nil {
			return processedCount, errors.Wrapf(err, "failed to process event %s", event.ID)
		}
		processedCount++
	}

	return processedCount, nil
}

// processEvent processes a single event
func (ep *EventProcessor) processEvent(event *models.EventStream) error {
	// Find or create consolidated animal
	consolidated, err := ep.findOrCreateConsolidatedAnimal(event.InstanceID, event.AnimalID)
	if err != nil {
		return err
	}

	// Apply the event
	if err := consolidated.ApplyEvent(*event); err != nil {
		return err
	}

	// Save the consolidated animal
	if err := ep.saveConsolidatedAnimal(consolidated); err != nil {
		return err
	}

	// Mark event as processed
	now := time.Now()
	event.ProcessedAt = &now
	if err := ep.tx.Update(event); err != nil {
		return errors.WithStack(err)
	}

	return nil
}

// findOrCreateConsolidatedAnimal finds an existing consolidated animal or creates a new one
func (ep *EventProcessor) findOrCreateConsolidatedAnimal(instanceID string, animalID int) (*models.ConsolidatedAnimal, error) {
	consolidated := &models.ConsolidatedAnimal{}

	exists, err := ep.tx.Where("instance_id = ? AND animal_id = ?", instanceID, animalID).Exists(consolidated)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	if exists {
		if err := ep.tx.Where("instance_id = ? AND animal_id = ?", instanceID, animalID).First(consolidated); err != nil {
			return nil, errors.WithStack(err)
		}
		return consolidated, nil
	}

	// Create new consolidated animal
	consolidated = &models.ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    instanceID,
		AnimalID:      animalID,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	return consolidated, nil
}

// saveConsolidatedAnimal saves or creates a consolidated animal
func (ep *EventProcessor) saveConsolidatedAnimal(consolidated *models.ConsolidatedAnimal) error {
	exists, err := ep.tx.Where("id = ?", consolidated.ID).Exists(consolidated)
	if err != nil {
		return errors.WithStack(err)
	}

	if exists {
		return ep.tx.Update(consolidated)
	}

	return ep.tx.Create(consolidated)
}

// GetConsolidatedStats returns statistics about the consolidated view
func (ep *EventProcessor) GetConsolidatedStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Total animals
	count, err := ep.tx.Count(&models.ConsolidatedAnimal{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stats["total_animals"] = count

	// By status
	statusCounts := []struct {
		Status string `db:"current_status"`
		Count  int    `db:"count"`
	}{}

	if err := ep.tx.RawQuery("SELECT current_status, COUNT(*) as count FROM consolidated_animals GROUP BY current_status").All(&statusCounts); err != nil {
		return nil, errors.WithStack(err)
	}

	statusMap := make(map[string]int)
	for _, sc := range statusCounts {
		statusMap[sc.Status] = sc.Count
	}
	stats["by_status"] = statusMap

	// By instance
	instanceCounts := []struct {
		InstanceID string `db:"instance_id"`
		Count      int    `db:"count"`
	}{}

	if err := ep.tx.RawQuery("SELECT instance_id, COUNT(*) as count FROM consolidated_animals GROUP BY instance_id").All(&instanceCounts); err != nil {
		return nil, errors.WithStack(err)
	}

	instanceMap := make(map[string]int)
	for _, ic := range instanceCounts {
		instanceMap[ic.InstanceID] = ic.Count
	}
	stats["by_instance"] = instanceMap

	// Unprocessed events
	unprocessedCount, err := ep.tx.Where("processed_at IS NULL").Count(&models.EventStream{})
	if err != nil {
		return nil, errors.WithStack(err)
	}
	stats["unprocessed_events"] = unprocessedCount

	return stats, nil
}

// ProcessEventsBatch processes events in batches with a limit
func (ep *EventProcessor) ProcessEventsBatch(limit int) (int, bool, error) {
	events := &models.EventStreams{}

	if err := ep.tx.Where("processed_at IS NULL").Order("created_at asc").Limit(limit).All(events); err != nil {
		return 0, false, errors.WithStack(err)
	}

	if len(*events) == 0 {
		return 0, true, nil
	}

	processedCount := 0
	for _, event := range *events {
		if err := ep.processEvent(&event); err != nil {
			return processedCount, false, errors.Wrapf(err, "failed to process event %s", event.ID)
		}
		processedCount++
	}

	// Check if there are more events
	remaining, err := ep.tx.Where("processed_at IS NULL").Count(&models.EventStream{})
	if err != nil {
		return processedCount, false, errors.WithStack(err)
	}

	return processedCount, remaining == 0, nil
}
