package actions

import (
	"io"
	"net/http"
	"testing"

	"github.com/gobuffalo/plush/v5"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"creaves/models"
)

// TestSyncTargetAPIKeyVisibilityByRole renders the api-key fragment of the
// sync-target form through plush to prove the webhook API key is visible for
// BOTH admin and maintainer roles (bugs.md #7: a plain admin must be able to
// verify the stored key against the Console-issued one). The Edit handler no
// longer blanks the key: the form is requireAdmin-gated and prefills the
// stored secret for every allowed role. A blank submit still preserves the
// stored key (bindSyncTarget write-only fallback).
func TestSyncTargetAPIKeyVisibilityByRole(t *testing.T) {
	const secret = "creaves_supersecret0123456789"

	// --- form fragment (as in templates/sync_targets/_form.plush*.html) ---
	formTpl := `<input type="<%= if (current_user.Maintainer || current_user.Admin) { %>text<% } else { %>password<% } %>" class="form-control font-monospace" id="webhook_api_key" name="WebhookAPIKey" value="<%= target.WebhookAPIKey %>" placeholder="Enter API key" autocomplete="new-password">`

	// Maintainer: prefilled text input with the stored key.
	maintainerCtx := plush.NewContext()
	maintainerCtx.Set("target", &models.SyncTarget{WebhookAPIKey: secret})
	maintainerCtx.Set("current_user", &models.User{Admin: true, Maintainer: true})
	outMaint, err := plush.Render(formTpl, maintainerCtx)
	require.NoError(t, err)
	assert.Contains(t, outMaint, `type="text"`, "maintainer form input must be type=text")
	assert.Contains(t, outMaint, `value="`+secret+`"`, "maintainer form input must prefill the stored key")

	// Plain admin: also a prefilled text input — the key must be visible
	// (no more write-only password field).
	adminCtx := plush.NewContext()
	adminCtx.Set("target", &models.SyncTarget{WebhookAPIKey: secret})
	adminCtx.Set("current_user", &models.User{Admin: true})
	outAdmin, err := plush.Render(formTpl, adminCtx)
	require.NoError(t, err)
	assert.Contains(t, outAdmin, `type="text"`, "admin form input must be type=text")
	assert.Contains(t, outAdmin, `value="`+secret+`"`, "admin form input must prefill the stored key")

	// MaskedAPIKey reveals only the last 4 characters (used on the list
	// page summary, unchanged).
	assert.Equal(t, "••••6789", (&models.SyncTarget{WebhookAPIKey: secret}).MaskedAPIKey())
	assert.Equal(t, "", (&models.SyncTarget{}).MaskedAPIKey())
}

// TestSyncTargetEditShowsAPIKeyToAdmin drives the real Edit handler as a
// plain admin (non-maintainer): the rendered form must contain the stored
// key in clear text (bugs.md #7).
func TestSyncTargetEditShowsAPIKeyToAdmin(t *testing.T) {
	requireMySQLSuite(t)
	client, baseURL := adminClientWithURL(t) // Admin: true, Maintainer: false
	target := seedHandlerSyncTarget(t, "TS-edit-visibility-"+uuid.Must(uuid.NewV4()).String()[:8], "http://console.example.org/webhook/events", false)

	resp, err := client.Get(baseURL + "/sync_targets/" + target.ID.String() + "/edit")
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, http.StatusOK, resp.StatusCode)

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	html := string(body)
	assert.Contains(t, html, `type="text"`, "api key input must be a visible text field for admins")
	assert.Contains(t, html, "stored-secret-key", "api key value must be rendered for admins")
	assert.NotContains(t, html, `name="WebhookAPIKey" value=""`, "api key field must not be blanked for admins")
}
