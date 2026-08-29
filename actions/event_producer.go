package actions

import (
	"creaves/models"
	"fmt"
	"time"

	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// PublishEvent creates an event in the event stream for the given animal and event type
// This should be called within an existing database transaction
func PublishEvent(tx *pop.Connection, eventType string, animal *models.Animal, payload *models.EventPayload, user *models.User) error {
	// Ensure config is loaded
	if CurrentConfig == nil {
		if _, err := LoadConfig(tx); err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}
	}

	// Check if event stream is enabled
	if !IsEventStreamEnabled() {
		return nil
	}

	// Build payload if not provided
	if payload == nil {
		payload = buildEventPayloadWithTranslations(tx, animal)
	}

	// Set timestamp
	if payload.Timestamp == "" {
		payload.Timestamp = time.Now().Format(time.RFC3339)
	}

	// Add user information for audit trail
	if user != nil {
		payload.UserID = user.ID.String()
		payload.UserLogin = user.Login
	}

	event := &models.EventStream{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: GetInstanceID(),
		AnimalID:   animal.ID,
		EventType:  eventType,
	}

	if err := event.SetPayload(*payload); err != nil {
		return fmt.Errorf("failed to set event payload: %w", err)
	}

	if err := tx.Create(event); err != nil {
		return fmt.Errorf("failed to create event: %w", err)
	}

	// Start webhook worker if webhook is enabled and not already running
	EnsureWebhookWorkerRunning()

	return nil
}

// buildEventPayload creates a comprehensive EventPayload from an animal record
func buildEventPayload(animal *models.Animal) *models.EventPayload {
	return buildEventPayloadWithTranslations(nil, animal)
}

