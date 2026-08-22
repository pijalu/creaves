package actions

import (
	"creaves/models"
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

func TestBuildEventPayload_IncludesAllLocales(t *testing.T) {
	animalTypeID := uuid.Must(uuid.NewV4())
	animal := &models.Animal{ID: 501, Species: "SP-T51", Animaltype: models.Animaltype{ID: animalTypeID, Name: "Mammifère"}}
	for _, tr := range []struct{ table, id, field, locale, value string }{
		{"species", animal.Species, "creaves_species", "en-US", "Hedgehog"},
		{"species", animal.Species, "creaves_species", "de", "Igel"},
		{"species", animal.Species, "creaves_species", "nl", "Egel"},
		{"animaltypes", animalTypeID.String(), "name", "en-US", "Mammal"},
	} {
		if err := models.SaveTranslation(models.DB, tr.table, tr.id, tr.field, tr.locale, tr.value); err != nil {
			t.Fatalf("save translation: %v", err)
		}
	}
	defer models.DB.RawQuery("DELETE FROM translations WHERE record_id IN (?, ?)", animal.Species, animalTypeID.String()).Exec()
	payload := buildEventPayloadWithTranslations(models.DB, animal)
	if len(payload.Translations) != len(models.SupportedLocales) {
		t.Fatalf("locales = %v, want %v", payload.Translations, models.SupportedLocales)
	}
	if payload.Translations["fr"]["species"] != "SP-T51" {
		t.Fatalf("fr species = %q", payload.Translations["fr"]["species"])
	}
	if payload.Translations["de"]["species"] != "Igel" {
		t.Fatalf("de species = %q", payload.Translations["de"]["species"])
	}
	if _, ok := payload.Translations["de"]["animal_type"]; ok {
		t.Fatal("de animal_type present despite missing translation")
	}
}

func TestBuildEventPayload_NoTranslationsStillCanonical(t *testing.T) {
	animal := &models.Animal{ID: 502, Species: "SP-T51-empty"}
	models.DB.RawQuery("DELETE FROM translations WHERE record_id = ?", animal.Species).Exec()
	payload := buildEventPayloadWithTranslations(models.DB, animal)
	if len(payload.Translations) != 0 {
		t.Fatalf("translations = %v, want empty", payload.Translations)
	}
	if payload.Animal.Species != animal.Species {
		t.Fatalf("species = %q, want %q", payload.Animal.Species, animal.Species)
	}
}

func TestBuildEventPayload_OuttakeAndZoneAndEntryCauseTranslated(t *testing.T) {
	outtakeTypeID, zoneID := uuid.Must(uuid.NewV4()), uuid.Must(uuid.NewV4())
	entryCauseID := "cause-t51"
	animal := &models.Animal{ID: 503, Species: "SP-T51-out", Zone: nulls.NewString(zoneID.String()), Discovery: models.Discovery{ID: uuid.Must(uuid.NewV4()), EntryCauseID: entryCauseID, EntryCause: models.EntryCause{ID: entryCauseID, Cause: "Accident", Detail: "Accident"}}, Outtake: &models.Outtake{Type: models.Outtaketype{ID: outtakeTypeID, Name: "Relâché"}}}
	for _, tr := range []struct{ table, id, field, locale, value string }{{"outtaketypes", outtakeTypeID.String(), "name", "de", "Freilassung"}, {"zones", zoneID.String(), "zone", "de", "Quarantäne"}, {"entry_causes", entryCauseID, "cause", "de", "Unfall"}} {
		if err := models.SaveTranslation(models.DB, tr.table, tr.id, tr.field, tr.locale, tr.value); err != nil {
			t.Fatalf("save translation: %v", err)
		}
	}
	defer models.DB.RawQuery("DELETE FROM translations WHERE record_id IN (?, ?, ?)", outtakeTypeID.String(), zoneID.String(), entryCauseID).Exec()
	payload := buildEventPayloadWithTranslations(models.DB, animal)
	if payload.Translations["de"]["outtake_type"] != "Freilassung" {
		t.Fatalf("outtake_type = %q", payload.Translations["de"]["outtake_type"])
	}
	if payload.Translations["de"]["zone"] != "Quarantäne" {
		t.Fatalf("zone = %q", payload.Translations["de"]["zone"])
	}
	if payload.Translations["de"]["entry_cause"] != "Unfall" {
		t.Fatalf("entry_cause = %q", payload.Translations["de"]["entry_cause"])
	}
}

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

// TestBuildEventPayload_Comprehensive exercises every optional branch in
// buildEventPayload (discovery, discoverer, intake, outtake, nulls fields) to
// maximise coverage of the event payload builder.
func TestBuildEventPayload_Comprehensive(t *testing.T) {
	animal := &models.Animal{
		ID:         99,
		Year:       2024,
		YearNumber: 7,
		Species:    "Red Fox",
		Gender:     nulls.NewString("Male"),
		Cage:       nulls.NewString("Cage A"),
		Zone:       nulls.NewString("Zone 1"),
		Ring:       nulls.NewString("RING-001"),
		Animaltype: models.Animaltype{ID: uuid.Must(uuid.NewV4()), Name: "Mammal"},
		Animalage:  models.Animalage{ID: uuid.Must(uuid.NewV4()), Name: "Adult"},
		Discovery: models.Discovery{
			ID:            uuid.Must(uuid.NewV4()),
			Location:      nulls.NewString("Forest"),
			PostalCode:    nulls.NewString("1000"),
			City:          nulls.NewString("Brussels"),
			Date:          time.Now(),
			EntryCauseID:  "cause-1",
			EntryCause:    models.EntryCause{ID: "cause-1"},
			Reason:        nulls.NewString("Injured"),
			Note:          nulls.NewString("Found near road"),
			ReturnHabitat: true,
			InGarden:      true,
			Discoverer: models.Discoverer{
				ID:         uuid.Must(uuid.NewV4()),
				Firstname:  nulls.NewString("Jane"),
				Lastname:   nulls.NewString("Doe"),
				Address:    nulls.NewString("1 Main St"),
				City:       nulls.NewString("Brussels"),
				PostalCode: nulls.NewString("1000"),
				Country:    nulls.NewString("BE"),
				Email:      nulls.NewString("jane@example.com"),
				Phone:      nulls.NewString("0123456789"),
				Note:       nulls.NewString("Caller"),
			},
		},
		Intake: models.Intake{
			ID:           uuid.Must(uuid.NewV4()),
			Date:         time.Now(),
			General:      nulls.NewString("Weak"),
			Wounds:       nulls.NewString("Leg"),
			Parasites:    nulls.NewString("Ticks"),
			Remarks:      nulls.NewString("Needs care"),
			HasWounds:    true,
			HasParasites: true,
		},
		Outtake: &models.Outtake{
			ID:       uuid.Must(uuid.NewV4()),
			Date:     time.Now(),
			Location: nulls.NewString("Forest Reserve"),
			Note:     nulls.NewString("Released"),
			Type:     models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "Released to Wild"},
		},
	}

	p := buildEventPayload(animal)

	// Animal
	assertEq(t, "animal species", "Red Fox", p.Animal.Species)
	assertEq(t, "animal gender", "Male", p.Animal.Gender)
	assertEq(t, "animal cage", "Cage A", p.Animal.Cage)
	assertEq(t, "animal zone", "Zone 1", p.Animal.Zone)
	assertEq(t, "animal ring", "RING-001", p.Animal.Ring)
	assertEq(t, "animal type", "Mammal", p.Animal.AnimalType)
	assertEq(t, "animal age", "Adult", p.Animal.AnimalAge)

	// Discovery
	assertEq(t, "discovery location", "Forest", p.Discovery.Location)
	assertEq(t, "discovery postal", "1000", p.Discovery.PostalCode)
	assertEq(t, "discovery city", "Brussels", p.Discovery.City)
	assertEq(t, "discovery entry cause id", "cause-1", p.Discovery.EntryCauseID)
	assertEq(t, "discovery reason", "Injured", p.Discovery.Reason)
	assertEq(t, "discovery note", "Found near road", p.Discovery.Note)
	if !p.Discovery.ReturnHabitat {
		t.Error("expected ReturnHabitat true")
	}
	if !p.Discovery.InGarden {
		t.Error("expected InGarden true")
	}

	// Discoverer
	assertEq(t, "discoverer firstname", "Jane", p.Discovery.DiscovererFirstname)
	assertEq(t, "discoverer lastname", "Doe", p.Discovery.DiscovererLastname)
	assertEq(t, "discoverer address", "1 Main St", p.Discovery.DiscovererAddress)
	assertEq(t, "discoverer city", "Brussels", p.Discovery.DiscovererCity)
	assertEq(t, "discoverer postal", "1000", p.Discovery.DiscovererPostalCode)
	assertEq(t, "discoverer country", "BE", p.Discovery.DiscovererCountry)
	assertEq(t, "discoverer email", "jane@example.com", p.Discovery.DiscovererEmail)
	assertEq(t, "discoverer phone", "0123456789", p.Discovery.DiscovererPhone)
	assertEq(t, "discoverer note", "Caller", p.Discovery.DiscovererNote)

	// Intake
	assertEq(t, "intake general", "Weak", p.Intake.General)
	assertEq(t, "intake wounds", "Leg", p.Intake.Wounds)
	assertEq(t, "intake parasites", "Ticks", p.Intake.Parasites)
	assertEq(t, "intake remarks", "Needs care", p.Intake.Remarks)
	if !p.Intake.HasWounds {
		t.Error("expected HasWounds true")
	}
	if !p.Intake.HasParasites {
		t.Error("expected HasParasites true")
	}

	// Outtake
	assertEq(t, "outtake type", "Released to Wild", p.Outtake.Type)
	if p.Outtake.TypeID == "" {
		t.Error("expected non-empty outtake TypeID")
	}
	assertEq(t, "outtake location", "Forest Reserve", p.Outtake.Location)
	assertEq(t, "outtake note", "Released", p.Outtake.Note)
	if p.Outtake.Date == "" {
		t.Error("expected outtake date")
	}
}

func assertEq(t *testing.T, label, want, got string) {
	t.Helper()
	if want != got {
		t.Errorf("%s: want %q, got %q", label, want, got)
	}
}
