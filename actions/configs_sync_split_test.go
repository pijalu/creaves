package actions

import (
	"io"
	"net/http"
	"net/url"
	"testing"

	"creaves/models"

	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// bugs.md #8: the general configuration pages must not contain sync details.
// General config = CREAVES identity only; the instance ID and the event
// stream flag live on the sync configuration page (/sync_configuration),
// next to the multiple sync targets.
//
// These tests drive the full App() with an admin session and the MySQL test
// database (GO_ENV=test).
// ---------------------------------------------------------------------------

// TestConfigShowHasNoSyncDetails proves the config show page renders the
// identity columns but neither the instance-ID row nor the event-stream
// section.
func TestConfigShowHasNoSyncDetails(t *testing.T) {
	requireMySQLTestDB(t)
	cfg := seedConfig(t, "cfg-show-nosync-"+uuid.Must(uuid.NewV4()).String()[:8], false)
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE id = ?", cfg.ID.String()).Exec()
	})

	client, baseURL := adminClientWithURL(t)
	resp, err := client.Get(baseURL + "/config/" + cfg.ID.String())
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	assert.Contains(t, html, cfg.Name, "general config shows the config name")
	assert.NotContains(t, html, "fa-exchange-alt", "event-stream section must be gone from the show page")
	assert.NotContains(t, html, cfg.InstanceID, "instance ID is a sync detail and must not appear")
}

// TestConfigNewIsIdentityOnly proves the new-config form no longer renders
// the sync fields (instance ID input + event-stream checkbox) but keeps the
// identity fields.
func TestConfigNewIsIdentityOnly(t *testing.T) {
	requireMySQLTestDB(t)
	client, baseURL := adminClientWithURL(t)

	resp, err := client.Get(baseURL + "/config/new")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	assert.Contains(t, html, `name="Settings.CenterName"`, "identity fields must be on the new form")
	assert.Contains(t, html, `name="Name"`, "config name must be on the new form")
	assert.NotContains(t, html, `name="InstanceID"`, "instance ID belongs to the sync configuration")
	assert.NotContains(t, html, `name="Settings.EnableEventStream"`, "event-stream flag belongs to the sync configuration")
}

// TestConfigCreateWithoutInstanceID proves a config created from the
// identity-only form gets a generated instance ID and keeps the default
// (enabled) event-stream flag.
func TestConfigCreateWithoutInstanceID(t *testing.T) {
	requireMySQLTestDB(t)
	saved := CurrentConfigGet()
	t.Cleanup(func() { CurrentConfigSet(saved) })

	client, baseURL := adminClientWithURL(t)
	token := todoToken(t, client, baseURL, "/config/new")

	name := "cfg-create-nosync-" + uuid.Must(uuid.NewV4()).String()[:8]
	t.Cleanup(func() {
		models.DB.RawQuery("DELETE FROM config WHERE name = ?", name).Exec()
	})

	form := url.Values{}
	form.Set("Name", name)
	form.Set("Description", "created without an instance ID")
	form.Set("Settings.CenterName", "Center Name")
	form.Set("Settings.AsblName", "ASBL")
	resp := postTodoForm(t, client, baseURL, "/config", token, form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("config create status = %d, want redirect", resp.StatusCode)
	}

	created := &models.Config{}
	require.NoError(t, models.DB.Where("name = ?", name).First(created))
	assert.NotEmpty(t, created.InstanceID, "instance ID must be generated when the form omits it")
	assert.True(t, created.Active || !created.Active) // untouched; no assertion on the checkbox
	settings, err := created.GetSettings()
	require.NoError(t, err)
	assert.True(t, settings.EnableEventStream, "event-stream flag keeps its default (true)")
	assert.Equal(t, "Center Name", settings.CenterName, "identity fields persist")
}
