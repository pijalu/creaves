package actions

import (
	"testing"

	"creaves/models"
)

func stateFixture() models.EventPayload {
	return models.EventPayload{
		Animal:        models.AnimalPayload{ID: 42, Year: 2024, YearNumber: 7, Species: "Hérisson", Gender: "female", Cage: "A12", Zone: "Quarantaine", Ring: "R-1", AnimalType: "Mammifère", AnimalAge: "Adulte"},
		CurrentStatus: "in_care",
		Discovery:     models.DiscoveryPayload{Location: "jardin", PostalCode: "75001", City: "Paris", Date: "2024/01/02 10:00", EntryCause: "collision", Reason: "note"},
		Intake:        models.IntakePayload{Date: "2024/01/02", General: "general", Wounds: "wounds", Parasites: "parasites", Remarks: "remarks"},
		Outtake:       models.OuttakePayload{Date: "2024/02/03", Type: "Relâché en nature", Location: "wild"},
		Translations:  map[string]map[string]string{"de": {"species": "Igel"}, "fr": {"species": "Hérisson"}},
	}
}

func TestStateContentHashDeterministicAcrossRuns(t *testing.T) {
	first, second := stateFixture(), stateFixture()
	if got, want := StateContentHashPayload("center-north", first), StateContentHashPayload("center-north", second); got != want {
		t.Fatalf("hash differs across equivalent builds: %s != %s", got, want)
	}
	if len(StateContentHashPayload("center-north", first)) != 64 {
		t.Fatal("content hash must be 64 hexadecimal characters")
	}
}

func TestStateContentHashVolatileFieldsExcluded(t *testing.T) {
	base := stateFixture()
	changed := base
	changed.Timestamp, changed.UserID, changed.UserLogin = "2099-01-01T00:00:00Z", "user-2", "other"
	if StateContentHashPayload("center-north", base) != StateContentHashPayload("center-north", changed) {
		t.Fatal("volatile fields changed content hash")
	}
	changed.Animal.Cage = "B99"
	if StateContentHashPayload("center-north", base) == StateContentHashPayload("center-north", changed) {
		t.Fatal("content change did not change hash")
	}
}

func TestStateEventUUIDStableFormula(t *testing.T) {
	hash := StateContentHashPayload("center-north", stateFixture())
	got := StateEventUUID("center-north", 42, hash)
	want := "7c727980-02bd-5853-ac4b-9d2f7d1ce19c"
	if got.String() != want {
		t.Fatalf("UUID = %s, want golden %s", got, want)
	}
}

func TestSortedTranslationsJSON(t *testing.T) {
	got := SortedTranslationsJSON(map[string]map[string]string{"en-US": {"species": "Hedgehog"}, "de": {"zone": "Z", "species": "Igel"}})
	want := `{"de":{"species":"Igel","zone":"Z"},"en-US":{"species":"Hedgehog"}}`
	if got != want {
		t.Fatalf("got %s want %s", got, want)
	}
}
