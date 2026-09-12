//go:build sqlite
// +build sqlite

package actions

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"creaves/models"

	"github.com/gobuffalo/nulls"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeAnimal builds a minimal, valid Animal for event-publishing tests.
func makeAnimal(id int) *models.Animal {
	return &models.Animal{
		ID:         id,
		Year:       2024,
		YearNumber: id,
		Species:    "Fox",
		Gender:     nulls.NewString("Male"),
	}
}

// TestPublishEvent_CreatesEventStreamRow proves PublishEvent writes an
// undelivered event row when the event stream is enabled.
func TestPublishEvent_CreatesEventStreamRow(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	// Enable event stream in the current config.
	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))

	before := countEvents(t)
	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(1), nil, nil))

	after := countEvents(t)
	assert.Equal(t, before+1, after, "one event row should have been created")

	// The newest event should be undelivered and carry the instance id.
	var ev models.EventStream
	require.NoError(t, pusherTestDB.Order("created_at desc").First(&ev))
	assert.Nil(t, ev.DeliveredAt)
	assert.Equal(t, CurrentConfigGet().InstanceID, ev.InstanceID)
}

// TestPublishEvent_DisabledByConfig proves no event is written when the event
// stream is disabled.
func TestPublishEvent_DisabledByConfig(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = false
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))

	before := countEvents(t)
	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(2), nil, nil))
	assert.Equal(t, before, countEvents(t), "no event should be written when disabled")
}

// TestPublishEvent_LoadsConfigWhenNil proves PublishEvent auto-loads the
// configuration if CurrentConfig is nil.
func TestPublishEvent_LoadsConfigWhenNil(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	saved := CurrentConfigGet()
	CurrentConfigSet(nil)
	defer func() { CurrentConfigSet(saved) }()

	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(3), nil, nil))
	assert.NotNil(t, CurrentConfigGet(), "config should have been loaded by PublishEvent")
	assert.Equal(t, 1, countEvents(t))
}

// seedPublishAnimals inserts persisted animal chains so the publish helpers
// can reload animals (reloadAnimalForEvent) with all associations.
func seedPublishAnimals(t *testing.T, ids ...int) {
	t.Helper()
	ageID := "aaaaaaaa-1111-1111-1111-1111111111f3"
	typeID := "bbbbbbbb-2222-2222-2222-2222222222f3"
	discID := "dddddddd-4444-4444-4444-4444444444f3"
	ecID := "SYNCST_EC_PUB"
	exec := func(q string, args ...interface{}) {
		t.Helper()
		if err := pusherTestDB.RawQuery(q, args...).Exec(); err != nil {
			t.Fatalf("seedPublishAnimals: %v\n%s", err, q)
		}
	}
	exec("DELETE FROM animals WHERE id IN ("+placeholderList(len(ids))+")", toIfaceSlice(ids)...)
	exec("DELETE FROM intakes WHERE id LIKE '55555557-0000-0000-0000-0000000000%'")
	exec("DELETE FROM discoveries WHERE id LIKE '66666668-0000-0000-0000-0000000000%'")
	exec("DELETE FROM discoverers WHERE id = ?", discID)
	exec("DELETE FROM entry_causes WHERE id = ?", ecID)
	exec("DELETE FROM animaltypes WHERE id = ?", typeID)
	exec("DELETE FROM animalages WHERE id = ?", ageID)
	exec("INSERT INTO animalages (id, name, `def`, created_at, updated_at) VALUES (?, 'PUB Age', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", ageID)
	exec("INSERT INTO animaltypes (id, name, `def`, created_at, updated_at) VALUES (?, 'PUB Type', 0, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", typeID)
	exec("INSERT INTO entry_causes (id, cause, detail, nature, indication, created_at, updated_at, sort_order) VALUES (?, 'PUB_C', 'PUB_D', 'PUB_N', 'x', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 1)", ecID)
	exec("INSERT INTO discoverers (id, created_at, updated_at) VALUES (?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", discID)
	for i, id := range ids {
		intakeID := fmt.Sprintf("55555557-0000-0000-0000-0000000000%02d", i)
		dscrID := fmt.Sprintf("66666668-0000-0000-0000-0000000000%02d", i)
		exec("INSERT INTO intakes (id, date, created_at, updated_at) VALUES (?, '2025-06-01 10:00:00', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", intakeID)
		exec("INSERT INTO discoveries (id, date, discoverer_id, entry_cause_id, created_at, updated_at) VALUES (?, '2025-06-01 10:00:00', ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)", dscrID, discID, ecID)
		exec("INSERT INTO animals (id, species, animalage_id, animaltype_id, discovery_id, intake_id, created_at, updated_at, year, yearNumber, IntakeDate) VALUES (?, 'PUB_Fox', ?, ?, ?, ?, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP, 2025, ?, '2025-06-01 10:00:00')", id, ageID, typeID, dscrID, intakeID, id)
	}
	t.Cleanup(func() {
		pusherTestDB.RawQuery("DELETE FROM animals WHERE id IN ("+placeholderList(len(ids))+")", toIfaceSlice(ids)...).Exec()
	})
}

