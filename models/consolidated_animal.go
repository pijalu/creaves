package models

import (
	"encoding/json"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gofrs/uuid"
)

// ConsolidatedAnimal represents a unified view of an animal across all instances
type ConsolidatedAnimal struct {
	ID                uuid.UUID    `json:"id" db:"id"`
	InstanceID        string       `json:"instance_id" db:"instance_id"`
	AnimalID          int          `json:"animal_id" db:"animal_id"`
	Year              int          `json:"year" db:"year"`
	YearNumber        int          `json:"year_number" db:"year_number"`
	Species           nulls.String `json:"species" db:"species"`
	Gender            nulls.String `json:"gender" db:"gender"`
	Cage              nulls.String `json:"cage" db:"cage"`
	Zone              nulls.String `json:"zone" db:"zone"`
	Ring              nulls.String `json:"ring" db:"ring"`
	AnimalType        nulls.String `json:"animal_type" db:"animal_type"`
	AnimalAge         nulls.String `json:"animal_age" db:"animal_age"`
	DiscoveryLocation nulls.String `json:"discovery_location" db:"discovery_location"`
	DiscoveryDate     nulls.Time   `json:"discovery_date" db:"discovery_date"`
	DiscoveryCity     nulls.String `json:"discovery_city" db:"discovery_city"`
	DiscoveryPostalCode nulls.String `json:"discovery_postal_code" db:"discovery_postal_code"`
	EntryCause        nulls.String `json:"entry_cause" db:"entry_cause"`
	CurrentStatus     string       `json:"current_status" db:"current_status"`
	IntakeDate        nulls.Time   `json:"intake_date" db:"intake_date"`
	IntakeGeneral     nulls.String `json:"intake_general" db:"intake_general"`
	IntakeWounds      nulls.String `json:"intake_wounds" db:"intake_wounds"`
	IntakeParasites   nulls.String `json:"intake_parasites" db:"intake_parasites"`
	IntakeRemarks     nulls.String `json:"intake_remarks" db:"intake_remarks"`
	OuttakeDate       nulls.Time   `json:"outtake_date" db:"outtake_date"`
	OuttakeType       nulls.String `json:"outtake_type" db:"outtake_type"`
	OuttakeLocation   nulls.String `json:"outtake_location" db:"outtake_location"`
	LastEventAt       time.Time    `json:"last_event_at" db:"last_event_at"`
	EventCount        int          `json:"event_count" db:"event_count"`
	CreatedAt         time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt         time.Time    `json:"updated_at" db:"updated_at"`
}

// String returns the JSON representation
func (c ConsolidatedAnimal) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// ConsolidatedAnimals is a slice of ConsolidatedAnimal
type ConsolidatedAnimals []ConsolidatedAnimal

// String returns the JSON representation
func (c ConsolidatedAnimals) String() string {
	jc, _ := json.Marshal(c)
	return string(jc)
}

// Validate gets run every time you call a "pop.Validate*" method
func (c *ConsolidatedAnimal) Validate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method
func (c *ConsolidatedAnimal) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method
func (c *ConsolidatedAnimal) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	return validate.NewErrors(), nil
}

// UpdateFromPayload updates the consolidated animal from an event payload
func (c *ConsolidatedAnimal) UpdateFromPayload(payload EventPayload, eventType string, eventTime time.Time) {
	// Update animal identification
	if payload.Animal.ID > 0 {
		c.AnimalID = payload.Animal.ID
	}
	if payload.Animal.Year > 0 {
		c.Year = payload.Animal.Year
	}
	if payload.Animal.YearNumber > 0 {
		c.YearNumber = payload.Animal.YearNumber
	}
	if payload.Animal.Species != "" {
		c.Species = nulls.NewString(payload.Animal.Species)
	}
	if payload.Animal.Gender != "" {
		c.Gender = nulls.NewString(payload.Animal.Gender)
	}
	if payload.Animal.Cage != "" {
		c.Cage = nulls.NewString(payload.Animal.Cage)
	}
	if payload.Animal.Zone != "" {
		c.Zone = nulls.NewString(payload.Animal.Zone)
	}
	if payload.Animal.Ring != "" {
		c.Ring = nulls.NewString(payload.Animal.Ring)
	}
	if payload.Animal.AnimalType != "" {
		c.AnimalType = nulls.NewString(payload.Animal.AnimalType)
	}
	if payload.Animal.AnimalAge != "" {
		c.AnimalAge = nulls.NewString(payload.Animal.AnimalAge)
	}

	// Update discovery info
	if payload.Discovery.Location != "" {
		c.DiscoveryLocation = nulls.NewString(payload.Discovery.Location)
	}
	if payload.Discovery.Date != "" {
		if dt, err := time.Parse(DateTimeFormat, payload.Discovery.Date); err == nil {
			c.DiscoveryDate = nulls.NewTime(dt)
		}
	}
	if payload.Discovery.City != "" {
		c.DiscoveryCity = nulls.NewString(payload.Discovery.City)
	}
	if payload.Discovery.PostalCode != "" {
		c.DiscoveryPostalCode = nulls.NewString(payload.Discovery.PostalCode)
	}
	if payload.Discovery.EntryCause != "" {
		c.EntryCause = nulls.NewString(payload.Discovery.EntryCause)
	}

	// Update intake info
	if payload.Intake.Date != "" {
		if dt, err := time.Parse(DateTimeFormat, payload.Intake.Date); err == nil {
			c.IntakeDate = nulls.NewTime(dt)
		}
	}
	if payload.Intake.General != "" {
		c.IntakeGeneral = nulls.NewString(payload.Intake.General)
	}
	if payload.Intake.Wounds != "" {
		c.IntakeWounds = nulls.NewString(payload.Intake.Wounds)
	}
	if payload.Intake.Parasites != "" {
		c.IntakeParasites = nulls.NewString(payload.Intake.Parasites)
	}
	if payload.Intake.Remarks != "" {
		c.IntakeRemarks = nulls.NewString(payload.Intake.Remarks)
	}

	// Update status based on event type
	switch eventType {
	case string(EventTypeAnimalDiscovered):
		c.CurrentStatus = payload.CurrentStatus
		if c.CurrentStatus == "" {
			c.CurrentStatus = "in_care"
		}
	case string(EventTypeAnimalStatusChanged):
		if payload.CurrentStatus != "" {
			c.CurrentStatus = payload.CurrentStatus
		}
	case string(EventTypeAnimalReleased):
		c.CurrentStatus = "released"
	case string(EventTypeAnimalDied):
		c.CurrentStatus = "died"
	}

	// Update outtake info
	if payload.Outtake.Date != "" {
		if dt, err := time.Parse(DateTimeFormat, payload.Outtake.Date); err == nil {
			c.OuttakeDate = nulls.NewTime(dt)
		}
	}
	if payload.Outtake.Type != "" {
		c.OuttakeType = nulls.NewString(payload.Outtake.Type)
	}
	if payload.Outtake.Location != "" {
		c.OuttakeLocation = nulls.NewString(payload.Outtake.Location)
	}

	// Update metadata
	c.LastEventAt = eventTime
	c.EventCount++
}

// ApplyEvent applies an event stream record to update the consolidated view
func (c *ConsolidatedAnimal) ApplyEvent(event EventStream) error {
	payload, err := event.GetPayload()
	if err != nil {
		return err
	}

	c.UpdateFromPayload(payload, event.EventType, event.CreatedAt)
	return nil
}
