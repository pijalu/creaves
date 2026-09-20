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

// seedDisabledWebhookConfig installs CurrentConfig plus one DISABLED sync
// target pointing at url — the state the resync page shows its
// enable-on-confirm offer for. Pass url="" to simulate "no destination
// configured at all".
func seedDisabledWebhookConfig(t *testing.T, url string) {
	t.Helper()
	resetPusherState()
	settings := models.ConfigSettings{EnableEventStream: true}
	cfg := &models.Config{
		ID:         uuid.Must(uuid.NewV4()),
		InstanceID: "test-instance",
		Name:       "Test",
		Active:     true,
	}
	require.NoError(t, cfg.SetSettings(settings))
	require.NoError(t, pusherTestDB.Create(cfg))
	CurrentConfigSet(cfg)
	if url == "" {
		return
	}
	target := &models.SyncTarget{
		ID:               uuid.Must(uuid.NewV4()),
		Name:             "disabled-target",
		Enabled:          false,
		WebhookURL:       url,
		WebhookAPIKey:    "creaves_testkey",
		WebhookBatchSize: 10,
		WebhookMaxPerMin: 100,
	}
	require.NoError(t, pusherTestDB.Create(target))
	SetSyncTargetsKnown(false)
	t.Cleanup(func() { SetSyncTargetsKnown(false) })
}

// TestStartResyncDisabledWebhookReturnsClearError proves a disabled-target
// resync fails with the user-facing sentinel message (not a raw buffalo
// trace) and creates no run row.
func TestStartResyncDisabledWebhookReturnsClearError(t *testing.T) {
	seedDisabledWebhookConfig(t, "http://127.0.0.1:1/unreachable")
	saved := CurrentConfigGet()
	t.Cleanup(func() { CurrentConfigSet(saved) })

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
// with a disabled target configured, enabling flips the persisted flag,
// after which StartResync accepts the run.
func TestEnableWebhookForwardingConfirmFlow(t *testing.T) {
	seedDisabledWebhookConfig(t, "http://127.0.0.1:1/unreachable")
	saved := CurrentConfigGet()
	t.Cleanup(func() {
		CurrentConfigSet(saved)
		pusherTestDB.RawQuery("DELETE FROM resync_runs WHERE instance_id = 'rsync-enable-test'").Exec()
	})
	require.False(t, IsWebhookEnabled())

	require.NoError(t, EnableWebhookForwarding(pusherTestDB))
	assert.True(t, IsWebhookEnabled())

	// The flag is persisted on the target row.
	persisted := models.SyncTargets{}
	require.NoError(t, pusherTestDB.All(&persisted))
	require.Len(t, persisted, 1)
	assert.True(t, persisted[0].Enabled)

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
	saved := CurrentConfigGet()
	t.Cleanup(func() { CurrentConfigSet(saved) })

	err := EnableWebhookForwarding(pusherTestDB)
	require.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "no sync target"))
	assert.False(t, IsWebhookEnabled())
}
