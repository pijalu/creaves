//go:build sqlite
// +build sqlite

package actions

import (
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
	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))

	before := countEvents(t)
	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(1), nil, nil))

	after := countEvents(t)
	assert.Equal(t, before+1, after, "one event row should have been created")

	// The newest event should be undelivered and carry the instance id.
	var ev models.EventStream
	require.NoError(t, pusherTestDB.Order("created_at desc").First(&ev))
	assert.Nil(t, ev.DeliveredAt)
	assert.Equal(t, CurrentConfig.InstanceID, ev.InstanceID)
}

// TestPublishEvent_DisabledByConfig proves no event is written when the event
// stream is disabled.
func TestPublishEvent_DisabledByConfig(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = false
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))

	before := countEvents(t)
	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(2), nil, nil))
	assert.Equal(t, before, countEvents(t), "no event should be written when disabled")
}

// TestPublishEvent_LoadsConfigWhenNil proves PublishEvent auto-loads the
// configuration if CurrentConfig is nil.
func TestPublishEvent_LoadsConfigWhenNil(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

	require.NoError(t, PublishEvent(pusherTestDB, string(models.EventTypeAnimalDiscovered), makeAnimal(3), nil, nil))
	assert.NotNil(t, CurrentConfig, "config should have been loaded by PublishEvent")
	assert.Equal(t, 1, countEvents(t))
}

// TestPublishAnimalHelpers exercises the typed publish helpers end-to-end.
func TestPublishAnimalHelpers(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))

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
	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))

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

	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

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
	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfig.SetSettings(settings))
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

// keep time referenced (helper used by payload parsing elsewhere)
var _ = time.Now
