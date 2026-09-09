package actions

import (
	"creaves/models"
	"fmt"
	"strings"
	"time"

	"github.com/gobuffalo/buffalo"
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

	// Wake the worker for near-immediate delivery. Note: this fires before
	// the surrounding request transaction commits, so the worker's drain may
	// not see this row yet; the commit completes within milliseconds and the
	// next signal (or the 60s fallback tick) picks it up.
	signalWebhookWake()

	return nil
}

// eagerEventAssociations is the explicit association list needed for a full
// event payload. An explicit Eager list loads ONLY the listed paths, so nested
// associations must be spelled out (Pop v6 behavior).
var eagerEventAssociations = []string{
	"Animalage",
	"Animaltype",
	"Intake",
	"Discovery",
	"Discovery.EntryCause",
	"Discovery.Discoverer",
	"Outtake",
	"Outtake.Type",
}

// reloadAnimalForEvent re-reads the animal with all associations required for a
// complete payload (v2 fields: animal_age, animal_type, outtake.type,
// entry_cause, translations). Publishing right after a create/update otherwise
// sends partial data because associations are not loaded on the in-memory
// struct.
func reloadAnimalForEvent(tx *pop.Connection, animal *models.Animal) (*models.Animal, error) {
	if tx == nil {
		return animal, nil
	}
	full := &models.Animal{}
	if err := tx.Eager(eagerEventAssociations...).Find(full, animal.ID); err != nil {
		return animal, err
	}
	return full, nil
}

// buildEventPayload creates a comprehensive EventPayload from an animal record
func buildEventPayload(animal *models.Animal) *models.EventPayload {
	return buildEventPayloadInto(nil, nil, animal)
}

// buildEventPayloadWithTranslations builds canonical payload and optionally enriches translations.
func buildEventPayloadWithTranslations(tx *pop.Connection, animal *models.Animal) *models.EventPayload {
	return buildEventPayloadInto(tx, nil, animal)
}

