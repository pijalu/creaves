package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
)

func TestConsolidatedAnimalString(t *testing.T) {
	ca := ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		Year:          2024,
		YearNumber:    42,
		Species:       nulls.NewString("Test Species"),
		CurrentStatus: "in_care",
		LastEventAt:   time.Now(),
		EventCount:    1,
	}

	s := ca.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConsolidatedAnimalsString(t *testing.T) {
	cas := ConsolidatedAnimals{
		{
			ID:            uuid.Must(uuid.NewV4()),
			InstanceID:    "test-instance",
			AnimalID:      1,
			CurrentStatus: "in_care",
			LastEventAt:   time.Now(),
		},
	}

	s := cas.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConsolidatedAnimalValidate(t *testing.T) {
	ca := ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		CurrentStatus: "in_care",
		LastEventAt:   time.Now(),
	}

	verrs, err := ca.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestConsolidatedAnimalUpdateFromPayload(t *testing.T) {
	ca := &ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	payload := EventPayload{
		Animal: AnimalPayload{
			Year:       2024,
			YearNumber: 42,
			Species:    "Test Species",
			AnimalType: "Bird",
			AnimalAge:  "Adult",
		},
		Discovery: DiscoveryPayload{
			Location: "Test Location",
			Date:     time.Now().Format(DateTimeFormat),
		},
		InitialStatus: "in_care",
		CurrentStatus: "in_care",
		Intake: IntakePayload{
			Date:    time.Now().Format(DateTimeFormat),
			General: "Good",
			Wounds:  "None",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	ca.UpdateFromPayload(payload, string(EventTypeAnimalDiscovered), time.Now())

	if ca.Year != 2024 {
		t.Errorf("Expected year 2024, got %d", ca.Year)
	}

	if ca.YearNumber != 42 {
		t.Errorf("Expected year number 42, got %d", ca.YearNumber)
	}

	if !ca.Species.Valid || ca.Species.String != "Test Species" {
		t.Errorf("Expected species 'Test Species', got %v", ca.Species)
	}

	if !ca.AnimalType.Valid || ca.AnimalType.String != "Bird" {
		t.Errorf("Expected animal type 'Bird', got %v", ca.AnimalType)
	}

	if ca.CurrentStatus != "in_care" {
		t.Errorf("Expected status 'in_care', got %s", ca.CurrentStatus)
	}

	if ca.EventCount != 1 {
		t.Errorf("Expected event count 1, got %d", ca.EventCount)
	}
}

func TestConsolidatedAnimalApplyEvent(t *testing.T) {
	ca := &ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	payload := EventPayload{
		Animal: AnimalPayload{
			Year:       2024,
			YearNumber: 42,
			Species:    "Test Species",
		},
		CurrentStatus: "released",
		Outtake: OuttakePayload{
			Date: time.Now().Format(DateTimeFormat),
			Type: "Released to Wild",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	event := EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		AnimalID:   1,
		EventType:  string(EventTypeAnimalReleased),
	}
	event.SetPayload(payload)

	err := ca.ApplyEvent(event)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if ca.CurrentStatus != "released" {
		t.Errorf("Expected status 'released', got %s", ca.CurrentStatus)
	}

	if !ca.OuttakeType.Valid || ca.OuttakeType.String != "Released to Wild" {
		t.Errorf("Expected outtake type 'Released to Wild', got %v", ca.OuttakeType)
	}

	if ca.EventCount != 1 {
		t.Errorf("Expected event count 1, got %d", ca.EventCount)
	}
}

func TestConsolidatedAnimalUpdateFromPayloadStatusTransitions(t *testing.T) {
	tests := []struct {
		name           string
		eventType      EventType
		payloadStatus  string
		expectedStatus string
	}{
		{"discovered", EventTypeAnimalDiscovered, "in_care", "in_care"},
		{"discovered_default", EventTypeAnimalDiscovered, "", "in_care"},
		{"status_changed", EventTypeAnimalStatusChanged, "released", "released"},
		{"released", EventTypeAnimalReleased, "", "released"},
		{"died", EventTypeAnimalDied, "", "died"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ca := &ConsolidatedAnimal{
				ID:            uuid.Must(uuid.NewV4()),
				InstanceID:    "test-instance",
				AnimalID:      1,
				CurrentStatus: "unknown",
				LastEventAt:   time.Now(),
				EventCount:    0,
			}

			payload := EventPayload{
				CurrentStatus: test.payloadStatus,
				Timestamp:     time.Now().Format(time.RFC3339),
			}

			ca.UpdateFromPayload(payload, string(test.eventType), time.Now())

			if ca.CurrentStatus != test.expectedStatus {
				t.Errorf("Expected status '%s', got '%s'", test.expectedStatus, ca.CurrentStatus)
			}
		})
	}
}

func TestConsolidatedAnimalUpdateFromPayloadDateParsing(t *testing.T) {
	ca := &ConsolidatedAnimal{
		ID:            uuid.Must(uuid.NewV4()),
		InstanceID:    "test-instance",
		AnimalID:      1,
		CurrentStatus: "unknown",
		LastEventAt:   time.Now(),
		EventCount:    0,
	}

	now := time.Now()
	payload := EventPayload{
		Discovery: DiscoveryPayload{
			Date: now.Format(DateTimeFormat),
		},
		Intake: IntakePayload{
			Date: now.Format(DateTimeFormat),
		},
		Outtake: OuttakePayload{
			Date: now.Format(DateTimeFormat),
		},
		Timestamp: now.Format(time.RFC3339),
	}

	ca.UpdateFromPayload(payload, string(EventTypeAnimalDiscovered), time.Now())

	if !ca.DiscoveryDate.Valid {
		t.Error("Expected discovery date to be valid")
	}

	if !ca.IntakeDate.Valid {
		t.Error("Expected intake date to be valid")
	}

	if !ca.OuttakeDate.Valid {
		t.Error("Expected outtake date to be valid")
	}
}
