//go:build sqlite
// +build sqlite

package actions

import (
	"strings"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedDisabledWebhookConfig installs CurrentConfig with a webhook URL but
// forwarding switched off — the state the resync page shows its
// enable-on-confirm offer for.
func seedDisabledWebhookConfig(t *testing.T, url string) {
	t.Helper()
	resetPusherState()
	settings := models.ConfigSettings{
		EnableEventStream: true,
		WebhookEnabled:    false,
		WebhookURL:        url,
		WebhookAPIKey:     "creaves_testkey",
		WebhookBatchSize:  10,
		WebhookMaxPerMin:  100,
	}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	require.NoError(t, pusherTestDB.Create(cfg))
	CurrentConfig = cfg
}

// TestStartResyncDisabledWebhookReturnsClearError proves a disabled-webhook
// resync fails with the user-facing sentinel message (not a raw buffalo
// trace) and creates no run row.
func TestStartResyncDisabledWebhookReturnsClearError(t *testing.T) {
	seedDisabledWebhookConfig(t, "http://127.0.0.1:1/unreachable")
	saved := CurrentConfig
	t.Cleanup(func() { CurrentConfig = saved })

	_, err := StartResync(pusherTestDB, "rsync-disabled-test", 1, false)
	require.Error(t, err)
	assert.Equal(t, ErrWebhookDisabled, err)
	assert.Contains(t, err.Error(), "webhook forwarding is disabled")
	assert.Contains(t, err.Error(), "enable webhook forwarding")

	count, qErr := pusherTestDB.Where(
		"instance_id = ?", "rsync-disabled-test").Count(&models.ResyncRun{})
	require.NoError(t, qErr)
	assert.Equal(t, 0, count)
}

// TestEnableWebhookForwardingConfirmFlow proves the confirm-to-enable path:
// with a URL configured but forwarding off, enabling flips the persisted
// flag (and the in-memory cache), after which StartResync accepts the run.
func TestEnableWebhookForwardingConfirmFlow(t *testing.T) {
	seedDisabledWebhookConfig(t, "http://127.0.0.1:1/unreachable")
	saved := CurrentConfig
	t.Cleanup(func() {
		CurrentConfig = saved
		pusherTestDB.RawQuery("DELETE FROM resync_runs WHERE instance_id = 'rsync-enable-test'").Exec()
	})
	require.False(t, IsWebhookEnabled())

	require.NoError(t, EnableWebhookForwarding(pusherTestDB))
	assert.True(t, IsWebhookEnabled())

	settings, err := CurrentConfig.GetSettings()
	require.NoError(t, err)
	assert.True(t, settings.WebhookEnabled)

	persisted := &models.Config{}
	require.NoError(t, pusherTestDB.Find(persisted, CurrentConfig.ID))
	persistedSettings, err := persisted.GetSettings()
	require.NoError(t, err)
	assert.True(t, persistedSettings.WebhookEnabled)

	savedCfg := CurrentConfig
	CurrentConfig = &models.Config{InstanceID: "rsync-enable-test"}
	require.NoError(t, CurrentConfig.SetSettings(models.ConfigSettings{
		WebhookEnabled: true, WebhookURL: "http://127.0.0.1:1/unreachable",
	}))
	t.Cleanup(func() { CurrentConfig = savedCfg })
	_ = savedCfg

	run, err := StartResync(pusherTestDB, "rsync-enable-test", 1, false)
	require.NoError(t, err)
	assert.NotNil(t, run)
	require.NoError(t, pusherTestDB.RawQuery(
		"UPDATE resync_runs SET status = 'cancelled' WHERE id = ?", run.ID).Exec())
}

// TestEnableWebhookForwardingRequiresURL proves enabling with no configured
// destination fails with a clear message instead of silently queueing.
func TestEnableWebhookForwardingRequiresURL(t *testing.T) {
	seedDisabledWebhookConfig(t, "")
	saved := CurrentConfig
	t.Cleanup(func() { CurrentConfig = saved })

	err := EnableWebhookForwarding(pusherTestDB)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no webhook URL"))
	assert.False(t, IsWebhookEnabled())
}