// buildEventPayloadInto is the shared payload builder. When pre is non-nil it
// supplies species taxonomy rows and translations from a run-wide batch
// (translationPreloader) instead of issuing per-animal queries; tx is then
// unused for those lookups. pre != nil requires the animals to come from the
// same set the preloader was built from.
func buildEventPayloadInto(tx *pop.Connection, pre *translationPreloader, animal *models.Animal) *models.EventPayload {
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
	var species *models.Species
	if tx != nil && animal.Species != "" {
		if pre != nil {
			species = pre.speciesFor(animal.Species)
		} else {
			species = &models.Species{}
			// RawQuery instead of pop's generated SELECT: explicit column
			// list with backtick-quoted `order` (works on MySQL and
			// SQLite) and lowercase aliases — pop strict-maps raw query
			// columns to db tags, and the sqlite test schema names the
			// key column "ID".
			if err := tx.RawQuery("SELECT ID AS id, species, creaves_species, class, `order`, family, native_status, agw_group, subside_group, game, huntable, created_at, updated_at FROM species WHERE creaves_species = ?", animal.Species).First(species); err != nil {
				species = nil
			}
		}
		if species != nil {
			payload.Animal.SpeciesClass = species.Class
			payload.Animal.SpeciesAGWGroup = species.AgwGroup
			payload.Animal.SpeciesSubsideGroup = species.SubsideGroup
			payload.Animal.SpeciesNativeStatus = species.NativeStatus
			payload.Animal.SpeciesFamily = species.Family
			payload.Animal.SpeciesOrder = species.Order
			payload.Animal.SpeciesGame = species.Game
			payload.Animal.SpeciesHuntable = species.Huntable
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
			// Resolved locality reference (same join as the stat_communes
			// export: d.city = locality). Unknown city → fields stay empty,
			// mirroring the export's LEFT JOIN producing NULL columns.
			var loc *models.Locality
			if tx != nil && animal.Discovery.City.String != "" {
				if pre != nil {
					loc = pre.localityFor(animal.Discovery.City.String)
				} else {
					l := &models.Locality{}
					if err := tx.Where("locality = ?", animal.Discovery.City.String).First(l); err == nil {
						loc = l
					}
				}
			}
			if loc != nil {
				payload.Discovery.LocalityCommune = loc.Municipality
				payload.Discovery.LocalityProvince = loc.Province
				payload.Discovery.LocalityRegion = loc.Region
				payload.Discovery.LocalityCountry = loc.Country
				payload.Discovery.LocalityCantonnement = loc.Zoning
				payload.Discovery.LocalityDirection = loc.Direction
			}
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
			if animal.Discovery.Discoverer.Donation.Valid {
				payload.Discovery.DiscovererDonation = animal.Discovery.Discoverer.Donation.String
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
			payload.Outtake.Error = animal.Outtake.Type.Error
		}
		if animal.Outtake.Location.Valid {
			payload.Outtake.Location = animal.Outtake.Location.String
		}
		if animal.Outtake.Note.Valid {
			payload.Outtake.Note = animal.Outtake.Note.String
		}
	}

	if pre != nil {
		payload.Translations = loadPayloadTranslationsPreloaded(pre, animal, payload)
	} else if tx != nil {
		payload.Translations = loadPayloadTranslations(tx, animal, payload, species)
	}
	return payload
}

// PublishAnimalStateEvent creates the content-addressed full-state
// (animal_state) event for an animal. It is published after every successful
// animal update and after intake/outtake/discovery sub-resource changes so
// ordinary edits (cage, zone, species, ...) reach the console — not just
// discovered/status/died transitions.
//
// Identity and dedupe are shared with the resync path:
//   - the hash is StateContentHashPayload over the same canonical builder the
//     resync uses (volatile audit fields excluded), so an update and a resync
//     of identical state produce the same hash;
//   - the event UUID is StateEventUUID(instance, animal, hash), so the
//     console's idempotent upsert collapses re-deliveries;
//   - an event with the same (instance, animal, hash) that already exists
//     suppresses the insert — a no-op update (same form re-saved) creates no
//     event;
//   - payload.state_hash carries the hash so the console can no-op
//     consolidation when the snapshot is unchanged.
func PublishAnimalStateEvent(tx *pop.Connection, animalID int, user *models.User) error {
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

	full, err := reloadAnimalForEvent(tx, &models.Animal{ID: animalID})
	if err != nil {
		return fmt.Errorf("failed to reload animal %d for event: %w", animalID, err)
	}
	payload := buildEventPayloadWithTranslations(tx, full)
	if payload == nil {
		return fmt.Errorf("failed to build payload for animal %d", animalID)
	}

	// Derive CurrentStatus exactly like processResyncAnimal so hashes are
	// comparable across the resync and update paths.
	applyCurrentStatus(payload, full)

	// Add user information for the audit trail (not part of the state hash).
	if user != nil {
		payload.UserID = user.ID.String()
		payload.UserLogin = user.Login
	}

	instanceID := GetInstanceID()
	hash := StateContentHashPayload(instanceID, *payload)
	payload.StateHash = hash

	// Content-hash dedupe: the console already holds this state (a no-op
	// update re-saves identical content and creates no event).
	exists, err := tx.Where("instance_id = ? AND animal_id = ? AND event_type = ? AND content_hash = ?",
		instanceID, animalID, string(models.EventTypeAnimalState), hash).Exists(&models.EventStream{})
	if err != nil {
		return fmt.Errorf("failed to check for existing state event: %w", err)
	}
	if exists {
		return nil
	}

	return createAnimalStateEvent(tx, instanceID, animalID, hash, payload)
}

// createAnimalStateEvent inserts the deterministic full-state event and wakes
// the webhook worker. Losing an insert race against a concurrent resync
// enqueueing the identical deterministic event is treated as "already
// published" rather than failing the user's request.
func createAnimalStateEvent(tx *pop.Connection, instanceID string, animalID int, hash string, payload *models.EventPayload) error {
	event := &models.EventStream{
		ID:          StateEventUUID(instanceID, animalID, hash),
		InstanceID:  instanceID,
		AnimalID:    animalID,
		EventType:   string(models.EventTypeAnimalState),
		ContentHash: &hash,
	}
	if err := event.SetPayload(*payload); err != nil {
		return fmt.Errorf("failed to set event payload: %w", err)
	}

	if err := tx.Create(event); err != nil {
		if isDuplicateKeyError(err) {
			return nil
		}
		return fmt.Errorf("failed to create event: %w", err)
	}

	// Start webhook worker if webhook is enabled and not already running, and
	// wake it for near-immediate delivery (same contract as PublishEvent).
	EnsureWebhookWorkerRunning()
	signalWebhookWake()

	return nil
}

// isDuplicateKeyError reports whether err is a storage-layer duplicate-key
// violation (MySQL 1062 "Duplicate entry", SQLite "UNIQUE constraint").
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "Duplicate entry") || strings.Contains(msg, "UNIQUE constraint failed")
}