func toIfaceSlice(ids []int) []interface{} {
	out := make([]interface{}, len(ids))
	for i, id := range ids {
		out[i] = id
	}
	return out
}

func placeholderList(n int) string {
	parts := make([]string, n)
	for i := range parts {
		parts[i] = "?"
	}
	return strings.Join(parts, ",")
}

// TestPublishAnimalHelpers exercises the typed publish helpers end-to-end.
func TestPublishAnimalHelpers(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))

	seedPublishAnimals(t, 10, 11)

	require.NoError(t, PublishAnimalDiscoveredEvent(pusherTestDB, makeAnimal(10), nil))
	require.NoError(t, PublishAnimalStatusChangedEvent(pusherTestDB, makeAnimal(10), "in_care", "under_treatment", nil))
	require.NoError(t, PublishAnimalReleasedEvent(pusherTestDB, makeAnimal(10), nil))
	require.NoError(t, PublishAnimalDiedEvent(pusherTestDB, makeAnimal(11), nil))

	assert.Equal(t, 4, countEvents(t))

	events := &models.EventStreams{}
	require.NoError(t, pusherTestDB.Order("created_at asc").All(events))
	assert.Equal(t, string(models.EventTypeAnimalDiscovered), (*events)[0].EventType)
	assert.Equal(t, string(models.EventTypeAnimalStatusChanged), (*events)[1].EventType)
	assert.Equal(t, string(models.EventTypeAnimalReleased), (*events)[2].EventType)
	assert.Equal(t, string(models.EventTypeAnimalDied), (*events)[3].EventType)
}

// TestPublishEvent_SetsUserAuditTrail proves the user id/login are embedded in
// the payload when a user is supplied.
func TestPublishEvent_SetsUserAuditTrail(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))

	uid := uuid.Must(uuid.NewV4())
	user := &models.User{ID: uid, Login: "alice"}

	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(5), nil, user))

	var ev models.EventStream
	require.NoError(t, pusherTestDB.Order("created_at desc").First(&ev))
	p, err := ev.GetPayload()
	require.NoError(t, err)
	assert.Equal(t, uid.String(), p.UserID)
	assert.Equal(t, "alice", p.UserLogin)
}

// TestPublishEvent_LoadConfigFails verifies the error path when PublishEvent
// cannot load the config (CurrentConfig is nil and the provided connection has
// no config table). PublishEvent must propagate the error.
func TestPublishEvent_LoadConfigFails(t *testing.T) {
	resetPusherState()

	bad := newEmptyDB(t)
	defer bad.Close()

	saved := CurrentConfigGet()
	CurrentConfigSet(nil)
	defer func() { CurrentConfigSet(saved) }()

	err := PublishEvent(bad, string(models.EventTypeAnimalDiscovered), makeAnimal(1), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load config")
}

// TestPublishEvent_CreateError verifies the error path when the event row
// cannot be persisted (CurrentConfig is set and event stream is enabled, but
// the provided connection has no event_streams table). PublishEvent must
// propagate the "failed to create event" error.
func TestPublishEvent_CreateError(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	// No need to persist the config update to pusherTestDB — we only need
	// CurrentConfig in memory for IsEventStreamEnabled() to return true.

	bad := newEmptyDB(t)
	defer bad.Close()

	err := PublishEvent(bad, string(models.EventTypeAnimalDiscovered), makeAnimal(1), nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to create event")
}

// countEvents returns the number of rows in event_streams.
func countEvents(t *testing.T) int {
	t.Helper()
	events := &models.EventStreams{}
	require.NoError(t, pusherTestDB.All(events))
	return len(*events)
}

// TestPublishAnimalDeletedEvent_TypeAndStatus proves the destroy flow's event
// is an animal_deleted event carrying current_status "deleted" — the console
// removes the animal instead of listing it as deceased (bugs.md "Delete show
// as deceased in console").
func TestPublishAnimalDeletedEvent_TypeAndStatus(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	settings, _ := CurrentConfigGet().GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfigGet().SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfigGet()))
	seedPublishAnimals(t, 12)

	require.NoError(t, PublishAnimalDeletedEvent(pusherTestDB, makeAnimal(12), nil))

	events := &models.EventStreams{}
	require.NoError(t, pusherTestDB.Where("animal_id = ?", 12).All(events))
	require.Len(t, *events, 1, "exactly one event must be created")
	ev := (*events)[0]
	assert.Equal(t, models.EventTypeAnimalDeleted, models.EventType(ev.EventType),
		"destroy must publish animal_deleted, not animal_died")

	payload, err := ev.GetPayload()
	require.NoError(t, err)
	assert.Equal(t, "deleted", payload.CurrentStatus)
}

// keep time referenced (helper used by payload parsing elsewhere)
var _ = time.Now
