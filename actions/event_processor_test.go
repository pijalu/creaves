package actions

import (
	"creaves/models"
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

func TestEventProcessor(t *testing.T) {
	// We can't test with actual DB without a test database
	// But we can test the helper functions and structures

	// Test that EventProcessor can be instantiated
	// (Actual DB tests would require a test database setup)
	_ = NewEventProcessor
}

func TestConsolidatedAnimalUpdateFromPayloadComprehensive(t *testing.T) {
	ca := &models.ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	// Test discovery event
	discoveryPayload := models.EventPayload{
		Animal: models.AnimalPayload{
			Year:       2024,
			YearNumber: 42,
			Species:    "Red Fox",
			AnimalType: "Mammal",
			AnimalAge:  "Adult",
		},
		Discovery: models.DiscoveryPayload{
			Location: "Forest Road",
			Date:     "2024/01/15 10:30",
		},
		Intake: models.IntakePayload{
			Date:    "2024/01/15 11:00",
			General: "Weak but responsive",
			Wounds:  "Leg injury",
		},
		InitialStatus: "in_care",
		CurrentStatus: "in_care",
		Timestamp:     time.Now().Format(time.RFC3339),
	}

	ca.UpdateFromPayload(discoveryPayload, string(models.EventTypeAnimalDiscovered), time.Now())

	if ca.CurrentStatus != "in_care" {
		t.Errorf("Expected status 'in_care', got '%s'", ca.CurrentStatus)
	}

	if ca.EventCount != 1 {
		t.Errorf("Expected event count 1, got %d", ca.EventCount)
	}

	// Test status change event
	statusPayload := models.EventPayload{
		PreviousStatus: "in_care",
		CurrentStatus:  "released",
		Timestamp:      time.Now().Format(time.RFC3339),
	}

	ca.UpdateFromPayload(statusPayload, string(models.EventTypeAnimalStatusChanged), time.Now())

	if ca.CurrentStatus != "released" {
		t.Errorf("Expected status 'released', got '%s'", ca.CurrentStatus)
	}

	if ca.EventCount != 2 {
		t.Errorf("Expected event count 2, got %d", ca.EventCount)
	}

	// Test outtake event
	outtakePayload := models.EventPayload{
		Outtake: models.OuttakePayload{
			Date:     "2024/03/20 14:00",
			Type:     "Released to Wild",
			Location: "Forest Reserve",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	ca.UpdateFromPayload(outtakePayload, string(models.EventTypeAnimalReleased), time.Now())

	if ca.CurrentStatus != "released" {
		t.Errorf("Expected status 'released', got '%s'", ca.CurrentStatus)
	}

	if !ca.OuttakeDate.Valid {
		t.Error("Expected outtake date to be valid")
	}

	if !ca.OuttakeType.Valid || ca.OuttakeType.String != "Released to Wild" {
		t.Errorf("Expected outtake type 'Released to Wild', got %v", ca.OuttakeType)
	}

	if ca.EventCount != 3 {
		t.Errorf("Expected event count 3, got %d", ca.EventCount)
	}
}

func TestConsolidatedAnimalMultipleEvents(t *testing.T) {
	ca := &models.ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      2,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	events := []struct {
		eventType models.EventType
		payload   models.EventPayload
	}{
		{
			models.EventTypeAnimalDiscovered,
			models.EventPayload{
				Animal: models.AnimalPayload{
					Year:       2024,
					YearNumber: 100,
					Species:    "Hedgehog",
				},
				InitialStatus: "in_care",
				Timestamp:     time.Now().Format(time.RFC3339),
			},
		},
		{
			models.EventTypeAnimalStatusChanged,
			models.EventPayload{
				CurrentStatus: "under_treatment",
				Timestamp:     time.Now().Format(time.RFC3339),
			},
		},
		{
			models.EventTypeAnimalStatusChanged,
			models.EventPayload{
				CurrentStatus: "in_care",
				Timestamp:     time.Now().Format(time.RFC3339),
			},
		},
		{
			models.EventTypeAnimalDied,
			models.EventPayload{
				Outtake: models.OuttakePayload{
					Date: "2024/02/01 09:00",
					Type: "Died",
				},
				Timestamp: time.Now().Format(time.RFC3339),
			},
		},
	}

	for i, event := range events {
		ca.UpdateFromPayload(event.payload, string(event.eventType), time.Now())

		if ca.EventCount != i+1 {
			t.Errorf("After event %d, expected event count %d, got %d", i, i+1, ca.EventCount)
		}
	}

	if ca.CurrentStatus != "died" {
		t.Errorf("Expected final status 'died', got '%s'", ca.CurrentStatus)
	}

	if ca.EventCount != 4 {
		t.Errorf("Expected final event count 4, got %d", ca.EventCount)
	}
}

func TestEventPayloadToConsolidatedAnimal(t *testing.T) {
	// Test that all fields from payload are correctly mapped
	payload := models.EventPayload{
		Animal: models.AnimalPayload{
			Year:       2024,
			YearNumber: 999,
			Species:    "Owl",
			AnimalType: "Bird",
			AnimalAge:  "Juvenile",
		},
		Discovery: models.DiscoveryPayload{
			Location: "Park",
			Date:     "2024/06/01 08:00",
		},
		InitialStatus: "in_care",
		CurrentStatus: "in_care",
		Intake: models.IntakePayload{
			Date:    "2024/06/01 09:00",
			General: "Good",
			Wounds:  "Wing",
			Parasites: "Mites",
			Remarks: "Young",
		},
		Outtake: models.OuttakePayload{
			Date:     "2024/08/01 10:00",
			Type:     "Released",
			Location: "Park",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	ca := &models.ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      3,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	ca.UpdateFromPayload(payload, string(models.EventTypeAnimalDiscovered), time.Now())

	// Verify all fields
	if ca.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", ca.Year)
	}

	if ca.YearNumber != 999 {
		t.Errorf("Expected year number 999, got %d", ca.YearNumber)
	}

	if !ca.Species.Valid || ca.Species.String != "Owl" {
		t.Errorf("Expected species 'Owl', got %v", ca.Species)
	}

	if !ca.AnimalType.Valid || ca.AnimalType.String != "Bird" {
		t.Errorf("Expected animal type 'Bird', got %v", ca.AnimalType)
	}

	if !ca.AnimalAge.Valid || ca.AnimalAge.String != "Juvenile" {
		t.Errorf("Expected animal age 'Juvenile', got %v", ca.AnimalAge)
	}

	if !ca.DiscoveryLocation.Valid || ca.DiscoveryLocation.String != "Park" {
		t.Errorf("Expected discovery location 'Park', got %v", ca.DiscoveryLocation)
	}

	if !ca.IntakeGeneral.Valid || ca.IntakeGeneral.String != "Good" {
		t.Errorf("Expected intake general 'Good', got %v", ca.IntakeGeneral)
	}

	if !ca.IntakeWounds.Valid || ca.IntakeWounds.String != "Wing" {
		t.Errorf("Expected intake wounds 'Wing', got %v", ca.IntakeWounds)
	}

	if !ca.IntakeParasites.Valid || ca.IntakeParasites.String != "Mites" {
		t.Errorf("Expected intake parasites 'Mites', got %v", ca.IntakeParasites)
	}

	if !ca.IntakeRemarks.Valid || ca.IntakeRemarks.String != "Young" {
		t.Errorf("Expected intake remarks 'Young', got %v", ca.IntakeRemarks)
	}

	if !ca.OuttakeType.Valid || ca.OuttakeType.String != "Released" {
		t.Errorf("Expected outtake type 'Released', got %v", ca.OuttakeType)
	}

	if !ca.OuttakeLocation.Valid || ca.OuttakeLocation.String != "Park" {
		t.Errorf("Expected outtake location 'Park', got %v", ca.OuttakeLocation)
	}
}

func TestConsolidatedAnimalNullHandling(t *testing.T) {
	ca := &models.ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      4,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	// Test with minimal payload (many fields empty)
	payload := models.EventPayload{
		Animal: models.AnimalPayload{
			Year:       2024,
			YearNumber: 1,
			Species:    "Unknown",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	ca.UpdateFromPayload(payload, string(models.EventTypeAnimalDiscovered), time.Now())

	// Verify that empty fields don't overwrite with invalid nulls
	if !ca.Species.Valid || ca.Species.String != "Unknown" {
		t.Errorf("Expected species 'Unknown', got %v", ca.Species)
	}

	// These should remain invalid (null) since payload didn't set them
	if ca.AnimalType.Valid {
		t.Error("Expected animal type to be invalid (null)")
	}

	if ca.DiscoveryLocation.Valid {
		t.Error("Expected discovery location to be invalid (null)")
	}

	if ca.IntakeDate.Valid {
		t.Error("Expected intake date to be invalid (null)")
	}
}

// Helper function to verify nulls.String
func verifyNullsString(t *testing.T, name string, expected string, actual nulls.String) {
	if expected == "" {
		if actual.Valid {
			t.Errorf("Expected %s to be null, got '%s'", name, actual.String)
		}
	} else {
		if !actual.Valid {
			t.Errorf("Expected %s to be '%s', got null", name, expected)
		} else if actual.String != expected {
			t.Errorf("Expected %s to be '%s', got '%s'", name, expected, actual.String)
		}
	}
}