// publishAnimalStateEventWarn publishes the update-path animal_state event,
// treating failures as non-fatal for the user's request — the same contract
// the other publish call sites in these actions follow. animalID 0 (no
// linked animal) is a no-op.
func publishAnimalStateEventWarn(c buffalo.Context, tx *pop.Connection, animalID int) {
	if animalID == 0 {
		return
	}
	if err := PublishAnimalStateEvent(tx, animalID, GetCurrentUser(c)); err != nil {
		c.Logger().Warnf("Failed to publish animal_state event: %v", err)
	}
}

// PublishAnimalDiscoveredEvent creates an animal_discovered event
func PublishAnimalDiscoveredEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	full, err := reloadAnimalForEvent(tx, animal)
	if err != nil {
		return fmt.Errorf("failed to reload animal %d for event: %w", animal.ID, err)
	}
	payload := buildEventPayloadWithTranslations(tx, full)
	payload.InitialStatus = "in_care"
	payload.CurrentStatus = "in_care"
	return PublishEvent(tx, string(models.EventTypeAnimalDiscovered), animal, payload, user)
}

// PublishAnimalStatusChangedEvent creates an animal_status_changed event
func PublishAnimalStatusChangedEvent(tx *pop.Connection, animal *models.Animal, previousStatus, currentStatus string, user *models.User) error {
	full, err := reloadAnimalForEvent(tx, animal)
	if err != nil {
		return fmt.Errorf("failed to reload animal %d for event: %w", animal.ID, err)
	}
	payload := buildEventPayloadWithTranslations(tx, full)
	payload.PreviousStatus = previousStatus
	payload.CurrentStatus = currentStatus
	return PublishEvent(tx, string(models.EventTypeAnimalStatusChanged), animal, payload, user)
}

// PublishAnimalReleasedEvent creates an animal_released event
func PublishAnimalReleasedEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	full, err := reloadAnimalForEvent(tx, animal)
	if err != nil {
		return fmt.Errorf("failed to reload animal %d for event: %w", animal.ID, err)
	}
	payload := buildEventPayloadWithTranslations(tx, full)
	payload.CurrentStatus = "released"
	return PublishEvent(tx, string(models.EventTypeAnimalReleased), animal, payload, user)
}

// PublishAnimalDiedEvent creates an animal_died event
func PublishAnimalDiedEvent(tx *pop.Connection, animal *models.Animal, user *models.User) error {
	full, err := reloadAnimalForEvent(tx, animal)
	if err != nil {
		return fmt.Errorf("failed to reload animal %d for event: %w", animal.ID, err)
	}
	payload := buildEventPayloadWithTranslations(tx, full)
	payload.CurrentStatus = "died"
	return PublishEvent(tx, string(models.EventTypeAnimalDied), animal, payload, user)
}
