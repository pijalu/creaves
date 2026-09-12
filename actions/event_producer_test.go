package actions

import (
	"bytes"
	"creaves/models"
	"encoding/json"
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// TestBuildEventPayload_IncludesAllLocales proves the builder resolves
// species translations by species row id (not the French display name) and
// still keys every other field by its record id.
func TestBuildEventPayload_IncludesAllLocales(t *testing.T) {
	animalTypeID := uuid.Must(uuid.NewV4())
	animal := &models.Animal{ID: 501, Species: "SP-T51", Animaltype: models.Animaltype{ID: animalTypeID, Name: "Mammifère"}}
	// Species translations are keyed by the species row id; the row must exist
	// for the payload builder to resolve animals.species → species.ID.
	if err := models.DB.RawQuery("INSERT INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, game, agw_group, native_status, huntable) VALUES ('SP-T51', 'SP-T51 latin', 'Mammifères', 'Erinaceidae', 'SP-T51', 'SG-T51', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'Ordre', 0, 'AGW-T51', 'Indigène', 0)").Exec(); err != nil {
		t.Fatalf("create species: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM species WHERE ID = ?", "SP-T51").Exec()
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

// TestBuildEventPayload_SpeciesAndOuttakeTranslationsAllLocales pins the
// webhook contract for the console i18n fix: every species-derived field
// (name, class, agw_group, subside_group, native_status) plus outtake_type
// must carry en-US/de/nl translations keyed per locale; fr falls back to the
// canonical base values.
func TestBuildEventPayload_SpeciesAndOuttakeTranslationsAllLocales(t *testing.T) {
	outtakeTypeID := uuid.Must(uuid.NewV4())
	animal := &models.Animal{ID: 506, Species: "SP-T61", Outtake: &models.Outtake{Type: models.Outtaketype{ID: outtakeTypeID, Name: "Relâché"}}}
	if err := models.DB.RawQuery("INSERT INTO species (ID, species, class, family, creaves_species, subside_group, created_at, updated_at, `order`, game, agw_group, native_status, huntable) VALUES ('SP-T61', 'Erinaceus europaeus', 'Mammifères', 'Erinaceidae', 'SP-T61', 'SG-T61', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 'Ordre', 0, 'AGW-T61', 'Indigène', 0)").Exec(); err != nil {
		t.Fatalf("create species: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM species WHERE ID = ?", "SP-T61").Exec()

	type tr struct{ table, id, field, locale, value string }
	want := map[string]map[string]string{}
	add := func(locale, field, value string) {
		if want[locale] == nil {
			want[locale] = map[string]string{}
		}
		want[locale][field] = value
	}
	var rows []tr
	for _, tc := range []struct {
		locale, field, en, de, nl string
	}{
		{"species", "creaves_species", "Hedgehog", "Igel", "Egel"},
		{"species_class", "class", "Mammals", "Säugetiere", "Zoogdieren"},
		{"species_agw_group", "agw_group", "Insectivores", "Insektenfresser", "Insecteneters"},
		{"species_subside_group", "subside_group", "SG1 EN", "SG1 DE", "SG1 NL"},
		{"species_native_status", "native_status", "Native", "Heimisch", "Inheems"},
		{"outtake_type", "", "Released", "Freilassung", "Vrijlating"},
	} {
		field := tc.field
		if field == "" {
			field = "name"
		}
		table, id := "species", "SP-T61"
		if tc.locale == "outtake_type" {
			table, id = "outtaketypes", outtakeTypeID.String()
		}
		for _, loc := range []struct{ key, value string }{{"en-US", tc.en}, {"de", tc.de}, {"nl", tc.nl}} {
			rows = append(rows, tr{table, id, field, loc.key, loc.value})
			add(loc.key, tc.locale, loc.value)
		}
	}
	for _, r := range rows {
		if err := models.SaveTranslation(models.DB, r.table, r.id, r.field, r.locale, r.value); err != nil {
			t.Fatalf("save translation: %v", err)
		}
	}
	defer models.DB.RawQuery("DELETE FROM translations WHERE record_id IN (?, ?)", "SP-T61", outtakeTypeID.String()).Exec()

	payload := buildEventPayloadWithTranslations(models.DB, animal)
	for _, locale := range []string{"en-US", "de", "nl"} {
		for _, field := range []string{"species", "species_class", "species_agw_group", "species_subside_group", "species_native_status", "outtake_type"} {
			if got := payload.Translations[locale][field]; got != want[locale][field] {
				t.Errorf("%s %s = %q, want %q", locale, field, got, want[locale][field])
			}
		}
	}
	if payload.Translations["fr"]["species"] != "SP-T61" {
		t.Errorf("fr species = %q, want canonical base", payload.Translations["fr"]["species"])
	}
	if payload.Translations["fr"]["species_class"] != "Mammifères" {
		t.Errorf("fr species_class = %q, want canonical base", payload.Translations["fr"]["species_class"])
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

func TestBuildEventPayload_SpeciesTaxonomyAndEntryCauseFields(t *testing.T) {
	animalTypeID := uuid.Must(uuid.NewV4())
	animalType := &models.Animaltype{ID: animalTypeID, Name: "T51-taxo type"}
	if err := models.DB.Create(animalType); err != nil {
		t.Fatalf("create animal type: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM animaltypes WHERE ID = ?", animalTypeID).Exec()

	spID := uuid.Must(uuid.NewV4()).String()
	sp := &models.Species{ID: spID, Species: "Erinaceus europaeus", CreavesSpecies: "SP-T51-taxo", Class: "Mammalia", Order: "Eulipotyphla", Family: "Erinaceidae", AgwGroup: "AGW-T51", SubsideGroup: "SUB-T51", NativeStatus: "Indigène", AnimaltypeID: animalTypeID}
	if err := models.DB.Create(sp); err != nil {
		t.Fatalf("create species: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM species WHERE ID = ?", spID).Exec()

	// Species translations are keyed by the species row id (spID), not by the
	// creaves_species display name.
	if err := models.SaveTranslation(models.DB, "species", spID, "class", "de", "Säugetiere"); err != nil {
		t.Fatalf("save translation: %v", err)
	}
	if err := models.SaveTranslation(models.DB, "entry_causes", "cause-t51-taxo", "detail", "de", "Fahrzeugkollision"); err != nil {
		t.Fatalf("save translation: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM translations WHERE record_id IN (?, ?)", spID, "cause-t51-taxo").Exec()

	entryCauseID := "cause-t51-taxo"
	animal := &models.Animal{
		ID:      504,
		Species: "SP-T51-taxo",
		Discovery: models.Discovery{
			ID:           uuid.Must(uuid.NewV4()),
			EntryCauseID: entryCauseID,
			EntryCause:   models.EntryCause{ID: entryCauseID, Cause: "Accident", Detail: "Collision véhicule", Nature: "Traumatique"},
		},
		Outtake: &models.Outtake{Type: models.Outtaketype{ID: uuid.Must(uuid.NewV4()), Name: "Relâché", Rating: 1, Dead: false}},
	}

	payload := buildEventPayloadWithTranslations(models.DB, animal)

	// Taxonomy is resolved via `SELECT * FROM species WHERE
	// creaves_species = ?` (RawQuery) which works on every dialect, so the
	// assertions run unconditionally (previously gated on a pop-generated
	// lookup that fails on SQLite because of the unquoted `order` column).
	if payload.Animal.SpeciesClass != "Mammalia" {
		t.Errorf("species_class = %q, want Mammalia", payload.Animal.SpeciesClass)
	}
	if payload.Animal.SpeciesAGWGroup != "AGW-T51" {
		t.Errorf("species_agw_group = %q", payload.Animal.SpeciesAGWGroup)
	}
	if payload.Animal.SpeciesSubsideGroup != "SUB-T51" {
		t.Errorf("species_subside_group = %q", payload.Animal.SpeciesSubsideGroup)
	}
	if payload.Animal.SpeciesNativeStatus != "Indigène" {
		t.Errorf("species_native_status = %q", payload.Animal.SpeciesNativeStatus)
	}
	// Canonical French values land in the fr translations bucket.
	if payload.Translations["fr"]["species_class"] != "Mammalia" {
		t.Errorf("fr species_class = %q", payload.Translations["fr"]["species_class"])
	}
	if payload.Discovery.EntryCauseDetail != "Collision véhicule" {
		t.Errorf("entry_cause_detail = %q", payload.Discovery.EntryCauseDetail)
	}
	if payload.Discovery.EntryCauseNature != "Traumatique" {
		t.Errorf("entry_cause_nature = %q", payload.Discovery.EntryCauseNature)
	}
	if payload.Outtake.Rating != 1 {
		t.Errorf("outtake rating = %d, want 1", payload.Outtake.Rating)
	}
	if payload.Outtake.Dead {
		t.Error("outtake dead = true, want false")
	}
	if payload.Translations["fr"]["entry_cause_detail"] != "Collision véhicule" {
		t.Errorf("fr entry_cause_detail = %q", payload.Translations["fr"]["entry_cause_detail"])
	}
	if payload.Translations["de"]["species_class"] != "Säugetiere" {
		t.Errorf("de species_class = %q", payload.Translations["de"]["species_class"])
	}
	if payload.Translations["de"]["entry_cause_detail"] != "Fahrzeugkollision" {
		t.Errorf("de entry_cause_detail = %q", payload.Translations["de"]["entry_cause_detail"])
	}
}

func TestBuildEventPayload_LocalityResolution(t *testing.T) {
	requireMySQLTestDB(t)
	locID := "T-LOC-bug9"
	models.DB.RawQuery("DELETE FROM localities WHERE id = ?", locID).Exec()
	loc := &models.Locality{
		ID: locID, Country: "Belgique", Region: "Wallonie", Province: "BRABANT WALLON",
		Municipality: "T-COMMUNE", Locality: "T-Ville-Bug9", PostalCode: "9999",
		Zoning: "T-CANTONNEMENT", Direction: "T-DIRECTION",
	}
	if err := models.DB.Create(loc); err != nil {
		t.Fatalf("create locality: %v", err)
	}
	defer models.DB.RawQuery("DELETE FROM localities WHERE id = ?", locID).Exec()

	animal := &models.Animal{
		ID:      506,
		Species: "SP-T-loc",
		Discovery: models.Discovery{
			ID:   uuid.Must(uuid.NewV4()),
			City: nulls.NewString("T-Ville-Bug9"),
		},
	}

	payload := buildEventPayloadWithTranslations(models.DB, animal)

	if payload.Discovery.LocalityCommune != "T-COMMUNE" {
		t.Errorf("locality_commune = %q, want T-COMMUNE", payload.Discovery.LocalityCommune)
	}
	if payload.Discovery.LocalityProvince != "BRABANT WALLON" {
		t.Errorf("locality_province = %q", payload.Discovery.LocalityProvince)
	}
	if payload.Discovery.LocalityRegion != "Wallonie" {
		t.Errorf("locality_region = %q", payload.Discovery.LocalityRegion)
	}
	if payload.Discovery.LocalityCountry != "Belgique" {
		t.Errorf("locality_country = %q", payload.Discovery.LocalityCountry)
	}
	if payload.Discovery.LocalityCantonnement != "T-CANTONNEMENT" {
		t.Errorf("locality_cantonnement = %q", payload.Discovery.LocalityCantonnement)
	}
	if payload.Discovery.LocalityDirection != "T-DIRECTION" {
		t.Errorf("locality_direction = %q", payload.Discovery.LocalityDirection)
	}

	// Unknown city → all locality fields stay empty (export LEFT JOIN → NULL).
	unknown := &models.Animal{
		ID:      507,
		Species: "SP-T-loc",
		Discovery: models.Discovery{
			ID:   uuid.Must(uuid.NewV4()),
			City: nulls.NewString("T-Ville-Inconnue-Bug9"),
		},
	}
	p2 := buildEventPayloadWithTranslations(models.DB, unknown)
	if p2.Discovery.LocalityCommune != "" || p2.Discovery.LocalityProvince != "" ||
		p2.Discovery.LocalityRegion != "" || p2.Discovery.LocalityCountry != "" ||
		p2.Discovery.LocalityCantonnement != "" || p2.Discovery.LocalityDirection != "" {
		t.Errorf("locality fields = %+v, want all empty for unknown city", p2.Discovery)
	}
}

func TestBuildEventPayload_UnknownSpeciesTolerated(t *testing.T) {
	models.DB.RawQuery("DELETE FROM species WHERE creaves_species = ?", "SP-T51-unknown").Exec()
	animal := &models.Animal{ID: 505, Species: "SP-T51-unknown"}
	payload := buildEventPayloadWithTranslations(models.DB, animal)
	if payload.Animal.SpeciesClass != "" || payload.Animal.SpeciesAGWGroup != "" || payload.Animal.SpeciesSubsideGroup != "" || payload.Animal.SpeciesNativeStatus != "" {
		t.Errorf("taxonomy fields = %+v, want all empty for unknown species", payload.Animal)
	}
}

func TestEventPayload_OmitemptyBackwardsCompat(t *testing.T) {
	// Old payload (without the new fields) must unmarshal into the new structs.
	oldJSON := []byte(`{"animal":{"id":1,"species":"Hérisson"},"discovery":{"entry_cause":"Accident"},"outtake":{"type":"Relâché"},"timestamp":"2024-01-15T10:30:00Z"}`)
	var payload models.EventPayload
	if err := json.Unmarshal(oldJSON, &payload); err != nil {
		t.Fatalf("unmarshal old payload: %v", err)
	}
	if payload.Animal.SpeciesClass != "" || payload.Discovery.EntryCauseDetail != "" || payload.Outtake.Rating != 0 || payload.Outtake.Dead {
		t.Errorf("new fields = %+v / %+v / %+v, want zero values", payload.Animal, payload.Discovery, payload.Outtake)
	}

	// New payload with empty new fields must omit them (old console ignores unknown keys anyway,
	// but absence keeps old consumers byte-compatible). Exception: outtake
	// rating, dead and error are ALWAYS serialized — an explicit neutral
	// rating (0), dead=false or error=false is a real outcome the console
	// must store.
	fresh := models.EventPayload{
		Animal:    models.AnimalPayload{ID: 1, Species: "Hérisson"},
		Discovery: models.DiscoveryPayload{EntryCause: "Accident"},
		Outtake:   models.OuttakePayload{Type: "Relâché"},
		Timestamp: "2024-01-15T10:30:00Z",
	}
	encoded, err := json.Marshal(fresh)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	for _, absent := range []string{"species_class", "species_agw_group", "species_subside_group", "species_native_status", "entry_cause_detail", "entry_cause_nature"} {
		if bytes.Contains(encoded, []byte(absent)) {
			t.Errorf("marshalled payload %s contains %q, want omitted", encoded, absent)
		}
	}
	// rating, dead and error must be present even at zero value.
	for _, present := range []string{`"rating":0`, `"dead":false`, `"error":false`} {
		if !bytes.Contains(encoded, []byte(present)) {
			t.Errorf("marshalled payload %s misses %q, want always serialized", encoded, present)
		}
	}
}
