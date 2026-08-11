package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// EventType represents the type of event in the event stream
type EventType string

const (
	// EventTypeAnimalDiscovered is emitted when a new animal is discovered
	EventTypeAnimalDiscovered EventType = "animal_discovered"
	// EventTypeAnimalStatusChanged is emitted when an animal's status changes
	EventTypeAnimalStatusChanged EventType = "animal_status_changed"
	// EventTypeAnimalReleased is emitted when an animal is released
	EventTypeAnimalReleased EventType = "animal_released"
	// EventTypeAnimalDied is emitted when an animal dies
	EventTypeAnimalDied EventType = "animal_died"
)

// EventStream represents an event in the event stream for multi-instance consolidation
type EventStream struct {
	ID          uuid.UUID       `json:"id" db:"id"`
	InstanceID  string          `json:"instance_id" db:"instance_id"`
	AnimalID    int             `json:"animal_id" db:"animal_id"`
	EventType   string          `json:"event_type" db:"event_type"`
	Payload     json.RawMessage `json:"payload" db:"payload"`
	ProcessedAt *time.Time      `json:"processed_at" db:"processed_at"`
	DeliveredAt *time.Time      `json:"delivered_at" db:"delivered_at"`
	CreatedAt   time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at" db:"updated_at"`
}

// AnimalPayload represents the complete animal information in an event
type AnimalPayload struct {
	ID         int    `json:"id,omitempty"`
	Year       int    `json:"year,omitempty"`
	YearNumber int    `json:"year_number,omitempty"`
	Species    string `json:"species,omitempty"`
	Gender     string `json:"gender,omitempty"`
	Cage       string `json:"cage,omitempty"`
	Zone       string `json:"zone,omitempty"`
	Ring       string `json:"ring,omitempty"`
	AnimalType string `json:"animal_type,omitempty"`
	AnimalAge  string `json:"animal_age,omitempty"`
}

// DiscoveryPayload represents the complete discovery information in an event
type DiscoveryPayload struct {
	ID            string `json:"id,omitempty"`
	Location      string `json:"location,omitempty"`
	PostalCode    string `json:"postal_code,omitempty"`
	City          string `json:"city,omitempty"`
	Date          string `json:"date,omitempty"`
	EntryCauseID  string `json:"entry_cause_id,omitempty"`
	EntryCause    string `json:"entry_cause,omitempty"`
	Reason        string `json:"reason,omitempty"`
	Note          string `json:"note,omitempty"`
	ReturnHabitat bool   `json:"return_habitat,omitempty"`
	InGarden      bool   `json:"in_garden,omitempty"`
	// Discoverer information
	DiscovererFirstname string `json:"discoverer_firstname,omitempty"`
	DiscovererLastname  string `json:"discoverer_lastname,omitempty"`
	DiscovererAddress   string `json:"discoverer_address,omitempty"`
	DiscovererCity      string `json:"discoverer_city,omitempty"`
	DiscovererPostalCode string `json:"discoverer_postal_code,omitempty"`
	DiscovererCountry   string `json:"discoverer_country,omitempty"`
	DiscovererEmail     string `json:"discoverer_email,omitempty"`
	DiscovererPhone     string `json:"discoverer_phone,omitempty"`
	DiscovererNote      string `json:"discoverer_note,omitempty"`
}

// IntakePayload represents the complete intake information in an event
type IntakePayload struct {
	ID           string `json:"id,omitempty"`
	Date         string `json:"date,omitempty"`
	General      string `json:"general,omitempty"`
	HasWounds    bool   `json:"has_wounds,omitempty"`
	Wounds       string `json:"wounds,omitempty"`
	HasParasites bool   `json:"has_parasites,omitempty"`
	Parasites    string `json:"parasites,omitempty"`
	Remarks      string `json:"remarks,omitempty"`
}

// OuttakePayload represents the complete outtake information in an event
type OuttakePayload struct {
	ID       string `json:"id,omitempty"`
	Date     string `json:"date,omitempty"`
	Type     string `json:"type,omitempty"`
	TypeID   string `json:"type_id,omitempty"`
	Location string `json:"location,omitempty"`
	Note     string `json:"note,omitempty"`
}

// EventPayload represents the complete structured event payload with all entities
type EventPayload struct {
	// Main animal information
	Animal AnimalPayload `json:"animal,omitempty"`

	// Discovery information
	Discovery DiscoveryPayload `json:"discovery,omitempty"`

	// Intake information
	Intake IntakePayload `json:"intake,omitempty"`

	// Outtake information
	Outtake OuttakePayload `json:"outtake,omitempty"`

	// Status information (for animal_status_changed events)
	InitialStatus  string `json:"initial_status,omitempty"`
	CurrentStatus  string `json:"current_status,omitempty"`
	PreviousStatus string `json:"previous_status,omitempty"`

	// Audit trail - user information
	UserID    string `json:"user_id,omitempty"`
	UserName  string `json:"user_name,omitempty"`
	UserLogin string `json:"user_login,omitempty"`

	// Common fields
	Timestamp string `json:"timestamp"`
}

// String returns the JSON representation of the event
func (e EventStream) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

// EventStreams is a slice of EventStream
type EventStreams []EventStream

// String returns the JSON representation of the events
func (e EventStreams) String() string {
	je, _ := json.Marshal(e)
	return string(je)
}

// Validate gets run every time you call a "pop.Validate*" method
func (e *EventStream) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method
func (e *EventStream) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method
func (e *EventStream) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// SetPayload sets the payload from an EventPayload struct
func (e *EventStream) SetPayload(payload EventPayload) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	e.Payload = json.RawMessage(data)
	return nil
}

// GetPayload returns the payload as an EventPayload struct
func (e EventStream) GetPayload() (EventPayload, error) {
	var payload EventPayload
	if len(e.Payload) == 0 {
		return payload, nil
	}
	err := json.Unmarshal(e.Payload, &payload)
	return payload, err
}
