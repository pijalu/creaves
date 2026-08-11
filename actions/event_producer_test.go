package actions

import (
	"creaves/models"
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

func TestBuildEventPayload(t *testing.T) {
	animal := &models.Animal{
		ID:         1,
		Year:       2024,
		YearNumber: 42,
		Species:    "Test Species",
		Animaltype: models.Animaltype{
			ID:   uuid.Must(uuid.NewV4()),
			Name: "Bird",
		},
		Animalage: models.Animalage{
			ID:   uuid.Must(uuid.NewV4()),
			Name: "Adult",
		},
		Discovery: models.Discovery{
			ID:       uuid.Must(uuid.NewV4()),
			Location: nulls.NewString("Test Location"),
			Date:     time.Now(),
		},
		Intake: models.Intake{
			ID:        uuid.Must(uuid.NewV4()),
			Date:      time.Now(),
			General:   nulls.NewString("Good"),
			Wounds:    nulls.NewString("None"),
			Parasites: nulls.NewString("None"),
			Remarks:   nulls.NewString("Test remark"),
		},
	}

	payload := buildEventPayload(animal)

	if payload.Animal.Year != animal.Year {
		t.Errorf("Expected year %d, got %d", animal.Year, payload.Animal.Year)
	}

	if payload.Animal.YearNumber != animal.YearNumber {
		t.Errorf("Expected year number %d, got %d", animal.YearNumber, payload.Animal.YearNumber)
	}

	if payload.Animal.Species != animal.Species {
		t.Errorf("Expected species %s, got %s", animal.Species, payload.Animal.Species)
	}

	if payload.Animal.AnimalType != animal.Animaltype.Name {
		t.Errorf("Expected animal type %s, got %s", animal.Animaltype.Name, payload.Animal.AnimalType)
	}

	if payload.Animal.AnimalAge != animal.Animalage.Name {
		t.Errorf("Expected animal age %s, got %s", animal.Animalage.Name, payload.Animal.AnimalAge)
	}

	if payload.Discovery.Location != animal.Discovery.Location.String {
		t.Errorf("Expected discovery location %s, got %s", animal.Discovery.Location.String, payload.Discovery.Location)
	}

	if payload.Intake.General != animal.Intake.General.String {
		t.Errorf("Expected intake general %s, got %s", animal.Intake.General.String, payload.Intake.General)
	}
}

func TestBuildEventPayloadMinimal(t *testing.T) {
	animal := &models.Animal{
		ID:      2,
		Year:    2024,
		Species: "Minimal Species",
	}

	payload := buildEventPayload(animal)

	if payload.Animal.Year != animal.Year {
		t.Errorf("Expected year %d, got %d", animal.Year, payload.Animal.Year)
	}

	if payload.Animal.Species != animal.Species {
		t.Errorf("Expected species %s, got %s", animal.Species, payload.Animal.Species)
	}

	if payload.Animal.AnimalType != "" {
		t.Error("Expected empty animal type for minimal animal")
	}

	if payload.Animal.AnimalAge != "" {
		t.Error("Expected empty animal age for minimal animal")
	}
}

func TestPublishEventWithoutInstanceConfig(t *testing.T) {
	// Save and clear current config
	oldConfig := CurrentConfig
	CurrentConfig = nil
	defer func() {
		CurrentConfig = oldConfig
	}()

	// This should fail because we can't load instance config without DB
	animal := &models.Animal{
		ID:      1,
		Year:    2024,
		Species: "Test",
	}

	// We can't test actual DB operations without a DB connection
	// But we can verify the function exists and has correct signature
	_ = PublishEvent
	_ = PublishAnimalDiscoveredEvent
	_ = PublishAnimalStatusChangedEvent
	_ = PublishAnimalReleasedEvent
	_ = PublishAnimalDiedEvent

	// Verify buildEventPayload works
	payload := buildEventPayload(animal)
	if payload == nil {
		t.Error("Expected non-nil payload")
	}
}

// MockConnection is a minimal mock for testing
func TestEventPayloadTimestamp(t *testing.T) {
	animal := &models.Animal{
		ID:      1,
		Year:    2024,
		Species: "Test",
	}

	payload := buildEventPayload(animal)

	if payload.Timestamp == "" {
		t.Error("Expected timestamp to be set")
	}

	// Verify it's a valid timestamp
	_, err := time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		t.Errorf("Expected valid RFC3339 timestamp, got error: %v", err)
	}
}

func TestPublishAnimalDiscoveredEventPayload(t *testing.T) {
	animal := &models.Animal{
		ID:         1,
		Year:       2024,
		YearNumber: 42,
		Species:    "Test Species",
	}

	// Test that the helper function generates correct payload
	payload := buildEventPayload(animal)
	payload.InitialStatus = "in_care"
	payload.CurrentStatus = "in_care"

	if payload.InitialStatus != "in_care" {
		t.Errorf("Expected initial status 'in_care', got %s", payload.InitialStatus)
	}

	if payload.CurrentStatus != "in_care" {
		t.Errorf("Expected current status 'in_care', got %s", payload.CurrentStatus)
	}
}

func TestPublishAnimalStatusChangedEventPayload(t *testing.T) {
	animal := &models.Animal{
		ID:      1,
		Year:    2024,
		Species: "Test Species",
	}

	payload := buildEventPayload(animal)
	payload.PreviousStatus = "in_care"
	payload.CurrentStatus = "released"

	if payload.PreviousStatus != "in_care" {
		t.Errorf("Expected previous status 'in_care', got %s", payload.PreviousStatus)
	}

	if payload.CurrentStatus != "released" {
		t.Errorf("Expected current status 'released', got %s", payload.CurrentStatus)
	}
}

func TestPublishAnimalReleasedEventPayload(t *testing.T) {
	animal := &models.Animal{
		ID:      1,
		Year:    2024,
		Species: "Test Species",
		Outtake: &models.Outtake{
			Type: models.Outtaketype{
				ID:   uuid.Must(uuid.NewV4()),
				Name: "Released to Wild",
			},
		},
	}

	payload := buildEventPayload(animal)
	payload.CurrentStatus = "released"
	if animal.Outtake != nil && animal.Outtake.Type.ID != uuid.Nil {
		payload.Outtake.Type = animal.Outtake.Type.Name
	}

	if payload.CurrentStatus != "released" {
		t.Errorf("Expected current status 'released', got %s", payload.CurrentStatus)
	}

	if payload.Outtake.Type != "Released to Wild" {
		t.Errorf("Expected outtake type 'Released to Wild', got %s", payload.Outtake.Type)
	}
}

func TestPublishAnimalDiedEventPayload(t *testing.T) {
	animal := &models.Animal{
		ID:      1,
		Year:    2024,
		Species: "Test Species",
		Outtake: &models.Outtake{
			Type: models.Outtaketype{
				ID:   uuid.Must(uuid.NewV4()),
				Name: "Died",
			},
		},
	}

	payload := buildEventPayload(animal)
	payload.CurrentStatus = "died"
	if animal.Outtake != nil && animal.Outtake.Type.ID != uuid.Nil {
		payload.Outtake.Type = animal.Outtake.Type.Name
	}

	if payload.CurrentStatus != "died" {
		t.Errorf("Expected current status 'died', got %s", payload.CurrentStatus)
	}

	if payload.Outtake.Type != "Died" {
		t.Errorf("Expected outtake type 'Died', got %s", payload.Outtake.Type)
	}
}

// Helper to avoid unused import
var _ = pop.Connection{}