// buildEventPayloadWithTranslations builds canonical payload and optionally enriches translations.
func buildEventPayloadWithTranslations(tx *pop.Connection, animal *models.Animal) *models.EventPayload {
	payload := &models.EventPayload{
		Timestamp: time.Now().Format(time.RFC3339),
	}

	// Build animal information
	payload.Animal = models.AnimalPayload{
		ID:         animal.ID,
		Year:       animal.Year,
		YearNumber: animal.YearNumber,
		Species:    animal.Species,
	}

	if animal.Gender.Valid {
		payload.Animal.Gender = animal.Gender.String
	}
	if animal.Cage.Valid {
		payload.Animal.Cage = animal.Cage.String
	}
	if animal.Zone.Valid {
		payload.Animal.Zone = animal.Zone.String
	}
	if animal.Ring.Valid {
		payload.Animal.Ring = animal.Ring.String
	}

	// Animal type
	if animal.Animaltype.ID != uuid.Nil {
		payload.Animal.AnimalType = animal.Animaltype.Name
	}

	// Animal age
	if animal.Animalage.ID != uuid.Nil {
		payload.Animal.AnimalAge = animal.Animalage.Name
	}

	// Species taxonomy from the species table (canonical French values).
	// Joined on species.creaves_species = animals.species; unknown species → fields stay empty.
	if tx != nil && animal.Species != "" {
		species := &models.Species{}
		if err := tx.Where("creaves_species = ?", animal.Species).First(species); err == nil {
			payload.Animal.SpeciesClass = species.Class
			payload.Animal.SpeciesAGWGroup = species.AgwGroup
			payload.Animal.SpeciesSubsideGroup = species.SubsideGroup
			payload.Animal.SpeciesNativeStatus = species.NativeStatus
		}
	}

	// Build discovery details
	if animal.Discovery.ID != uuid.Nil {
		payload.Discovery = models.DiscoveryPayload{
			ID:            animal.Discovery.ID.String(),
			ReturnHabitat: animal.Discovery.ReturnHabitat,
			InGarden:      animal.Discovery.InGarden,
		}

		if animal.Discovery.Location.Valid {
			payload.Discovery.Location = animal.Discovery.Location.String
		}
		if animal.Discovery.PostalCode.Valid {
			payload.Discovery.PostalCode = animal.Discovery.PostalCode.String
		}
		if animal.Discovery.City.Valid {
			payload.Discovery.City = animal.Discovery.City.String
		}
		if !animal.Discovery.Date.IsZero() {
			payload.Discovery.Date = animal.Discovery.Date.Format(models.DateTimeFormat)
		}
		if animal.Discovery.EntryCauseID != "" {
			payload.Discovery.EntryCauseID = animal.Discovery.EntryCauseID
			payload.Discovery.EntryCause = animal.Discovery.EntryCause.Fmt(false)
			payload.Discovery.EntryCauseDetail = animal.Discovery.EntryCause.Detail
			payload.Discovery.EntryCauseNature = animal.Discovery.EntryCause.Nature
		}
		if animal.Discovery.Reason.Valid {
			payload.Discovery.Reason = animal.Discovery.Reason.String
		}
		if animal.Discovery.Note.Valid {
			payload.Discovery.Note = animal.Discovery.Note.String
		}

		// Discoverer information
		if animal.Discovery.Discoverer.ID != uuid.Nil {
			if animal.Discovery.Discoverer.Firstname.Valid {
				payload.Discovery.DiscovererFirstname = animal.Discovery.Discoverer.Firstname.String
			}
			if animal.Discovery.Discoverer.Lastname.Valid {
				payload.Discovery.DiscovererLastname = animal.Discovery.Discoverer.Lastname.String
			}
			if animal.Discovery.Discoverer.Address.Valid {
				payload.Discovery.DiscovererAddress = animal.Discovery.Discoverer.Address.String
			}
			if animal.Discovery.Discoverer.City.Valid {
				payload.Discovery.DiscovererCity = animal.Discovery.Discoverer.City.String
			}
			if animal.Discovery.Discoverer.PostalCode.Valid {
				payload.Discovery.DiscovererPostalCode = animal.Discovery.Discoverer.PostalCode.String
			}
			if animal.Discovery.Discoverer.Country.Valid {
				payload.Discovery.DiscovererCountry = animal.Discovery.Discoverer.Country.String
			}
			if animal.Discovery.Discoverer.Email.Valid {
				payload.Discovery.DiscovererEmail = animal.Discovery.Discoverer.Email.String
			}
			if animal.Discovery.Discoverer.Phone.Valid {
				payload.Discovery.DiscovererPhone = animal.Discovery.Discoverer.Phone.String
			}
			if animal.Discovery.Discoverer.Note.Valid {
				payload.Discovery.DiscovererNote = animal.Discovery.Discoverer.Note.String
			}
		}
	}

	// Build intake details
	if animal.Intake.ID != uuid.Nil {
		payload.Intake = models.IntakePayload{
			ID:           animal.Intake.ID.String(),
			HasWounds:    animal.Intake.HasWounds,
			HasParasites: animal.Intake.HasParasites,
		}

		if !animal.Intake.Date.IsZero() {
			payload.Intake.Date = animal.Intake.Date.Format(models.DateTimeFormat)
		}
		if animal.Intake.General.Valid {
			payload.Intake.General = animal.Intake.General.String
		}
		if animal.Intake.Wounds.Valid {
			payload.Intake.Wounds = animal.Intake.Wounds.String
		}
		if animal.Intake.Parasites.Valid {
			payload.Intake.Parasites = animal.Intake.Parasites.String
		}
		if animal.Intake.Remarks.Valid {
			payload.Intake.Remarks = animal.Intake.Remarks.String
		}
	}

	// Build outtake details (if present)
	if animal.Outtake != nil {
		payload.Outtake = models.OuttakePayload{
			ID: animal.Outtake.ID.String(),
		}

		if !animal.Outtake.Date.IsZero() {
			payload.Outtake.Date = animal.Outtake.Date.Format(models.DateTimeFormat)
		}
		if animal.Outtake.Type.ID != uuid.Nil {
			payload.Outtake.Type = animal.Outtake.Type.Name
			payload.Outtake.TypeID = animal.Outtake.Type.ID.String()
			payload.Outtake.Rating = animal.Outtake.Type.Rating
			payload.Outtake.Dead = animal.Outtake.Type.Dead
		}
		if animal.Outtake.Location.Valid {
			payload.Outtake.Location = animal.Outtake.Location.String
		}
		if animal.Outtake.Note.Valid {
			payload.Outtake.Note = animal.Outtake.Note.String
		}
	}

	if tx != nil {
		payload.Translations = loadPayloadTranslations(tx, animal, payload)
	}
	return payload
}

// PublishAnimalDiscoveredEvent creates an animal_discovered event
func PublishAnimalDiscoveredEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	payload := buildEventPayload(animal)
	payload.InitialStatus = "in_care"
	payload.CurrentStatus = "in_care"
	return PublishEvent(tx, string(models.EventTypeAnimalDiscovered), animal, payload, user)
}

// PublishAnimalStatusChangedEvent creates an animal_status_changed event
func PublishAnimalStatusChangedEvent(tx *pop.Connection, animal *models.Animal, previousStatus, currentStatus string, user *models.User) error {
	payload := buildEventPayload(animal)
	payload.PreviousStatus = previousStatus
	payload.CurrentStatus = currentStatus
	return PublishEvent(tx, string(models.EventTypeAnimalStatusChanged), animal, payload, user)
}

// PublishAnimalReleasedEvent creates an animal_released event
func PublishAnimalReleasedEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	payload := buildEventPayload(animal)
	payload.CurrentStatus = "released"
	return PublishEvent(tx, string(models.EventTypeAnimalReleased), animal, payload, user)
}

// PublishAnimalDiedEvent creates an animal_died event
func PublishAnimalDiedEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	payload := buildEventPayload(animal)
	payload.CurrentStatus = "died"
	return PublishEvent(tx, string(models.EventTypeAnimalDied), animal, payload, user)
}
