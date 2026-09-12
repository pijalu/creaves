//go:build sqlite
// +build sqlite

package actions

import (
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLoadConfig_FindsExistingActive proves LoadConfig reads the active config
// row from the DB into the package global CurrentConfig.
func TestLoadConfig_FindsExistingActive(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://example.com/webhook")

	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

	cfg, err := LoadConfig(pusherTestDB)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Active)
	assert.Equal(t, CurrentConfig.ID, cfg.ID)

	// Webhook fields round-tripped from settings JSON.
	settings, err := cfg.GetSettings()
	require.NoError(t, err)
	assert.True(t, settings.WebhookEnabled)
	assert.Equal(t, "http://example.com/webhook", settings.WebhookURL)
}

// TestLoadConfig_CreatesDefaultWhenMissing proves LoadConfig bootstraps a new
// default config when none exists.
func TestLoadConfig_CreatesDefaultWhenMissing(t *testing.T) {
	resetPusherState()

	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

	cfg, err := LoadConfig(pusherTestDB)
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Active)

	// Default settings applied.
	settings, err := cfg.GetSettings()
	require.NoError(t, err)
	assert.True(t, settings.EnableEventStream)
	assert.False(t, settings.WebhookEnabled)
	assert.Equal(t, 1, settings.WebhookBatchSize)
	assert.Equal(t, 60, settings.WebhookMaxPerMin)
}

// TestIsEventStreamEnabled proves the flag reflects CurrentConfig.
func TestIsEventStreamEnabled(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")

	// Disabled by default in seeded config? Enable it explicitly.
	settings, _ := CurrentConfig.GetSettings()
	settings.EnableEventStream = true
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))
	assert.True(t, IsEventStreamEnabled())

	// Now disable.
	settings.EnableEventStream = false
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))
	assert.False(t, IsEventStreamEnabled())

	// Nil config → false.
	CurrentConfig = nil
	assert.False(t, IsEventStreamEnabled())
}

// TestIsWebhookEnabled proves the flag requires both WebhookEnabled and URL.
func TestIsWebhookEnabled(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	assert.True(t, IsWebhookEnabled())

	// URL empty → disabled.
	settings, _ := CurrentConfig.GetSettings()
	settings.WebhookURL = ""
	require.NoError(t, CurrentConfig.SetSettings(settings))
	require.NoError(t, pusherTestDB.Update(CurrentConfig))
	assert.False(t, IsWebhookEnabled())

	// Nil config → false.
	CurrentConfig = nil
	assert.False(t, IsWebhookEnabled())
}

// TestGetInstanceID_FromConfig proves GetInstanceID reads from CurrentConfig.
func TestGetInstanceID_FromConfig(t *testing.T) {
	resetPusherState()
	seedPusherConfig(t, "http://unused.example")
	assert.Equal(t, CurrentConfig.InstanceID, GetInstanceID())

	CurrentConfig = nil
	assert.Equal(t, "", GetInstanceID())
}

// TestInitWebhookAtBoot proves the boot helper loads config and starts the worker.
func TestInitWebhookAtBoot(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	// Seed an enabled config into the DB (CurrentConfig is nil so it must be
	// loaded by InitWebhookAtBoot).
	seedPusherConfig(t, "http://127.0.0.1:1/webhook")
	CurrentConfig = nil

	InitWebhookAtBoot()
	assert.NotNil(t, CurrentConfig, "config should be loaded at boot")
	assert.True(t, IsWebhookWorkerRunning(), "worker should start at boot when enabled")

	StopWebhookWorker()
}

// TestInitWebhookAtBoot_StartsWhenDisabled proves boot starts the worker even
// when webhook forwarding is off: the worker also runs the hourly event
// purge, so it must run regardless; delivery stays gated per-tick by
// IsWebhookEnabled().
func TestInitWebhookAtBoot_StartsWhenDisabled(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()

	// Seed a config with webhook disabled.
	settings := models.ConfigSettings{
		EnableEventStream: true,
		WebhookEnabled:    false,
	}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	require.NoError(t, pusherTestDB.Create(cfg))
	CurrentConfig = nil

	InitWebhookAtBoot()
	assert.True(t, IsWebhookWorkerRunning(), "worker should start at boot even when disabled (purge duty)")

	StopWebhookWorker()
}


// TestLoadConfig_QueryError verifies the error path when the database query
// itself fails (e.g. the config table does not exist). LoadConfig must return
// the error rather than panicking or creating a default.
func TestLoadConfig_QueryError(t *testing.T) {
	bad := newEmptyDB(t)
	defer bad.Close()

	saved := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = saved }()

	cfg, err := LoadConfig(bad)
	require.Error(t, err)
	assert.Nil(t, cfg)
}

// TestInitWebhookAtBoot_ConfigLoadFailure verifies the boot failure path: when
// LoadConfig fails (here because models.DB points at a table-less DB),
// InitWebhookAtBoot logs the error and returns early without starting the
// worker.
func TestInitWebhookAtBoot_ConfigLoadFailure(t *testing.T) {
	resetPusherState()
	StopWebhookWorker()
	require.False(t, IsWebhookWorkerRunning())

	bad := newEmptyDB(t)
	defer bad.Close()

	savedDB := models.DB
	models.DB = bad
	defer func() { models.DB = savedDB }()

	savedCfg := CurrentConfig
	CurrentConfig = nil
	defer func() { CurrentConfig = savedCfg }()

	InitWebhookAtBoot()
	// Config load failed, so the worker must NOT start.
	assert.False(t, IsWebhookWorkerRunning())
	assert.Nil(t, CurrentConfig)
}
