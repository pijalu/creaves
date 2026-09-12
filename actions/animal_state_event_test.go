//go:build sqlite
// +build sqlite

package actions

import (
	"testing"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countStateEvents returns the number of animal_state events for one animal.
func countStateEvents(t *testing.T, animalID int) int {
	t.Helper()
	n, err := pusherTestDB.Where("animal_id = ? AND event_type = ?", animalID, string(models.EventTypeAnimalState)).Count(&models.EventStream{})
	require.NoError(t, err)
	return n
}

// latestStateEvent returns the newest animal_state event for one animal.
func latestStateEvent(t *testing.T, animalID int) models.EventStream {
	t.Helper()
	ev := &models.EventStream{}
	require.NoError(t, pusherTestDB.Where("animal_id = ? AND event_type = ?", animalID, string(models.EventTypeAnimalState)).Order("created_at desc").First(ev))
	return *ev
}

// TestPublishAnimalStateEvent_CreatesDeterministicEvent proves the update-path
// state event reuses the resync identity: deterministic UUID, content hash in
// both the column and the payload (the console no-op dedupe keys on
// payload.state_hash).
func TestPublishAnimalStateEvent_CreatesDeterministicEvent(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	seedPublishAnimals(t, 9101)

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9101, nil))
	require.Equal(t, 1, countStateEvents(t, 9101), "one state event should be created")

	ev := latestStateEvent(t, 9101)
	payload, err := ev.GetPayload()
	require.NoError(t, err)

	// Same derivation as the publish path: CurrentStatus is part of the
	// canonical hash content.
	full := &models.Animal{}
	require.NoError(t, pusherTestDB.Eager(eagerEventAssociations...).Find(full, 9101))
	wantPayload := buildEventPayloadWithTranslations(pusherTestDB, full)
	wantPayload.CurrentStatus = "in_care"
	wantHash := StateContentHashPayload("test-instance", *wantPayload)

	require.NotNil(t, ev.ContentHash, "content hash must be stored on the event")
	assert.Equal(t, wantHash, *ev.ContentHash, "content hash column should match the canonical builder")
	assert.Equal(t, wantHash, payload.StateHash, "payload.state_hash must be set (console no-op dedupe)")
	assert.Equal(t, StateEventUUID("test-instance", 9101, wantHash), ev.ID, "deterministic UUID shared with the resync path")
	assert.Nil(t, ev.DeliveredAt, "new event must be undelivered")
	assert.Equal(t, "in_care", payload.CurrentStatus)
}

// TestPublishAnimalStateEvent_NoOpUpdateDedupes proves an unchanged re-publish
// (the no-op update case) creates no second event.
func TestPublishAnimalStateEvent_NoOpUpdateDedupes(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	seedPublishAnimals(t, 9102)

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9102, nil))
	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9102, nil))
	assert.Equal(t, 1, countStateEvents(t, 9102), "identical state must dedupe to one event")

	// A different event type (e.g. a status change) must not suppress the
	// state event dedupe either way: state count stays 1.
	require.NoError(t, PublishAnimalStatusChangedEvent(pusherTestDB, mustAnimal(t, 9102), "in_care", "in_care", nil))
	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9102, nil))
	assert.Equal(t, 1, countStateEvents(t, 9102), "state dedupe unaffected by other event types")
}

// TestPublishAnimalStateEvent_CageChangePublishesNewEvent proves a real edit
// (cage VE12 -> VE99) produces a second event whose payload carries the new
// cage and a different hash.
func TestPublishAnimalStateEvent_CageChangePublishesNewEvent(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	seedPublishAnimals(t, 9103)

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9103, nil))
	first := latestStateEvent(t, 9103)

	require.NoError(t, pusherTestDB.RawQuery("UPDATE animals SET cage = ? WHERE id = ?", "VE99", 9103).Exec())

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9103, nil))
	require.Equal(t, 2, countStateEvents(t, 9103), "changed state must create a new event")

	second := latestStateEvent(t, 9103)
	assert.NotEqual(t, first.ID, second.ID, "different state -> different deterministic UUID")
	assert.NotEqual(t, *first.ContentHash, *second.ContentHash, "different state -> different hash")

	payload, err := second.GetPayload()
	require.NoError(t, err)
	assert.Equal(t, "VE99", payload.Animal.Cage, "payload must carry the edited cage")
	assert.Equal(t, *second.ContentHash, payload.StateHash)
}

// TestPublishAnimalStateEvent_OuttakeSetsReleasedStatus proves the status
// derivation matches the resync path (Outtake != nil -> "released").
func TestPublishAnimalStateEvent_OuttakeSetsReleasedStatus(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	seedPublishAnimals(t, 9104)

	// Give the animal an outtake via the same shape the eager loader produces
	// (outtakes link to animals through animals.outtake_id). The fixture DB
	// has empty reference tables, so seed one outtake type.
	otypeID := "eeeeeeee-5555-5555-5555-555555550001"
	require.NoError(t, pusherTestDB.RawQuery(
		"INSERT INTO outtaketypes (id, name, def, created_at, updated_at, dead, rating, error) VALUES (?, 'Released', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 0, 1, 0)",
		otypeID).Exec())
	t.Cleanup(func() { pusherTestDB.RawQuery("DELETE FROM outtaketypes WHERE id = ?", otypeID).Exec() })
	require.NoError(t, pusherTestDB.RawQuery(
		"INSERT INTO outtakes (id, date, outtaketype_id, created_at, updated_at) VALUES (?, '2025-07-01 10:00:00', ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)",
		"77777777-0000-0000-0000-000000000001", otypeID).Exec())
	t.Cleanup(func() {
		pusherTestDB.RawQuery("DELETE FROM outtakes WHERE id = ?", "77777777-0000-0000-0000-000000000001").Exec()
	})
	require.NoError(t, pusherTestDB.RawQuery("UPDATE animals SET outtake_id = ? WHERE id = ?", "77777777-0000-0000-0000-000000000001", 9104).Exec())

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9104, nil))
	payload, err := latestStateEvent(t, 9104).GetPayload()
	require.NoError(t, err)
	assert.Equal(t, "released", payload.CurrentStatus)
}

// TestPublishAnimalStateEvent_DisabledByConfig proves the event-stream kill
// switch applies to the state event path.
func TestPublishAnimalStateEvent_DisabledByConfig(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	seedPublishAnimals(t, 9105)

	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = false
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))

	require.NoError(t, PublishAnimalStateEvent(pusherTestDB, 9105, nil))
	assert.Equal(t, 0, countStateEvents(t, 9105), "no event when the stream is disabled")
}

// mustAnimal loads an animal row for helpers needing a struct.
func mustAnimal(t *testing.T, id int) *models.Animal {
	t.Helper()
	animal := &models.Animal{}
	require.NoError(t, pusherTestDB.Find(animal, id))
	return animal
}

// TestCanonicalStateHash_IgnoresAuditFields proves two payloads differing only
// in timestamp/user audit fields hash identically (the no-op update guarantee).
func TestCanonicalStateHash_IgnoresAuditFields(t *testing.T) {
	base := buildEventPayload(&models.Animal{ID: 77, Year: 2025, Species: "Fox", Cage: nulls.NewString("VE12")})
	base.CurrentStatus = "in_care"
	other := *base
	other.Timestamp = "2001-01-01T00:00:00Z"
	other.UserID = "someone-else"
	other.UserLogin = "root"

	assert.Equal(t,
		StateContentHashPayload("inst", *base),
		StateContentHashPayload("inst", other),
		"audit-only differences must not change the state hash")
}
