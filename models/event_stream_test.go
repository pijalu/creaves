package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/gofrs/uuid"
)

func TestEventStreamString(t *testing.T) {
	e := EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		AnimalID:   1,
		EventType:  string(EventTypeAnimalDiscovered),
		Payload:    json.RawMessage(`{"discovery":{"location":"Test Location"}}`),
	}

	s := e.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestEventStreamsString(t *testing.T) {
	es := EventStreams{
		{
			ID:         uuid.Must(uuid.NewV4()),
			InstanceID: "test-instance",
			AnimalID:   1,
			EventType:  string(EventTypeAnimalDiscovered),
		},
	}

	s := es.String()
	if s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestEventStreamSetPayload(t *testing.T) {
	e := EventStream{}
	payload := EventPayload{
		Discovery: DiscoveryPayload{
			Location: "Test Location",
		},
		Animal: AnimalPayload{
			Species: "Test Species",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	err := e.SetPayload(payload)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if len(e.Payload) == 0 {
		t.Error("Expected payload to be set")
	}
}

func TestEventStreamGetPayload(t *testing.T) {
	e := EventStream{
		Payload: json.RawMessage(`{"discovery":{"location":"Test Location"},"animal":{"species":"Test Species"},"timestamp":"2024-01-01T00:00:00Z"}`),
	}

	payload, err := e.GetPayload()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if payload.Discovery.Location != "Test Location" {
		t.Errorf("Expected discovery location 'Test Location', got %s", payload.Discovery.Location)
	}

	if payload.Animal.Species != "Test Species" {
		t.Errorf("Expected species 'Test Species', got %s", payload.Animal.Species)
	}
}

func TestEventStreamGetPayloadEmpty(t *testing.T) {
	e := EventStream{}

	payload, err := e.GetPayload()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if payload.Discovery.Location != "" {
		t.Error("Expected empty payload for empty raw message")
	}
}

func TestEventStreamValidate(t *testing.T) {
	e := EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		AnimalID:   1,
		EventType:  string(EventTypeAnimalDiscovered),
	}

	verrs, err := e.Validate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestEventStreamValidateCreate(t *testing.T) {
	e := EventStream{}

	verrs, err := e.ValidateCreate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestEventStreamValidateUpdate(t *testing.T) {
	e := EventStream{}

	verrs, err := e.ValidateUpdate(nil)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if verrs.HasAny() {
		t.Errorf("Expected no validation errors, got %v", verrs)
	}
}

func TestEventTypes(t *testing.T) {
	tests := []struct {
		eventType EventType
		expected  string
	}{
		{EventTypeAnimalDiscovered, "animal_discovered"},
		{EventTypeAnimalStatusChanged, "animal_status_changed"},
		{EventTypeAnimalReleased, "animal_released"},
		{EventTypeAnimalDied, "animal_died"},
	}

	for _, test := range tests {
		if string(test.eventType) != test.expected {
			t.Errorf("Expected event type %s, got %s", test.expected, test.eventType)
		}
	}
}

func TestEventPayloadWithAnimalDetails(t *testing.T) {
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
		},
		Intake: IntakePayload{
			Date:    "2024-01-01",
			General: "Good",
			Wounds:  "None",
		},
		Timestamp: time.Now().Format(time.RFC3339),
	}

	e := EventStream{}
	err := e.SetPayload(payload)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	retrieved, err := e.GetPayload()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if retrieved.Animal.Year != payload.Animal.Year {
		t.Errorf("Expected year %d, got %d", payload.Animal.Year, retrieved.Animal.Year)
	}

	if retrieved.Animal.YearNumber != payload.Animal.YearNumber {
		t.Errorf("Expected year number %d, got %d", payload.Animal.YearNumber, retrieved.Animal.YearNumber)
	}

	if retrieved.Animal.AnimalType != payload.Animal.AnimalType {
		t.Errorf("Expected animal type %s, got %s", payload.Animal.AnimalType, retrieved.Animal.AnimalType)
	}

	if retrieved.Animal.AnimalAge != payload.Animal.AnimalAge {
		t.Errorf("Expected animal age %s, got %s", payload.Animal.AnimalAge, retrieved.Animal.AnimalAge)
	}

	if retrieved.Intake.Date != payload.Intake.Date {
		t.Errorf("Expected intake date %s, got %s", payload.Intake.Date, retrieved.Intake.Date)
	}

	if retrieved.Intake.General != payload.Intake.General {
		t.Errorf("Expected intake general %s, got %s", payload.Intake.General, retrieved.Intake.General)
	}
}
