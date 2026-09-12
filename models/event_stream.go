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
	// EventTypeAnimalDeleted is emitted when an animal record is destroyed
	// (marked erroneous via the error outtake type). The console removes the
	// animal from the consolidated view on this event.
	EventTypeAnimalDeleted EventType = "animal_deleted"
	EventTypeAnimalState   EventType = "animal_state"
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
	// AcknowledgedAt is set when the console confirmed it PROCESSED AND
	// STORED this event's state (echoed state hash in the webhook response).
	// Delivery (HTTP 200) alone does not prove persistence; acknowledgements
	// feed the "Delivered & current" count on /webhook_resync.
	AcknowledgedAt *time.Time `json:"acknowledged_at" db:"acknowledged_at"`
	ContentHash    *string    `json:"content_hash" db:"content_hash"`
	ResyncRunID    *uuid.UUID `json:"resync_run_id" db:"resync_run_id"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at" db:"updated_at"`
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

	// Species taxonomy (canonical French values from the species table,
	// joined on species.creaves_species = animals.species; empty when unknown)
	SpeciesClass        string `json:"species_class,omitempty"`
	SpeciesAGWGroup     string `json:"species_agw_group,omitempty"`
	SpeciesSubsideGroup string `json:"species_subside_group,omitempty"`
	SpeciesNativeStatus string `json:"species_native_status,omitempty"`
	// Family/Order/Game/Huntable complete the taxonomy (bugs.md item 3) so the
	// console can run the species/family/order/game/huntable register exports.
	SpeciesFamily   string `json:"species_family,omitempty"`
	SpeciesOrder    string `json:"species_order,omitempty"`
	SpeciesGame     bool   `json:"species_game,omitempty"`
	SpeciesHuntable bool   `json:"species_huntable,omitempty"`
}

// DiscoveryPayload represents the complete discovery information in an event
type DiscoveryPayload struct {
	ID               string `json:"id,omitempty"`
	Location         string `json:"location,omitempty"`
	PostalCode       string `json:"postal_code,omitempty"`
	City             string `json:"city,omitempty"`
	Date             string `json:"date,omitempty"`
	EntryCauseID     string `json:"entry_cause_id,omitempty"`
	EntryCause       string `json:"entry_cause,omitempty"`
	EntryCauseDetail string `json:"entry_cause_detail,omitempty"`
	EntryCauseNature string `json:"entry_cause_nature,omitempty"`
	Reason           string `json:"reason,omitempty"`
	Note             string `json:"note,omitempty"`
	ReturnHabitat    bool   `json:"return_habitat,omitempty"`
	InGarden         bool   `json:"in_garden,omitempty"`
	// Resolved locality reference data (localities table, joined on
	// city = locality) for the stat_communes export hierarchy. Empty when
	// the city is unknown to the reference table.
	LocalityCommune      string `json:"locality_commune,omitempty"`
	LocalityProvince     string `json:"locality_province,omitempty"`
	LocalityRegion       string `json:"locality_region,omitempty"`
	LocalityCountry      string `json:"locality_country,omitempty"`
	LocalityCantonnement string `json:"locality_cantonnement,omitempty"`
	LocalityDirection    string `json:"locality_direction,omitempty"`
	// Discoverer information
	DiscovererFirstname  string `json:"discoverer_firstname,omitempty"`
	DiscovererLastname   string `json:"discoverer_lastname,omitempty"`
	DiscovererAddress    string `json:"discoverer_address,omitempty"`
	DiscovererCity       string `json:"discoverer_city,omitempty"`
	DiscovererPostalCode string `json:"discoverer_postal_code,omitempty"`
	DiscovererCountry    string `json:"discoverer_country,omitempty"`
	DiscovererEmail      string `json:"discoverer_email,omitempty"`
	DiscovererPhone      string `json:"discoverer_phone,omitempty"`
	DiscovererNote       string `json:"discoverer_note,omitempty"`
	// Donation amount (free-form, e.g. "10,00") for the donation register.
	DiscovererDonation   string `json:"discoverer_donation,omitempty"`
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
	// Rating, Dead and Error come from the outtake type definition. They are
	// always serialized (no omitempty): a neutral rating (0), dead=false or
	// error=false is a real outcome the console must store, not an absent
	// value.
	Rating int  `json:"rating"`
	Dead   bool `json:"dead"`
	Error  bool `json:"error"`
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

	Translations map[string]map[string]string `json:"translations,omitempty"`
	StateHash    string                       `json:"state_hash,omitempty"`
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
